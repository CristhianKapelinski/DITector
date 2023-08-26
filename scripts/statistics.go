package scripts

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"myutils"
	"os"
	"path"
	"strconv"
)

// CalculateRepositoriesDependentWeights calculates
// dependent weights of all amd64 images under
// namespace/repository:tag and writes results to file
// /data/docker-crawler/results/dependent-weights.txt
func CalculateRepositoriesDependentWeights() {
	myMongo, err := myutils.ConfigMongoClient(false)
	if err != nil {
		log.Fatalln(err)
	}
	myNeo4jDriver, err := myutils.ConfigNewNeo4jDriverWithContext("neo4j://localhost:7687", "neo4j", "qazwsxedc")
	if err != nil {
		log.Fatalln(err)
	}
	resultFilePath := path.Join("/data/docker-crawler", "results/dependent-weights.txt")
	resultFile, err := os.OpenFile(resultFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0744)
	if err != nil {
		log.Fatalf("[ERROR] Open %s failed with: %s\n", resultFilePath, err)
	} else {
		fmt.Println("[+] Open result file succeed: ", resultFilePath)
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
		curRepo := new(myutils.Repository)
		err := cursor.Decode(curRepo)
		if err != nil {
			continue
		}

		for tagName, tagMeta := range curRepo.Tags {
			if arch, ok := tagMeta.Images["amd64"]; ok {
				for _, imageDigest := range arch {
					imageMeta, err := myMongo.FindImageByDigest(imageDigest)
					if err != nil {
						continue
					}

					// calculate node id ( hash(1-2-5) ) for neo4j index
					accumulateLayerID := "" // 用于堆1、1-2、1-2-5，方便直接计算hash
					for _, layer := range imageMeta.Layers {
						if layer.Size == 0 {
							continue
						}
						accumulateLayerID += layer.Digest[7:]
					}
					accumulateHash := myutils.CalSha256(accumulateLayerID)

					// calculate upstream and downstream images
					upImages, err := myNeo4jDriver.FindUpstreamImagesByNodeId(accumulateHash)
					if err != nil {
						upImages = []string{}
					}
					downImages, err := myNeo4jDriver.FindDownstreamImagesByNodeId(accumulateHash)
					if err != nil {
						downImages = []string{}
					}

					// write results to result file
					upImagesStr, _ := json.Marshal(upImages)
					downImagesStr, _ := json.Marshal(downImages)
					resultFile.WriteString(curRepo.Namespace + "," + curRepo.Name + "," + tagName + "," + imageDigest +
						"," + strconv.Itoa(len(upImages)) + "," + strconv.Itoa(len(downImages)) + "," +
						string(upImagesStr) + "," + string(downImagesStr) + "\n")
				}
			}
		}
	}
}
