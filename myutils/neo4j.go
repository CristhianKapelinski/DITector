package myutils

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

type MyNeo4j struct {
	Driver neo4j.DriverWithContext
}

type LayerNotScannedError struct {
	Msg string
}

func (e *LayerNotScannedError) Error() string {
	return fmt.Sprintf("LayerNotScannedError: %s", e.Msg)
}

type LayerNotExistsError struct {
	Msg string
}

func (e *LayerNotExistsError) Error() string {
	return fmt.Sprintf("LayerNotExistsError: %s", e.Msg)
}

func NewNeo4jDriverGlobalConfig() (*MyNeo4j, error) {
	return NewNeo4jDriver(GlobalConfig.Neo4jConfig.Neo4jURI, GlobalConfig.Neo4jConfig.Neo4jUsername,
		GlobalConfig.Neo4jConfig.Neo4jPassword, false)
}

// NewNeo4jDriver 返回一个配置完全的neo4j driver
func NewNeo4jDriver(target, username, password string, initFlag bool) (*MyNeo4j, error) {
	var ret = new(MyNeo4j)
	var err error

	ret.Driver, err = neo4j.NewDriverWithContext(
		target,
		neo4j.BasicAuth(username, password, ""),
	)
	if err != nil {
		return nil, err
	}

	// 验证连接
	err = ret.Driver.VerifyConnectivity(context.TODO())
	if err != nil {
		return nil, err
	}

	// 创建索引，neo4j没有提供判断重复创建索引导致报错的函数，所以不处理err
	if initFlag {
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
	}

	return ret, err
}

// InsertImageToNeo4j 将镜像插入到neo4j数据库中，imgName要求为registry/namespace/repository:tag@digest的格式
func (neo4jDriver *MyNeo4j) InsertImageToNeo4j(imgName string, image *Image) {
	// 创建一个neo4j session
	ctx := context.Background()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// 初始化变量
	preID := "" // 用于存储上一个镜像层对应Layer节点的ID

	// 遍历镜像层
	for i, layer := range image.Layers {
		// 有文件内容的层基于digest计算，没有文件内容的层基于命令计算
		dig := ""
		if layer.Digest != "" {
			dig = Sha256Str(layer.Digest)
		} else {
			dig = Sha256Str(layer.Instruction)
		}
		if dig == "" {
			Logger.Error(fmt.Sprintf("digest of layer %d of image %s still none after calculating SHA256", i, imgName))
			return
		}

		// 计算当前层的Layer节点ID
		currID := Sha256Str(preID + dig)

		// 将当前Layer节点存储到数据库
		if _, err := session.ExecuteWrite(ctx, addNewLayerFunc(ctx, preID, currID, layer)); err != nil {
			Logger.Error(fmt.Sprintf("insert layer %d of image %s failed with: %s", i, imgName, err))
			return
		}

		preID = currID
	}

	// 将image name放到最后一层的Layer节点上
	if _, err := session.ExecuteWrite(ctx, addImageToLayerFunc(ctx, imgName, preID)); err != nil {
		Logger.Error(fmt.Sprintf("insert name of image %s failed with: %s", imgName, err))
		return
	}
}

//// InsertImageToNeo4jOld Deprecated: 根据爬虫json格式结果将镜像元数据存储到neo4j数据库中
//// 没有考虑未产生文件修改的镜像的层（配置命令）
//func (neo4jDriver *MyNeo4j) InsertImageToNeo4jOld(image *ImageSource) {
//	// 创建一个neo4j session
//	ctx := context.Background()
//	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
//	defer session.Close(ctx)
//
//	previousHash := ""      // 用于存上一个hash(1-2)
//	accumulateLayerID := "" // 用于堆1、1-2、1-2-5，方便直接计算hash
//	accumulateHash := ""    // =hash(accumulateLayerID)，用于存当前hash(1-2-5)
//
//	lastLayerIndex := 0 // 仍有文件内容的最顶层在Image.Layers中的index
//	imageName := image.Namespace + "/" + image.RepositoryName + ":" + image.TagName
//
//	for i, _ := range image.Image.Layers {
//		// 跳过没有文件内容的层
//		if image.Image.Layers[i].Size == 0 {
//			continue
//		}
//
//		// 计算hash(1-2-5)，转成string类型
//		curLayer := image.Image.Layers[i]
//		layerID := curLayer.Digest
//		accumulateLayerID += layerID
//		accumulateHash = Sha256Str(accumulateLayerID)
//
//		// 插入层及层间的边
//		_, err := session.ExecuteWrite(ctx, addNewLayerFunc(ctx, previousHash, accumulateHash, curLayer))
//		if err != nil {
//			Logger.Error("Insert", imageName, "layer", layerID, "to neo4j failed with:", err.Error())
//			fmt.Printf("[ERROR] Insert "+imageName+" layer "+layerID+" to neo4j failed with: %s\n", err)
//			break
//		}
//
//		// 更新previousHash，下一轮插入节点的父节点ID应为previousHash
//		previousHash = accumulateHash
//		// 记录最后一层的index，
//		lastLayerIndex = i
//	}
//
//	// 需要将image信息加入到节点属性中
//	_, err := session.ExecuteWrite(ctx, addImageToLayerFunc(ctx, imageName, accumulateHash))
//	if err != nil {
//		Logger.Error(fmt.Sprintf("Insert image "+image.Namespace+"/"+image.RepositoryName+":"+image.TagName+" of layer "+strconv.Itoa(lastLayerIndex)+" to neo4j failed with: %s", err))
//		fmt.Printf("[ERROR] Insert image "+image.Namespace+"/"+image.RepositoryName+":"+image.TagName+" of layer "+strconv.Itoa(lastLayerIndex)+" to neo4j failed with: %s\n", err)
//	}
//}

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

	// 当前层为镜像的第一层，只需要插入层信息即可
	if previousHash == "" {
		// 配置命令对应的层不创建RawLayer
		if layer.Digest == "" {
			return func(tx neo4j.ManagedTransaction) (any, error) {
				var result, err = tx.Run(ctx,
					"MERGE (l:Layer {id: $idHash}) "+
						"ON CREATE SET l.digest=$digest, l.images=$images, l.size=$size, l.instruction=$instruction",
					map[string]any{"idHash": idHash, "digest": layer.Digest, "images": []string{},
						"size": layer.Size, "instruction": layer.Instruction},
				)

				if err != nil {
					return nil, err
				}

				return result.Consume(ctx)
			}
		} else {
			return func(tx neo4j.ManagedTransaction) (any, error) {
				var result, err = tx.Run(ctx,
					"MERGE (l:Layer {id: $idHash}) "+
						"ON CREATE SET l.digest=$digest, l.images=$images, l.size=$size, l.instruction=$instruction "+
						"WITH l "+
						"MERGE (rl:RawLayer {digest: $digest}) "+
						"ON CREATE SET rl.size=$size, rl.instruction=$instruction "+
						"WITH l,rl "+
						"MERGE (l)-[:IS_SAME_AS]-(rl)",
					map[string]any{"idHash": idHash, "digest": layer.Digest, "images": []string{},
						"size": layer.Size, "instruction": layer.Instruction},
				)

				if err != nil {
					return nil, err
				}

				return result.Consume(ctx)
			}
		}
	} else {
		// 当前层非镜像第一层，需要插入层节点、边previous-->current
		// 配置命令对应的层不创建RawLayer
		if layer.Digest == "" {
			return func(tx neo4j.ManagedTransaction) (any, error) {
				var result, err = tx.Run(ctx,
					"MERGE (l:Layer {id: $idHash}) "+
						"ON CREATE SET l.digest=$digest, l.images=$images, l.size=$size, l.instruction=$instruction "+
						"WITH l "+
						"MATCH (previous:Layer {id: $previousHash}) "+
						"MERGE (previous)-[:IS_BASE_OF]->(l)",
					map[string]any{"previousHash": previousHash, "idHash": idHash, "digest": layer.Digest, "images": []string{},
						"size": layer.Size, "instruction": layer.Instruction},
				)

				if err != nil {
					return nil, err
				}

				return result.Consume(ctx)
			}
		} else {
			return func(tx neo4j.ManagedTransaction) (any, error) {
				var result, err = tx.Run(ctx,
					"MERGE (l:Layer {id: $idHash}) "+
						"ON CREATE SET l.digest=$digest, l.images=$images, l.size=$size, l.instruction=$instruction "+
						"WITH l "+
						"MERGE (rl:RawLayer {digest: $digest}) "+
						"ON CREATE SET rl.size=$size, rl.instruction=$instruction "+
						"WITH l,rl "+
						"MERGE (l)-[:IS_SAME_AS]-(rl) "+
						"WITH l "+
						"MATCH (previous:Layer {id: $previousHash}) "+
						"MERGE (previous)-[:IS_BASE_OF]->(l)",
					map[string]any{"previousHash": previousHash, "idHash": idHash, "digest": layer.Digest, "images": []string{},
						"size": layer.Size, "instruction": layer.Instruction},
				)

				if err != nil {
					return nil, err
				}

				return result.Consume(ctx)
			}
		}
	}
}

// addImageToLayerFunc 返回可用于session.ExecuteWrite的func，将image添加到最顶层
func addImageToLayerFunc(ctx context.Context, imageName, idHash string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:Layer {id: $idHash}) "+
				"SET l.images = CASE WHEN NOT $imageName IN l.images THEN l.images + $imageName "+
				"ELSE l.images "+
				"END",
			map[string]any{"idHash": idHash, "imageName": imageName},
		)

		if err != nil {
			return nil, err
		}

		return result.Consume(ctx)
	}
}

// FindLayerByNodeId 根据node id查找Layer节点
func (neo4jDriver *MyNeo4j) FindLayerByNodeId(nodeId string) (*neo4j.Record, error) {
	ctx := context.Background()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	node, err := session.ExecuteRead(ctx, findLayerByNodeIdFunc(ctx, nodeId))
	if err != nil {
		return nil, err
	}

	return node.(*neo4j.Record), nil
}

// findLayerByNodeIdFunc 返回可用于session.ExecuteRead的func，根据节点id查找Layer节点
func findLayerByNodeIdFunc(ctx context.Context, nodeId string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:Layer {id: $nodeId}) RETURN l",
			map[string]any{"nodeId": nodeId},
		)

		if err != nil {
			return nil, err
		}

		// 必须通过Next消费获取下一个Record
		if result.Next(context.TODO()) {
			return result.Record(), nil
		} else {
			return nil, fmt.Errorf("ExecuteRead got no record of (:Layer {id: %s}) in neo4j", nodeId)
		}
	}
}

//// FindRawLayerByDigest TODO: 待测试
//func (neo4jDriver *MyNeo4j) FindRawLayerByDigest(digest string) (*LayerResult, error) {
//	ctx := context.Background()
//	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
//	defer session.Close(ctx)
//
//	layerNode, err := session.ExecuteRead(ctx, findRawLayerByDigestFunc(ctx, digest))
//	if err != nil {
//		return nil, err
//	}
//
//	prop := GetNodeProps(layerNode.(*neo4j.Record))
//	if scanned, ok := prop["scanned"]; ok {
//		if !scanned.(bool) {
//			return nil, &LayerNotScannedError{Msg: digest}
//		}
//
//		lr := &LayerResult{}
//		if analyzedFiles, ok := prop["analyzed_files"]; ok {
//			fmt.Println("RawLayer", digest, "analyzed_files:", analyzedFiles)
//			lr.AnalyzedFiles = analyzedFiles.([]string)
//		}
//		if fileIssues, ok := prop["file_issues"]; ok {
//			fmt.Println("RawLayer", digest, "file_issues:", fileIssues)
//			lr.FileIssues = fileIssues.(map[string][]*Issue)
//		}
//
//		return lr, nil
//	}
//
//	return nil, &LayerNotExistsError{Msg: digest}
//}

// findRawLayerByDigestFunc 返回可用于session.ExecuteRead的func，find RawLayer Nodes according to digest
func findRawLayerByDigestFunc(ctx context.Context, digest string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:RawLayer {id: $digest}) RETURN l",
			map[string]any{"digest": digest},
		)

		if err != nil {
			return nil, err
		}

		record := result.Record()

		return record, nil
	}
}

// FindUpstreamImagesByNodeId 根据hash(1-2-5)发现Layer节点的上游镜像，组织为[]string并返回
func (neo4jDriver *MyNeo4j) FindUpstreamImagesByNodeId(nodeId string) ([]string, error) {
	result := make([]string, 0)
	imageSet := make(map[string]struct{})

	ctx := context.TODO()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	nodes, err := session.ExecuteRead(ctx, findUpstreamNodesWithImagesByNodeIdFunc(ctx, nodeId))
	if err != nil {
		Logger.Error("Neo4j find upstream Layer nodes with images by node id", nodeId, "failed with:", err.Error())
		return nil, err
	}

	for _, node := range nodes.([]*neo4j.Record) {
		prop := GetNodeProps(node)
		if imagesList, ok := prop["images"]; ok && len(imagesList.([]interface{})) > 0 {
			for _, imageName := range imagesList.([]interface{}) {
				imgNameStr := imageName.(string)
				imageSet[imgNameStr] = struct{}{}
			}
		}
	}

	for k, _ := range imageSet {
		result = append(result, k)
	}

	return result, nil
}

// findUpstreamNodesWithImagesByNodeIdFunc 返回可用于session.ExecuteRead的func，
// 查询返回images属性非空的上游Layer节点according to hash(1-2-5)
func findUpstreamNodesWithImagesByNodeIdFunc(ctx context.Context, nodeId string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:Layer {id: $idHash}) "+
				"WITH l "+
				"MATCH (up:Layer)-[:IS_BASE_OF*]->(l) "+
				"WHERE size(up.images)>0 "+
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

// FindUpstreamLayerNodesByNodeId 根据hash(1-2-5)发现所有上游Layer节点
func (neo4jDriver *MyNeo4j) FindUpstreamLayerNodesByNodeId(nodeId string) (any, error) {

	ctx := context.TODO()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	upNodes, err := session.ExecuteRead(ctx, findUpstreamNodesByNodeIdFunc(ctx, nodeId))
	if err != nil {
		Logger.Error("Neo4j find upstream Layer nodes by node id", nodeId, "failed with:", err.Error())
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

// FindDownstreamImagesByNodeId 根据hash(1-2-5)发现Layer节点的下游镜像，组织为[]string并返回
func (neo4jDriver *MyNeo4j) FindDownstreamImagesByNodeId(nodeId string) ([]string, error) {
	result := make([]string, 0)
	imageSet := make(map[string]struct{})

	ctx := context.TODO()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	nodes, err := session.ExecuteRead(ctx, findDownstreamNodesWithImagesByNodeIdFunc(ctx, nodeId))
	if err != nil {
		Logger.Error("Neo4j find downstream Layer nodes with images by node id", nodeId, "failed with:", err.Error())
		return nil, err
	}

	for _, node := range nodes.([]*neo4j.Record) {
		prop := GetNodeProps(node)
		if imagesList, ok := prop["images"]; ok && len(imagesList.([]interface{})) > 0 {
			for _, imageName := range imagesList.([]interface{}) {
				strName := imageName.(string)
				imageSet[strName] = struct{}{}
			}
		}
	}

	for k, _ := range imageSet {
		result = append(result, k)
	}

	return result, nil
}

// findDownstreamNodesWithImagesByNodeIdFunc 返回可用于session.ExecuteRead的func，
// 查询返回images属性非空的下游Layer节点according to hash(1-2-5)
func findDownstreamNodesWithImagesByNodeIdFunc(ctx context.Context, nodeId string) neo4j.ManagedTransactionWork {

	return func(tx neo4j.ManagedTransaction) (any, error) {
		var result, err = tx.Run(ctx,
			"MATCH (l:Layer {id: $idHash}) "+
				"WITH l "+
				"MATCH (l)-[:IS_BASE_OF*]->(down:Layer) "+
				"WHERE size(down.images)>0 "+
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

// FindDownstreamLayerNodesByNodeId 根据hash(1-2-5)发现所有下游Layer节点
func (neo4jDriver *MyNeo4j) FindDownstreamLayerNodesByNodeId(nodeId string) (any, error) {

	ctx := context.TODO()
	session := neo4jDriver.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	nodes, err := session.ExecuteRead(ctx, findDownstreamNodesByNodeIdFunc(ctx, nodeId))
	if err != nil {
		Logger.Error("Neo4j find downstream Layer nodes by node id", nodeId, "failed with:", err.Error())
		return nil, err
	}

	return nodes, nil
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

// GetNodeProps 解析neo4j driver ExecuteRead返回*neo4j.Record节点属性
func GetNodeProps(n *neo4j.Record) map[string]any {
	keys := n.Keys
	if len(keys) == 1 {
		prop, _ := n.Get(keys[0])
		return prop.(dbtype.Node).Props
	}

	return nil
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

func CalculateImageNodeId(img *Image) string {
	preId := ""

	for _, layer := range img.Layers {
		dig := ""

		if layer.Digest != "" {
			dig = Sha256Str(layer.Digest)
		} else {
			dig = Sha256Str(layer.Instruction)
		}
		if dig == "" {
			break
		}

		preId = Sha256Str(preId + dig)
	}

	return preId
}

// CalculateImageNodeIdOld 用于计算镜像顶层节点所在的node id，未考虑镜像中的配置层，为旧版依赖图实现
// CalculateImageNodeIdOld calculates node-id of top layer
// (with real file contents) in the image layer-to-layer chain.
func CalculateImageNodeIdOld(image *ImageOld) string {
	accumulateLayerID := ""
	for _, layer := range image.Layers {
		if layer.Size == 0 {
			continue
		}
		accumulateLayerID += layer.Digest
	}
	accumulateHash := Sha256Str(accumulateLayerID)

	return accumulateHash
}

func IsLayerNotScannedError(err error) bool {
	_, is := err.(*LayerNotScannedError)
	return is
}
