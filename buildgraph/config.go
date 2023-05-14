package buildgraph

import (
	"context"
	"encoding/json"
	"fmt"
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
	BuilderLogFile string `json:"builder_log_file"`
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
		log.Fatalln("[ERROR] Failed to connect to MongoDB with err: ", err)
	}
	err = mongoClient.Ping(context.Background(), nil)
	if err != nil {
		log.Fatalln("[ERROR] Failed to ping MongoDB with err: ", err)
	}
	mongoRepositoryCollection = mongoClient.Database("dockerhub").Collection("repository")
	// 建立唯一索引，防止插入重复数据
	indexView := mongoRepositoryCollection.Indexes()
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
			{Key: "repository", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = indexView.CreateOne(context.Background(), model)
	if err != nil {
		log.Fatalln("[ERROR] Create unique index on mongodb failed with:", err)
	}
	fmt.Println("[+] Connect to MongoDB succeed")

	// TODO: 初始化neo4j connector，建立节点唯一索引，防止重复插入layerid

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
	default:
		fmt.Println("[ERROR] Invalid data source configured: ", format)
		os.Exit(-2)
	}

	// 初始化日志文件fd
	fileBuilderLogger, err = os.OpenFile(path.Join(ConfigBuilder.DataDir, ConfigBuilder.BuilderLogFile), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	if err != nil {
		log.Fatalf("[ERROR] Open %s failed with: %s\n", path.Join(ConfigBuilder.DataDir, ConfigBuilder.BuilderLogFile), err)
	} else {
		fmt.Println("[+] Open log file succeed: ", path.Join(ConfigBuilder.DataDir, ConfigBuilder.BuilderLogFile))
	}
}
