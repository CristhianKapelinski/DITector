package scripts

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"myutils"
	"testing"
)

// TestCalculateRepositoriesDependentWeights calculates
// dependent weights of all amd64 images under
// namespace/repository:tag and writes results to file
// /data/docker-crawler/results/dependent-weights.txt
func TestCalculateRepositoriesDependentWeights(t *testing.T) {
	myMongo, err := myutils.ConfigMongoClient(false)
	if err != nil {
		log.Fatalln(err)
	}
	myNeo4jDriver, err := myutils.NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc")
	if err != nil {
		log.Fatalln(err)
	}
	//resultFilePath := path.Join("/data/docker-crawler", "results/dependent-weights.txt")
	//resultFile, err := os.OpenFile(resultFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0744)
	//if err != nil {
	//	log.Fatalf("[ERROR] Open %s failed with: %s\n", resultFilePath, err)
	//} else {
	//	fmt.Println("[+] Open result file succeed: ", resultFilePath)
	//}
	cnt := 0

	// traverse all namespace/repository:tag to find amd64 image digest
	cursor, err := myMongo.RepositoriesCollection.Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatalln(err)
	}
	for cursor.Next(context.TODO()) {
		// separate page to query，try to fix 339200
		if cnt > 0 && cnt%10 == 0 {
			// close client of Mongo and Neo4j
			myMongo.Client.Disconnect(context.TODO())
			myNeo4jDriver.Driver.Close(context.TODO())

			myMongo, err = myutils.ConfigMongoClient(false)
			if err != nil {
				log.Fatalln(err)
			}
			myNeo4jDriver, err = myutils.NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc")
			if err != nil {
				log.Fatalln(err)
			}

			optSkip := options.Find().SetSkip(int64(cnt))
			cursor, err = myMongo.RepositoriesCollection.Find(context.TODO(), bson.D{}, optSkip)
			if err != nil {
				myutils.LogDockerCrawlerString(myutils.LogLevel.Error, "mongo find cursor failed with:", err.Error())
				log.Fatalln(err)
			}

			if cursor.Next(context.TODO()) {

			} else {
				myutils.LogDockerCrawlerString(myutils.LogLevel.Warn, "final document finish.")
				log.Fatalln("final document finish.")
			}
		}

		cnt++

		curRepo := new(myutils.RepositoryOld)
		err := cursor.Decode(curRepo)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(cnt, curRepo.Namespace, curRepo.Name)
		if cnt == 100 {
			return
		}
		//
		//for tagName, tagMeta := range curRepo.Tags {
		//	if arch, ok := tagMeta.Images["amd64"]; ok {
		//		for _, imageDigest := range arch {
		//			imageMeta, err := myMongo.FindImageByDigest(imageDigest)
		//			if err != nil {
		//				continue
		//			}
		//
		//			// calculate node id ( hash(1-2-5) ) for neo4j index
		//			accumulateLayerID := "" // 用于堆1、1-2、1-2-5，方便直接计算hash
		//			for _, layer := range imageMeta.Layers {
		//				if layer.Size == 0 {
		//					continue
		//				}
		//				accumulateLayerID += layer.Digest[7:]
		//			}
		//			accumulateHash := myutils.CalSha256(accumulateLayerID)
		//
		//			// calculate upstream and downstream images
		//			upImages, err := myNeo4jDriver.FindUpstreamImagesByNodeId(accumulateHash)
		//			if err != nil {
		//				upImages = []string{}
		//			}
		//			downImages, err := myNeo4jDriver.FindDownstreamImagesByNodeId(accumulateHash)
		//			if err != nil {
		//				downImages = []string{}
		//			}
		//
		//			// write results to result file
		//			upImagesStr, _ := json.Marshal(upImages)
		//			downImagesStr, _ := json.Marshal(downImages)
		//			resultFile.WriteString(curRepo.Namespace + "," + curRepo.Name + "," + tagName + "," + imageDigest +
		//				"," + strconv.Itoa(len(upImages)) + "," + strconv.Itoa(len(downImages)) + "," +
		//				string(upImagesStr) + "," + string(downImagesStr) + "\n")
		//		}
		//	}
		//}
	}
}

func TestCountTraverseRepositories(t *testing.T) {
	myMongo, err := myutils.ConfigMongoClient(false)
	if err != nil {
		log.Fatalln(err)
	}
	cnt := 0

	// traverse all namespace/repository:tag to find amd64 image digest
	cursor, err := myMongo.RepositoriesCollection.Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatalln(err)
	}
	for cursor.Next(context.TODO()) {
		cnt++
		if cnt%200 == 0 {
			fmt.Println(cnt)
		}
	}
}
