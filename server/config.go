package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/Musso12138/docker-scan/myutils"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	// totalCnt is the number of documents in each collection,
	// used for calculate table pages
	totalRepositoriesCnt int64
	totalImagesCnt       int64
	configLock           = sync.WaitGroup{}
)

func configServer() {
	configLock.Add(2)
	go func() {
		defer configLock.Done()
		updateRepositoriesCnt()
	}()
	go func() {
		defer configLock.Done()
		updateImagesCnt()
	}()
	configLock.Wait()
}

func updateRepositoriesCnt() error {
	//totalRepositoriesCnt, _ = myMongo.GetRepositoriesCountByText("")
	result := myutils.GlobalDBClient.Mongo.DockerHubDB.RunCommand(context.TODO(), bson.M{"collStats": myutils.GlobalConfig.MongoConfig.Collections.Repositories})
	stats := bson.M{}
	err := result.Decode(&stats)
	if err != nil {
		return err
	}
	totalRepositoriesCnt = stats["count"].(int64)
	fmt.Println("total repositories count:", totalRepositoriesCnt)

	return nil
}

func updateImagesCnt() error {
	//totalImagesCnt, _ = myMongo.GetImagesCountByText("")
	result := myutils.GlobalDBClient.Mongo.DockerHubDB.RunCommand(context.TODO(), bson.M{"collStats": myutils.GlobalConfig.MongoConfig.Collections.Images})
	stats := bson.M{}
	err := result.Decode(&stats)
	if err != nil {
		return err
	}
	totalImagesCnt = stats["count"].(int64)
	fmt.Println("total images count:", totalImagesCnt)

	return nil
}
