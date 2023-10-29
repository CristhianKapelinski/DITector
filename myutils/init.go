package myutils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
)

var GlobalConfig struct {
	MaxThread        int           `json:"max_thread"`
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
	SecretRulesFile         string `json:"secret_rules_file"`
	SensitiveParamRulesFile string `json:"sensitive_param_rules_file"`
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
	configFile := path.Join(root, "config.json")

	// 加载config.json
	fb, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalln("[ERROR] Failed to load ", configFile)
	}
	if err = json.Unmarshal(fb, &GlobalConfig); err != nil {
		log.Fatalf("[ERROR] Json failed to unmarshal %s with err: %v\n", configFile, err)
	}

	// 调整相对路径到绝对路径
	relativeToAbsoluteConfig(root)

	// 初始化日志模块
	//logFilePath := "/data/docker-crawler/docker-crawler.log"
	fmt.Println(GlobalConfig.LogFile)
	logFilepath := GlobalConfig.LogFile
	if err = configLogger(logFilepath, 1); err != nil {
		log.Fatalf("[ERROR] Open %s failed with: %s\n", logFilepath, err)
	} else {
		fmt.Println("[+] Open log file: ", logFilepath)
	}

	// 配置http代理
	configHTTPProxy()
	// 配置tls，跳过https证书验证
	configTLSConfig()

	// 初始化数据库连接
	connectDBs()
}

// relativeToAbsoluteConfig 将GlobalConfig中相对路径的部分调整为绝对路径
func relativeToAbsoluteConfig(root string) {
	if !strings.HasPrefix(GlobalConfig.LogFile, "/") {
		GlobalConfig.LogFile = path.Join(root, GlobalConfig.LogFile)
	}
	if !strings.HasPrefix(GlobalConfig.CrawlerConfig.ProxyFile, "/") {
		GlobalConfig.CrawlerConfig.ProxyFile = path.Join(root, GlobalConfig.CrawlerConfig.ProxyFile)
	}
	if !strings.HasPrefix(GlobalConfig.RulesConfig.SecretRulesFile, "/") {
		GlobalConfig.RulesConfig.SecretRulesFile = path.Join(root, GlobalConfig.RulesConfig.SecretRulesFile)
	}
	if !strings.HasPrefix(GlobalConfig.RulesConfig.SensitiveParamRulesFile, "/") {
		GlobalConfig.RulesConfig.SensitiveParamRulesFile = path.Join(root, GlobalConfig.RulesConfig.SensitiveParamRulesFile)
	}
}

// connectDBs connects MongoDB and Neo4j based on config.json
func connectDBs() {
	var err error

	// 连接MongoDB
	// TODO: Mongo Timeout设置不正确，目前不生效
	if GlobalDBClient.Mongo, err = NewMongo(GlobalConfig.MongoConfig.URI, GlobalConfig.MongoConfig.Database,
		GlobalConfig.MongoConfig.Collections.Repositories, GlobalConfig.MongoConfig.Collections.Tags,
		GlobalConfig.MongoConfig.Collections.Images, GlobalConfig.MongoConfig.Collections.Results,
		false); err != nil {
		GlobalDBClient.MongoFlag = false
		Logger.Error("connect to MongoDB failed with:", err.Error())
		fmt.Println("[-] Connect to MongoDB failed")
	} else {
		GlobalDBClient.MongoFlag = true
		fmt.Println("[+] Connect to MongoDB")
	}

	// 连接Neo4j
	// TODO: Neo4j连接返回的err不正确，目前永远为nil
	if GlobalDBClient.Neo4j, err = NewNeo4jDriver(GlobalConfig.Neo4jConfig.Neo4jURI, GlobalConfig.Neo4jConfig.Neo4jUsername,
		GlobalConfig.Neo4jConfig.Neo4jPassword, false); err != nil {
		GlobalDBClient.Neo4jFlag = false
		Logger.Error("connect to Neo4j failed with:", err.Error())
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
			Logger.Error("disconnect MongoDB client failed with:", err.Error())
		}
	}

	// close Neo4j
	if GlobalDBClient.Neo4jFlag {
		if err := GlobalDBClient.Neo4j.Driver.Close(context.TODO()); err != nil {
			Logger.Error("close Neo4j driver failed with:", err.Error())
		}
	}

	// close logger file
	if err := Logger.Close(); err != nil {
		log.Fatalln(LogLevelStr.Error, "close log file", GlobalConfig.LogFile, "failed with:", err.Error())
	}
}
