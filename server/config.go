package server

import (
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
	"log"
	"sync"
)

var (
	myMongo       *myutils.MyMongoOld
	myNeo4jDriver *myutils.MyNeo4j

	// totalCnt is the number of documents in each collection,
	// used for calculate table pages
	totalRepositoriesCnt int64
	totalImagesCnt       int64
	configLock           = sync.WaitGroup{}
)

func configServer(initFlag bool) {
	var err error
	myMongo, err = myutils.ConfigMongoClient(initFlag)
	if err != nil {
		log.Fatalln("[ERROR] connect to and config MongoDB failed with err: ", err)
	}
	fmt.Println("[+] Connect to MongoDB succeed")

	myNeo4jDriver, err = myutils.NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)
	if err != nil {
		log.Fatalln("[ERROR] Connect to neo4j failed with:", err)
	}
	fmt.Println("[+] Connect to Neo4j succeed")

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

func updateRepositoriesCnt() {
	//totalRepositoriesCnt, _ = myMongo.GetRepositoriesCountByText("")
	totalRepositoriesCnt = 2576742
	fmt.Println(totalRepositoriesCnt)
}

func updateImagesCnt() {
	//totalImagesCnt, _ = myMongo.GetImagesCountByText("")
	totalImagesCnt = 7111908
	fmt.Println(totalImagesCnt)
}
