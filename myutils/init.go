package myutils

import (
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
}

type CrawlerConfig struct {
	LocalProxy bool   `json:"local_proxy"`
	ProxyFile  string `json:"proxy_file"`
	MysqlDSN   string `json:"mysql_dsn"`
}

type MongoConfig struct {
	URI      string `json:"uri"`
	Database string `json:"database"`
}

type Neo4jConfig struct {
	Neo4jURI      string `json:"neo4j_uri"`
	Neo4jUsername string `json:"neo4j_username"`
	Neo4jPassword string `json:"neo4j_password"`
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
		fmt.Println("[+] Open log file succeed: ", logFilepath)
	}

}
