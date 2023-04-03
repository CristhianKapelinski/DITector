package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
)

func init() {
	dsn := "docker:docker@%s/%s"

	// 初始化创建新的database，命名为dockerhub
	// 默认data source name
	db, err := sql.Open("mysql", fmt.Sprintf(dsn, "tcp(localhost:3306)", ""))
	if err != nil {
		log.Fatalln("[ERROR] Open mysql failed with err: ", err)
	}
	defer db.Close()
	fmt.Println("[+] Open mysql Success.")
	if err := db.Ping(); err != nil {
		log.Fatalln("[ERROR] Ping mysql database failed with err: ", err)
	}
	fmt.Println("[+] Ping mysql Success.")

	createDatabase := `CREATE DATABASE IF NOT EXISTS dockerhub`
	_, err = db.Exec(createDatabase)
	if err != nil {
		log.Fatalln("[ERROR] CREATE DATABASE dockerhub failed with err: ", err)
	} else {
		fmt.Println("[+] Create db dockerhub success.")
	}

	// 初始化dockerhub数据库内的数据表
	db2, err := sql.Open("mysql", fmt.Sprintf(dsn, "tcp(localhost:3306)", "dockerhub"))
	if err != nil {
		log.Fatalln("[ERROR] Open DATABASE dockerhub failed with err: ", err)
	}
	defer db2.Close()
	fmt.Println("[+] Open DATABASE dockerhub Success.")
	if err := db2.Ping(); err != nil {
		log.Fatalln("[ERROR] Ping DATABASE dockerhub failed with err: ", err)
	}
	fmt.Println("[+] Ping database dockerhub Success.")

	// 创建keywords表
	createKeywords := `
CREATE TABLE IF NOT EXISTS keywords
(
    name VARCHAR(255) UNIQUE 
);`
	_, err = db2.Exec(createKeywords)
	if err != nil {
		log.Fatalln("[ERROR] CREATE TABLE keywords failed with err: ", err)
	} else {
		fmt.Println("[+] CREATE TABLE keywords success.")
	}

	// 创建repository表
	createRepository := `
CREATE TABLE IF NOT EXISTS repository
(
    user VARCHAR(255),
    name VARCHAR(255),
    namespace VARCHAR(255),
    repository_type VARCHAR(255),
    description TEXT,
	flag TINYINT,
	star_count INT,
	pull_count BIGINT,
	last_updated TIMESTAMP,
	date_registered TIMESTAMP,
	full_description LONGTEXT,
	PRIMARY KEY (namespace,name)
);`
	_, err = db2.Exec(createRepository)
	if err != nil {
		log.Fatalln("[ERROR] CREATE TABLE repository failed with err: ", err)
	} else {
		fmt.Println("[+] CREATE TABLE repository success.")
	}

	// 创建tags表
	createTags := `
CREATE TABLE IF NOT EXISTS tags
(
    namespace VARCHAR(255),
    repository VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    last_updated TIMESTAMP,
    last_updater_username VARCHAR(255),
    tag_last_pulled TIMESTAMP,
    tag_last_pushed TIMESTAMP,
    media_type TINYTEXT,
    content_type TINYTEXT,
    FOREIGN KEY (namespace,repository) REFERENCES repository(namespace,name)
);`
	_, err = db2.Exec(createTags)
	if err != nil {
		log.Fatalln("[ERROR] CREATE TABLE tags failed with err: ", err)
	} else {
		fmt.Println("[+] CREATE TABLE tags success.")
	}

	// 创建images表，真正对应到image上，包含层信息，来自Arch__
	createImages := `
CREATE TABLE IF NOT EXISTS images
(
    namespace VARCHAR(255),
    repository VARCHAR(255),
    tag VARCHAR(255) NOT NULL,
    architecture TINYTEXT,
    features TINYTEXT,
    variant TINYTEXT,
    digest CHAR(64) NOT NULL,
    os TINYTEXT,
    size BIGINT,
    status VARCHAR(8),
    last_pulled TIMESTAMP,
    last_pushed TIMESTAMP,
    layers LONGTEXT,
    FOREIGN KEY (namespace,repository) REFERENCES repository(namespace,name)
);`
	_, err = db2.Exec(createImages)
	if err != nil {
		log.Fatalln("[ERROR] CREATE TABLE images failed with err: ", err)
	} else {
		fmt.Println("[+] CREATE TABLE images success.")
	}

	// 创建layers表
	createLayers := `
CREATE TABLE IF NOT EXISTS layers
(
    digest CHAR(64) PRIMARY KEY,
    size BIGINT,
    instruction TEXT
);`
	_, err = db2.Exec(createLayers)
	if err != nil {
		log.Fatalln("[ERROR] CREATE TABLE layers failed with err: ", err)
	} else {
		fmt.Println("[+] CREATE TABLE layers success.")
	}
}
