package db

import (
	"crawler"
	"fmt"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	time.Sleep(1 * time.Second)
}

func TestNewDockerDB(t *testing.T) {
	db, err := NewDockerDB("docker:docker@tcp(localhost:3306)/dockerhub")
	defer db.db.Close()
	if err != nil {
		t.Fatal("[ERROR] Getting DockerDB: ", err)
	}
	err = db.db.Ping()
	if err != nil {
		t.Fatal("[ERROR] Ping DockerDB.db failed with: ", err)
	}
	fmt.Println("[+] TestNewDockerDB Pass!")
}

func TestDockerDB_InsertKeyword(t *testing.T) {
	db, err := NewDockerDB("docker:docker@/dockerhub")
	defer db.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}
	r, err := db.InsertKeyword("zzzz")
	if err != nil {
		t.Fatal("[ERROR] insert keyword failed with: ", err)
	}
	fmt.Println(r.RowsAffected())

	r, err = db.InsertKeyword("zzzzz")
	if err != nil {
		t.Fatal("[ERROR] insert keyword failed with: ", err)
	}
	fmt.Println(r.RowsAffected())

	r, err = db.InsertKeyword("zzzza")
	if err != nil {
		t.Fatal("[ERROR] insert keyword failed with: ", err)
	}
	fmt.Println(r.RowsAffected())
}

func TestDockerDB_GetLastKeyword(t *testing.T) {
	db, err := NewDockerDB("docker:docker@/dockerhub")
	defer db.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(db.GetLastKeyword())
}

// TODO: Insert Test废了，因为db和crawler循环依赖了，懒得写测试用例了，在crawler那边测试吧
func TestDockerDB_InsertRepository__(t *testing.T) {
	db, err := NewDockerDB("docker:docker@/dockerhub")
	defer db.Close()
	if err != nil {
		t.Fatal(err)
	}

	var r crawler.Repository__
	c := crawler.GetRepoMetadataCollector(&r)
	c.Visit(crawler.GetRepoMetaURL("patsissons", "xmrig"))

	res, err := crawler.StoreRepository__(&r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(res.RowsAffected())
}

// TODO: Insert Test废了，因为db和crawler循环依赖了，懒得写测试用例了，在crawler那边测试吧
func TestDockerDB_InsertTag__(t *testing.T) {
	db, err := NewDockerDB("docker:docker@/dockerhub")
	defer db.Close()
	if err != nil {
		t.Fatal(err)
	}

	var raw_json = `{
    "creator": 754309,
    "id": 24302077,
    "images": [
        {
            "architecture": "amd64",
            "features": "",
            "variant": null,
            "digest": "sha256:401e00ed6390984d94cecadd453009420524011c365912f9a6d1c705844e7e7e",
            "os": "linux",
            "os_features": "",
            "os_version": null,
            "size": 1100925349,
            "status": "active",
            "last_pulled": "2023-04-01T07:00:15.487441Z",
            "last_pushed": "2021-01-05T21:42:07.484014Z"
        }
    ],
    "last_updated": "2021-01-05T21:42:07.484014Z",
    "last_updater": 754309,
    "last_updater_username": "patsissons",
    "name": "latest",
    "repository": 4946896,
    "full_size": 1100925349,
    "v2": true,
    "tag_status": "active",
    "tag_last_pulled": "2023-04-01T07:00:15.487441Z",
    "tag_last_pushed": "2021-01-05T21:42:07.484014Z",
    "media_type": "application/vnd.docker.container.image.v1+json",
    "content_type": "image"
}`
	fmt.Println(raw_json)
}
