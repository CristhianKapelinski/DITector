package myutils

import (
	"context"
	"fmt"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
	"strconv"
)

type MyNeo4j struct {
	Driver neo4j.DriverWithContext
}

// ConfigNewNeo4jDriverWithContext 返回一个配置完全的neo4j driver
func ConfigNewNeo4jDriverWithContext(target, neo4jUsername, neo4jPassword string) (*MyNeo4j, error) {
	var ret = new(MyNeo4j)
	var err error

	ret.Driver, err = neo4j.NewDriverWithContext(
		target,
		neo4j.BasicAuth(neo4jUsername, neo4jPassword, ""),
	)

	// 创建索引，neo4j没有提供判断重复创建索引导致报错的函数，所以不处理err
	session := ret.Driver.NewSession(context.TODO(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.TODO())
	session.ExecuteWrite(context.TODO(), func(tx neo4j.ManagedTransaction) (any, error) {
		// 创建索引：基于节点id
		tx.Run(context.TODO(),
			"CREATE INDEX layer_id_index IF NOT EXISTS FOR (l:Layer) ON (l.id)",
			map[string]any{},
		)

		// 创建索引：基于节点layer-id
		tx.Run(context.TODO(),
			"CREATE INDEX layer_digest_index IF NOT EXISTS FOR (l:Layer) ON (l.digest)",
			map[string]any{},
		)

		// 创建索引：基于节点layer-id
		tx.Run(context.TODO(),
			"CREATE INDEX rawlayer_digest_index IF NOT EXISTS FOR (l:RawLayer) ON (l.digest)",
			map[string]any{},
		)

		return nil, nil
	})

	return ret, err
}

// InsertImageToNeo4j 将
func (neo4jDriver *MyNeo4j) InsertImageToNeo4j(image *ImageSource) {
	// 创建一个neo4j session
	ctx := context.Background()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	previousHash := ""      // 用于存上一个hash(1-2)
	accumulateLayerID := "" // 用于堆1、1-2、1-2-5，方便直接计算hash
	accumulateHash := ""    // =hash(accumulateLayerID)，用于存当前hash(1-2-5)

	// 一些基本赋值
	lastLayerIndex := 0 // 仍有文件内容的最顶层在Image.Layers中的index
	imageName := image.Namespace + "/" + image.RepositoryName + ":" + image.TagName

	for i, _ := range image.Image.Layers {
		// 跳过没有文件内容的层
		if image.Image.Layers[i].Size == 0 {
			continue
		}

		// 计算hash(1-2-5)，转成string类型
		curLayer := image.Image.Layers[i]
		layerID := curLayer.Digest[7:]
		accumulateLayerID += layerID
		accumulateHash = CalSha256(accumulateLayerID)

		// 插入层及层间的边
		_, err := session.ExecuteWrite(ctx, addNewLayerFunc(ctx, previousHash, accumulateHash, curLayer))
		if err != nil {
			LogDockerCrawlerString(fmt.Sprintf("[ERROR] Insert "+imageName+" layer "+layerID+" to neo4j failed with: %s", err))
			fmt.Printf("[ERROR] Insert "+imageName+" layer "+layerID+" to neo4j failed with: %s\n", err)
			break
		}

		// 更新previousHash，下一轮插入节点的父节点ID应为previousHash
		previousHash = accumulateHash
		// 记录最后一层的index，
		lastLayerIndex = i
	}

	// 需要将image信息加入到节点属性中
	_, err := session.ExecuteWrite(ctx, addImageToLayerFunc(ctx, imageName, accumulateHash))
	if err != nil {
		LogDockerCrawlerString(fmt.Sprintf("[ERROR] Insert image "+image.Namespace+"/"+image.RepositoryName+":"+image.TagName+" of layer "+strconv.Itoa(lastLayerIndex)+" to neo4j failed with: %s", err))
		fmt.Printf("[ERROR] Insert image "+image.Namespace+"/"+image.RepositoryName+":"+image.TagName+" of layer "+strconv.Itoa(lastLayerIndex)+" to neo4j failed with: %s\n", err)
	}
}

// addNewLayerFunc 返回可用于session.ExecuteWrite的func，将Layer节点及节点间的边插入neo4j
func addNewLayerFunc(ctx context.Context, previousHash, idHash string, layer Layer) neo4j.ManagedTransactionWork {
	// 节点的两种label
	// Layer:
	// 		id: hash(1-2-5)
	// 		digest: layer-ID
	// 		images: [namespace1/repository1:tag1, ...]
	// RawLayer:
	// 		digest: layer-ID
	// 		size: size
	//		instruction: instruction
	//		scanned: true/false
	// 		file_added: []
	//		file_deleted: []
	//		vul: [[]]

	// 当前层为镜像的第一层，只需要插入层信息即可
	if previousHash == "" {
		return func(tx neo4j.ManagedTransaction) (any, error) {
			var result, err = tx.Run(ctx,
				"MERGE (l:Layer {id: $idHash}) "+
					"ON CREATE SET l.digest=$digest, l.images=$images "+
					"WITH l "+
					"MERGE (rl:RawLayer {digest: $digest}) "+
					"ON CREATE SET rl.size=$size, rl.instruction=$instruction, rl.scanned=$scanned, rl.file_added=$file_added, rl.file_deleted=$file_deleted, rl.vul=$vul "+
					"WITH l,rl "+
					"MERGE (l)-[:SAME]-(rl)",
				map[string]any{"idHash": idHash, "digest": layer.Digest, "images": []string{},
					"size": layer.Size, "instruction": layer.Instruction, "scanned": false, "file_added": []string{}, "file_deleted": []string{}, "vul": [][]string{}},
			)

			if err != nil {
				return nil, err
			}

			return result.Consume(ctx)
		}
	} else {
		// 当前层非镜像第一层，需要插入层节点、边previous-->current
		return func(tx neo4j.ManagedTransaction) (any, error) {
			var result, err = tx.Run(ctx,
				"MERGE (l:Layer {id: $idHash}) "+
					"ON CREATE SET l.digest=$digest, l.images=$images "+
					"WITH l "+
					"MERGE (rl:RawLayer {digest: $digest}) "+
					"ON CREATE SET rl.size=$size, rl.instruction=$instruction, rl.scanned=$scanned, rl.file_added=$file_added, rl.file_deleted=$file_deleted, rl.vul=$vul "+
					"WITH l,rl "+
					"MERGE (l)-[:SAME]-(rl) "+
					"WITH l "+
					"MATCH (previous:Layer {id: $previousHash}) "+
					"MERGE (previous)-[:IS_BASE_OF]->(l)",
				map[string]any{"previousHash": previousHash, "idHash": idHash, "digest": layer.Digest, "images": []string{},
					"size": layer.Size, "instruction": layer.Instruction, "scanned": false, "file_added": []string{}, "file_deleted": []string{}, "vul": [][]string{}},
			)

			if err != nil {
				return nil, err
			}

			return result.Consume(ctx)
		}
	}
}

// addImageToLayerFunc 返回可用于session.ExecuteWrite的func，将image添加到最顶层
func addImageToLayerFunc(ctx context.Context, imageName, idHash string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:Layer {id: $idHash}) "+
				"WHERE NOT $imageInfo IN l.images "+
				"SET l.images=l.images+$imageInfo",
			map[string]any{"idHash": idHash, "imageInfo": imageName},
		)

		if err != nil {
			return nil, err
		}

		return result.Consume(ctx)
	}
}

// GetNodeProps 解析neo4j driver ExecuteRead返回*neo4j.Record节点属性
func GetNodeProps(n *neo4j.Record) map[string]any {
	keys := n.Keys
	if len(keys) == 1 {
		prop, _ := n.Get(keys[0])
		return prop.(dbtype.Node).Props
	}

	return nil
}

// FindUpstreamLayerNodesByNodeId 根据hash(1-2-5)发现所有上游Layer节点
func (neo4jDriver *MyNeo4j) FindUpstreamLayerNodesByNodeId(nodeId string) (any, error) {

	ctx := context.TODO()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	upNodes, err := session.ExecuteRead(ctx, findUpstreamNodesByNodeIdFunc(ctx, nodeId))
	if err != nil {
		LogDockerCrawlerString("[ERROR] Neo4j find upstream Layer nodes by node id", nodeId, "failed with:", err.Error())
		return nil, err
	}

	return upNodes, nil
}

// findUpstreamNodesByNodeIdFunc 返回可用于session.ExecuteRead的func，find upstream Layer Nodes according to hash(1-2-5)
func findUpstreamNodesByNodeIdFunc(ctx context.Context, nodeId string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:Layer {id: $idHash}) "+
				"WITH l "+
				"MATCH (up:Layer)-[:IS_BASE_OF*]->(l) "+
				"RETURN up",
			map[string]any{"idHash": nodeId},
		)

		if err != nil {
			return nil, err
		}

		records, err := result.Collect(ctx)

		return records, err
	}
}

// FindDownstreamLayerNodesByNodeId 根据hash(1-2-5)发现所有下游Layer节点
func (neo4jDriver *MyNeo4j) FindDownstreamLayerNodesByNodeId(nodeId string) (any, error) {

	ctx := context.TODO()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	upNodes, err := session.ExecuteRead(ctx, findDownstreamNodesByNodeIdFunc(ctx, nodeId))
	if err != nil {
		LogDockerCrawlerString("[ERROR] Neo4j find downstream Layer nodes by node id", nodeId, "failed with:", err.Error())
		return nil, err
	}

	return upNodes, nil
}

// findDownstreamNodesByNodeIdFunc 返回可用于session.ExecuteRead的func，find downstream Layer Nodes according to hash(1-2-5)
func findDownstreamNodesByNodeIdFunc(ctx context.Context, nodeId string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:Layer {id: $idHash}) "+
				"WITH l "+
				"MATCH (l)-[:IS_BASE_OF*]->(down:Layer) "+
				"RETURN down",
			map[string]any{"idHash": nodeId},
		)

		if err != nil {
			return nil, err
		}

		records, err := result.Collect(ctx)

		return records, err
	}
}

// DropNodesAndRelationshipsFromNeo4j 将neo4j数据库清空
func (neo4jDriver *MyNeo4j) DropNodesAndRelationshipsFromNeo4j() {
	ctx := context.TODO()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	session.ExecuteWrite(ctx, func(transaction neo4j.ManagedTransaction) (any, error) {
		transaction.Run(ctx,
			"MATCH (i)-[j]->(k) DELETE i,j,k",
			map[string]any{})
		transaction.Run(ctx,
			"MATCH (n) DELETE n",
			map[string]any{})

		return nil, nil
	})
}
