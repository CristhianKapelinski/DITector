package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Musso12138/docker-scan/myutils"
)

const JWTENV = "JWT_SECRET"

var (
	// totalCnt is the number of documents in each collection,
	// used for calculate table pages
	totalRepoCnt   int64
	totalTagCnt    int64
	totalImgCnt    int64
	totalResultCnt int64
	configLock     = sync.WaitGroup{}
)

func configServer() {
	configLock.Add(4)
	go func() {
		defer configLock.Done()
		if err := updateRepositoriesCnt(); err != nil {
			fmt.Println("failed to get totalRepoCnt, failed with:", err)
		}
	}()
	go func() {
		defer configLock.Done()
		if err := updateTagsCnt(); err != nil {
			fmt.Println("failed to get totalTagCnt, failed with:", err)
		}
	}()
	go func() {
		defer configLock.Done()
		if err := updateImagesCnt(); err != nil {
			fmt.Println("failed to get totalImgCnt, failed with:", err)
		}
	}()
	go func() {
		defer configLock.Done()
		if err := updateResultsCnt(); err != nil {
			fmt.Println("failed to get totalResultCnt, failed with:", err)
		}
	}()

	// 初始化服务器生成jwt的secret
	token := os.Getenv(JWTENV)
	if token == "" {
		secretRand := myutils.GetRandStr(64)
		secret := myutils.Sha256Str(secretRand)
		err := os.Setenv(JWTENV, secret)
		if err != nil {
			log.Fatalln("generate and save jwt secret failed with:", err)
		}
	}
	configLock.Wait()
}

func updateRepositoriesCnt() (err error) {
	//totalRepositoriesCnt, _ = myMongo.GetRepositoriesCountByText("")
	// result := myutils.GlobalDBClient.Mongo.DockerHubDB.RunCommand(context.TODO(), bson.M{"collStats": myutils.GlobalConfig.MongoConfig.Collections.Repositories})
	// stats := bson.M{}
	// err := result.Decode(&stats)
	// if err != nil {
	// 	return err
	// }
	// totalRepositoriesCnt = stats["count"].(int64)
	// fmt.Println("total repositories count:", totalRepositoriesCnt)

	begin := time.Now()
	totalRepoCnt, err = myutils.GlobalDBClient.Mongo.RepoColl.EstimatedDocumentCount(context.TODO())
	fmt.Println("total repository count:", totalRepoCnt, ", total time used:", time.Since(begin))
	return
}

func updateTagsCnt() (err error) {
	begin := time.Now()
	totalTagCnt, err = myutils.GlobalDBClient.Mongo.TagColl.EstimatedDocumentCount(context.TODO())
	fmt.Println("total tag count:", totalTagCnt, ", total time used:", time.Since(begin))
	return
}

func updateImagesCnt() (err error) {
	//totalImagesCnt, _ = myMongo.GetImagesCountByText("")
	// result := myutils.GlobalDBClient.Mongo.DockerHubDB.RunCommand(context.TODO(), bson.M{"collStats": myutils.GlobalConfig.MongoConfig.Collections.Images})
	// stats := bson.M{}
	// err := result.Decode(&stats)
	// if err != nil {
	// 	return err
	// }
	// totalImagesCnt = stats["count"].(int64)
	// fmt.Println("total images count:", totalImagesCnt)

	begin := time.Now()
	totalImgCnt, err = myutils.GlobalDBClient.Mongo.ImgColl.EstimatedDocumentCount(context.TODO())
	fmt.Println("total image count:", totalImgCnt, ", total time used:", time.Since(begin))
	return
}

func updateResultsCnt() (err error) {
	begin := time.Now()
	totalResultCnt, err = myutils.GlobalDBClient.Mongo.ImgResultColl.EstimatedDocumentCount(context.TODO())
	fmt.Println("total result count:", totalResultCnt, ", total time used:", time.Since(begin))
	return
}
