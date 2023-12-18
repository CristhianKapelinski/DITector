package buildgraph

import (
	"fmt"
	"log"
	"os"
	"path"
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
	var err error

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
	case "mongo":
		// 目前没什么要做的
	case "mysql":
		// deprecated: mysql性能低下
		// 初始化mysql connector
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
