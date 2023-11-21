package myutils

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
)

var GlobalConfig struct {
	MaxThread        int              `yaml:"max_thread"`
	RepositoriesFile string           `yaml:"repositories_file"`
	TagsFile         string           `yaml:"tags_file"`
	ImagesFile       string           `yaml:"images_file"`
	LogFile          string           `yaml:"log_file"`
	TmpDir           string           `yaml:"tmp_dir"`
	CrawlerConfig    CrawlerConfig    `yaml:"crawler_config"`
	MongoConfig      MongoConfig      `yaml:"mongo_config"`
	Neo4jConfig      Neo4jConfig      `yaml:"neo4j_config"`
	RulesConfig      RulesConfig      `yaml:"rules_config"`
	AskyConfig       AskyConfig       `yaml:"asky_config"`
	TrufflehogConfig TrufflehogConfig `yaml:"trufflehog_config"`
}

type CrawlerConfig struct {
	LocalProxy bool   `yaml:"local_proxy"`
	ProxyFile  string `yaml:"proxy_file"`
	MysqlDSN   string `yaml:"mysql_dsn"`
}

type MongoConfig struct {
	URI         string           `yaml:"uri"`
	Database    string           `yaml:"database"`
	Collections MongoCollections `yaml:"collections"`
}

type MongoCollections struct {
	Repositories string `yaml:"repositories"`
	Tags         string `yaml:"tags"`
	Images       string `yaml:"images"`
	ImageResults string `yaml:"image_results"`
	LayerResults string `yaml:"layer_results"`
}

type Neo4jConfig struct {
	Neo4jURI      string `yaml:"neo4j_uri"`
	Neo4jUsername string `yaml:"neo4j_username"`
	Neo4jPassword string `yaml:"neo4j_password"`
}

type RulesConfig struct {
	SecretRulesFile         string `yaml:"secret_rules_file"`
	SensitiveParamRulesFile string `yaml:"sensitive_param_rules_file"`
}

type AskyConfig struct {
	AskyFile  string `yaml:"filepath"`
	AskyToken string `yaml:"token"`
}

type TrufflehogConfig struct {
	Filepath string `yaml:"filepath"`
	Verify   bool   `yaml:"verify"`
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
	configFile := path.Join(root, "config.yaml")

	// 加载config.yaml
	fb, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalln("[ERROR] Failed to load ", configFile)
	}
	if err = yaml.Unmarshal(fb, &GlobalConfig); err != nil {
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
	//connectDBs()
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

// connectDBs connects MongoDB and Neo4j based on config.yaml
func connectDBs() {
	var err error

	// 连接MongoDB
	// TODO: Mongo Timeout设置不正确，目前不生效
	if GlobalDBClient.Mongo, err = NewMongoGlobalConfig(); err != nil {
		GlobalDBClient.MongoFlag = false
		Logger.Error("connect to MongoDB failed with:", err.Error())
		fmt.Println("[-] Connect to MongoDB failed")
	} else {
		GlobalDBClient.MongoFlag = true
		fmt.Println("[+] Connect to MongoDB")
	}

	// 连接Neo4j
	// TODO: Neo4j连接返回的err不正确，目前永远为nil
	if GlobalDBClient.Neo4j, err = NewNeo4jDriverGlobalConfig(); err != nil {
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
