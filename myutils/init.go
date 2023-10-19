package myutils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
)

var GlobalConfig struct {
	MaxThread        int           `json:"max_thread"`
	DataDir          string        `json:"data_dir"`
	RepositoriesFile string        `json:"repositories_file"`
	TagsFile         string        `json:"tags_file"`
	ImagesFile       string        `json:"images_file"`
	LogFile          string        `json:"log_file"`
	CrawlerConfig    CrawlerConfig `json:"crawler_config"`
	MongoConfig      MongoConfig   `json:"mongo_config"`
	Neo4jConfig      Neo4jConfig   `json:"neo4j_config"`
	RulesConfig      RulesConfig   `json:"rules_config"`
}

type CrawlerConfig struct {
	LocalProxy bool   `json:"local_proxy"`
	ProxyFile  string `json:"proxy_file"`
	MysqlDSN   string `json:"mysql_dsn"`
}

type MongoConfig struct {
	URI         string           `json:"uri"`
	Database    string           `json:"database"`
	Collections MongoCollections `json:"collections"`
}

type MongoCollections struct {
	Repositories string `json:"repositories"`
	Tags         string `json:"tags"`
	Images       string `json:"images"`
	Results      string `json:"results"`
}

type Neo4jConfig struct {
	Neo4jURI      string `json:"neo4j_uri"`
	Neo4jUsername string `json:"neo4j_username"`
	Neo4jPassword string `json:"neo4j_password"`
}

type RulesConfig struct {
	SecretRulesFile string `json:"secret_rules_file"`
}

// GlobalDBClient 用于维护全局所有模块的数据库client连接
var GlobalDBClient struct {
	Mongo     *MyMongo
	MongoFlag bool // 标识Mongo是否成功连接
	Neo4j     *MyNeo4j
	Neo4jFlag bool // 标识Neo4j是否成功连接
}

func init() {
	// 获取程序根目录
	_, filename, _, _ := runtime.Caller(0)
	root := path.Dir(path.Dir(filename))
	configFile := root + "/config.json"

	// 加载config.json
	fb, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalln("[ERROR] Failed to load ", configFile)
	}
	if err = json.Unmarshal(fb, &GlobalConfig); err != nil {
		log.Fatalf("[ERROR] Json failed to unmarshal %s with err: %v\n", configFile, err)
	}

	// 初始化日志模块
	//logFilePath := "/data/docker-crawler/docker-crawler.log"
	logFilepath := GlobalConfig.LogFile
	if err = configLogger(logFilepath); err != nil {
		log.Fatalf("[ERROR] Open %s failed with: %s\n", logFilepath, err)
	} else {
		fmt.Println("[+] Open log file: ", logFilepath)
	}

	// 连接MongoDB
	if GlobalDBClient.Mongo, err = NewMongo(GlobalConfig.MongoConfig.URI, GlobalConfig.MongoConfig.Database,
		GlobalConfig.MongoConfig.Collections.Repositories, GlobalConfig.MongoConfig.Collections.Tags,
		GlobalConfig.MongoConfig.Collections.Images, GlobalConfig.MongoConfig.Collections.Results,
		false); err != nil {
		GlobalDBClient.MongoFlag = false
		LogDockerCrawlerString(LogLevel.Error, "connect to MongoDB failed with:", err.Error())
		fmt.Println("[-] Connect to MongoDB failed")
	} else {
		GlobalDBClient.MongoFlag = true
		fmt.Println("[+] Connect to MongoDB")
	}

	// 连接Neo4j
	if GlobalDBClient.Neo4j, err = NewNeo4jDriver(GlobalConfig.Neo4jConfig.Neo4jURI, GlobalConfig.Neo4jConfig.Neo4jUsername,
		GlobalConfig.Neo4jConfig.Neo4jPassword, false); err != nil {
		GlobalDBClient.Neo4jFlag = false
		LogDockerCrawlerString(LogLevel.Error, "connect to Neo4j failed with:", err.Error())
		fmt.Println("[-] Connect to Neo4j failed")
	} else {
		GlobalDBClient.Neo4jFlag = true
		fmt.Println("[+] Connect to Neo4j")
	}
}

// CloseAllConnections 用于在主函数退出前关闭所有占用的资源
func CloseAllConnections() {
	// disconnect MongoDB
	if GlobalDBClient.MongoFlag {
		if err := GlobalDBClient.Mongo.Client.Disconnect(context.TODO()); err != nil {
			LogDockerCrawlerString(LogLevel.Error, "disconnect MongoDB client failed with:", err.Error())
		}
	}

	// close Neo4j
	if GlobalDBClient.Neo4jFlag {
		if err := GlobalDBClient.Neo4j.Driver.Close(context.TODO()); err != nil {
			LogDockerCrawlerString(LogLevel.Error, "close Neo4j driver failed with:", err.Error())
		}
	}

	// close logger file
	if err := CloseLogger(); err != nil {
		log.Fatalln(LogLevel.Error, "close log file", GlobalConfig.LogFile, "failed with:", err.Error())
	}
}
