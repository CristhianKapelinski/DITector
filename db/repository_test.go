package db

import (
	"crawler"
	"fmt"
	"testing"
)

func TestDockerDB_InsertRepository__(t *testing.T) {
	db, err := NewDockerDB("docker:docker@/dockerhub")
	defer db.Close()
	if err != nil {
		t.Fatal(err)
	}

	var r crawler.Repository__
	c := crawler.GetRepoMetadataCollector(&r)
	c.Visit(crawler.GetRepoMetaURL("library", "mongo"))

	res, err := db.InsertRepository__(r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(res.RowsAffected())
}
