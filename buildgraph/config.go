package buildgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"path"
	"runtime"
)

var ConfigBuilder struct {
	MaxThread      int    `json:"max_thread"`
	DataDir        string `json:"data_dir"`
	RepositoryFile string `json:"repository_file"`
	TagsFile       string `json:"tags_file"`
	ImagesFile     string `json:"images_file"`
	Builder        URIS   `json:"builder"`
}

type URIS struct {
	Neo4jURI      string `json:"neo4j_uri"`
	Neo4jUsername string `json:"neo4j_username"`
	Neo4jPassword string `json:"neo4j_password"`
}

func config(format string) {
	// 获取程序根目录
	_, filename, _, _ := runtime.Caller(0)
	root := path.Dir(path.Dir(filename))
	configFile := root + "/config.json"
	// 加载Config
	fb, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println("[ERROR] Failed to load ", configFile)
	}
	if err = json.Unmarshal(fb, &ConfigBuilder); err != nil {
		fmt.Printf("[ERROR] Json failed to unmarshal %s with err: %v\n", configFile, err)
	}
	// 默认情况下，允许启动的核心goroutine数为系统可调内核数
	if ConfigBuilder.MaxThread <= 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
		// runtime.GOMAXPROCS 返回的是设置成功之前的GOMAXPROCS，所以要再设一次获取上一次获取成功的数
		ConfigBuilder.MaxThread = runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(ConfigBuilder.MaxThread)
	}

	fmt.Println("[+] Init Builder Config Success: ", ConfigBuilder)

	// 初始化限制最大goroutine数的全局管道
	chanLimitMainGoroutine = make(chan struct{}, ConfigBuilder.MaxThread)

	// 初始化数据库connector
	// Mongo
	mongoOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	mongoClient, err = mongo.Connect(context.Background(), mongoOptions)
	if err != nil {
		log.Fatalln("[ERROR] Connect to MongoDB failed with err: ", err)
	}
	err = mongoClient.Ping(context.Background(), nil)
	if err != nil {
		log.Fatalln("[ERROR] Ping MongoDB failed with err: ", err)
	}
	// mongoRepositoryCollection 用于存repository的元数据
	mongoRepositoryCollection = mongoClient.Database("dockerhub").Collection("repository")
	// 建立唯一索引，namespace-repository防止插入重复数据
	repoIndexView := mongoRepositoryCollection.Indexes()
	repoModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
			{Key: "repository", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = repoIndexView.CreateOne(context.Background(), repoModel)
	if err != nil {
		log.Fatalln("[ERROR] Create unique index on mongodb.dockerhub.repository failed with:", err)
	}
	// mongoImagesCollection 用于存image的层信息
	mongoImagesCollection = mongoClient.Database("dockerhub").Collection("images")
	// 建立唯一索引digest，防止插入重复数据
	imageIndexView := mongoImagesCollection.Indexes()
	imageModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "digest", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = imageIndexView.CreateOne(context.Background(), imageModel)
	if err != nil {
		log.Fatalln("[ERROR] Create unique index on mongodb.dockerhub.images failed with:", err)
	}
	fmt.Println("[+] Connect to MongoDB succeed")

	// Neo4j
	neo4jDriver, err = neo4j.NewDriverWithContext(
		ConfigBuilder.Builder.Neo4jURI,
		neo4j.BasicAuth(ConfigBuilder.Builder.Neo4jUsername, ConfigBuilder.Builder.Neo4jPassword, ""),
	)
	if err != nil {
		log.Fatalln("[ERROR] Connect to neo4j failed with:", err)
	}
	// 创建索引，neo4j没有提供判断重复创建索引导致报错的函数，所以不处理err
	session := neo4jDriver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())
	session.ExecuteWrite(context.Background(), func(tx neo4j.ManagedTransaction) (any, error) {
		// 创建索引：基于节点id
		tx.Run(context.Background(),
			"CREATE INDEX layer_id_index IF NOT EXISTS FOR (l:Layer) ON (l.id)",
			map[string]any{},
		)

		// 创建索引：基于节点layer-id
		tx.Run(context.Background(),
			"CREATE INDEX layer_digest_index IF NOT EXISTS FOR (l:Layer) ON (l.digest)",
			map[string]any{},
		)

		// 创建索引：基于节点layer-id
		tx.Run(context.Background(),
			"CREATE INDEX rawlayer_digest_index IF NOT EXISTS FOR (l:RawLayer) ON (l.digest)",
			map[string]any{},
		)

		return nil, nil
	})
	fmt.Println("[+] Connect to Neo4j succeed")

	// 根据format连接数据源
	switch format {
	case "json":
		// 初始化json文件fd
		fileRepository, err = os.Open(path.Join(ConfigBuilder.DataDir, ConfigBuilder.RepositoryFile))
		if err != nil {
			log.Fatalf("[ERROR] Open %s failed with: %s\n", path.Join(ConfigBuilder.DataDir, "repository.json"), err)
		} else {
			fmt.Println("[+] Open source file succeed: ", path.Join(ConfigBuilder.DataDir, ConfigBuilder.RepositoryFile))
		}
		fileTags, err = os.Open(path.Join(ConfigBuilder.DataDir, ConfigBuilder.TagsFile))
		if err != nil {
			log.Fatalf("[ERROR] Open %s failed with: %s\n", path.Join(ConfigBuilder.DataDir, "tags.json"), err)
		} else {
			fmt.Println("[+] Open source file succeed: ", path.Join(ConfigBuilder.DataDir, ConfigBuilder.TagsFile))
		}
		fileImages, err = os.Open(path.Join(ConfigBuilder.DataDir, ConfigBuilder.ImagesFile))
		if err != nil {
			log.Fatalf("[ERROR] Open %s failed with: %s\n", path.Join(ConfigBuilder.DataDir, "images.json"), err)
		} else {
			fmt.Println("[+] Open source file succeed: ", path.Join(ConfigBuilder.DataDir, ConfigBuilder.ImagesFile))
		}
	case "mysql":
		// 初始化mysql connector
	case "clear":
		// 删除数据库中的数据
		DropRepositoryCollectionFromMongo()
		DropNodesAndRelationshipsFromNeo4j()
		logBuilderString("[WARN] Clear Database Mongo and Neo4j")
	default:
		fmt.Println("[ERROR] Invalid data source configured: ", format)
		os.Exit(-2)
	}

	// 初始化日志文件fd
	fileBuilderLogger, err = os.OpenFile(path.Join(ConfigBuilder.DataDir, "builder.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	if err != nil {
		log.Fatalf("[ERROR] Open %s failed with: %s\n", path.Join(ConfigBuilder.DataDir, "builder.log"), err)
	} else {
		fmt.Println("[+] Open log file succeed: ", path.Join(ConfigBuilder.DataDir, "builder.log"))
	}
}
