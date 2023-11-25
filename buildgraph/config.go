package buildgraph

import (
	"encoding/json"
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
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
	configFile := root + "/config.yaml"
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
	myMongo, err = myutils.ConfigMongoClient(false)
	if err != nil {
		log.Fatalln("[ERROR] connect to and config MongoDB failed with err: ", err)
	}
	fmt.Println("[+] Connect to MongoDB succeed")

	// Neo4j
	myNeo4jDriver, err = myutils.NewNeo4jDriver(
		ConfigBuilder.Builder.Neo4jURI,
		ConfigBuilder.Builder.Neo4jUsername,
		ConfigBuilder.Builder.Neo4jPassword,
		false,
	)
	if err != nil {
		log.Fatalln("[ERROR] Connect to neo4j failed with:", err)
	}
	fmt.Println("[+] Connect to Neo4j succeed")

	// Deprecated
	// 初始化日志文件fd
	//myutils.fileBuilderLogger, err = os.OpenFile(path.Join(ConfigBuilder.DataDir, "builder.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	//if err != nil {
	//	log.Fatalf("[ERROR] Open %s failed with: %s\n", path.Join(ConfigBuilder.DataDir, "builder.log"), err)
	//} else {
	//	fmt.Println("[+] Open log file succeed: ", path.Join(ConfigBuilder.DataDir, "builder.log"))
	//}

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
		// deprecated: mysql性能低下
		// 初始化mysql connector
	case "count":
		// 统计数据库信息并打印
		fmt.Println("[+] get statistics of MongoDB and Neo4j")
		statistics, err := myMongo.GetAllDocumentsCount()
		if err != nil {
			fmt.Println("[-] get document statistics failed with err:", err)
		} else {
			fmt.Println("-------------------------------------------------")
			fmt.Println("MongoDB:")
			for k, v := range statistics {
				fmt.Println("\t", k, ":", v)
			}
		}
	// 数据已成规模，禁用clear
	//case "clear":
	//	// 删除数据库中的数据
	//	myMongo.DropAllDocuments()
	//	myNeo4jDriver.DropNodesAndRelationshipsFromNeo4j()
	//	fmt.Println("[-] clean data from MongoDB and Neo4j")
	//	myutils.LogDockerCrawlerString("[WARN] Clean Database Mongo and Neo4j")
	default:
		fmt.Println("[ERROR] Invalid data source configured: ", format)
		os.Exit(-2)
	}
}
