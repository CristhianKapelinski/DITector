package scripts

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"myutils"
	"os"
	"path"
)

// Record is used for store a piece of JSON record
// of upstream and downstream information of an image.
// marshal and write to result file.
type Record struct {
	Namespace            string   `json:"namespace"`
	RepositoryName       string   `json:"repository_name"`
	TagName              string   `json:"tag_name"`
	ImageDigest          string   `json:"image_digest"`
	UpstreamImageCount   int      `json:"upstream_image_count"`
	UpstreamImageList    []string `json:"upstream_image_list"`
	DownstreamImageCount int      `json:"downstream_image_count"`
	DownstreamImageList  []string `json:"downstream_image_list"`
}

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
		myutils.LogDockerCrawlerString(myutils.LogLevel.Error, "mongo find cursor failed with:", err.Error())
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
			myNeo4jDriver, err = myutils.ConfigNewNeo4jDriverWithContext("neo4j://localhost:7687", "neo4j", "qazwsxedc")
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
		fmt.Println(cnt)
		//if cnt%10 == 0 {
		//	fmt.Println(cnt)
		//}
		curRepo := new(myutils.Repository)
		err := cursor.Decode(curRepo)
		if err != nil {
			myutils.LogDockerCrawlerString(myutils.LogLevel.Error, err.Error())
			continue
		}
		myutils.LogDockerCrawlerString(myutils.LogLevel.Info, "begin to calculate dependent weights of repository:",
			curRepo.Namespace, curRepo.Name)

		for tagName, tagMeta := range curRepo.Tags {
			if arch, ok := tagMeta.Images["amd64"]; ok {
				myutils.LogDockerCrawlerString(myutils.LogLevel.Debug, "find amd64 images in",
					curRepo.Namespace, curRepo.Name, tagName)

				for _, imageDigest := range arch {

					imageMeta, err := myMongo.FindImageByDigest(imageDigest)
					if err != nil {
						myutils.LogDockerCrawlerString(myutils.LogLevel.Error, err.Error())
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
						myutils.LogDockerCrawlerString(myutils.LogLevel.Error, err.Error())
						upImages = []string{}
					}
					downImages, err := myNeo4jDriver.FindDownstreamImagesByNodeId(accumulateHash)
					if err != nil {
						myutils.LogDockerCrawlerString(myutils.LogLevel.Error, err.Error())
						downImages = []string{}
					}

					// write results to result file
					//upImagesStr, err := json.Marshal(upImages)
					//if err != nil {
					//	myutils.LogDockerCrawlerString(myutils.LogLevel.Error, err.Error())
					//}
					//downImagesStr, err := json.Marshal(downImages)
					//if err != nil {
					//	myutils.LogDockerCrawlerString(myutils.LogLevel.Error, err.Error())
					//}

					record := Record{
						Namespace:            curRepo.Namespace,
						RepositoryName:       curRepo.Name,
						TagName:              tagName,
						ImageDigest:          imageDigest,
						UpstreamImageCount:   len(upImages),
						UpstreamImageList:    upImages,
						DownstreamImageCount: len(downImages),
						DownstreamImageList:  downImages,
					}

					recordBytes, err := json.Marshal(record)
					if err != nil {
						myutils.LogDockerCrawlerString(myutils.LogLevel.Error, err.Error())
					}
					resultFile.Write(recordBytes)
					resultFile.WriteString("\n")

					myutils.LogDockerCrawlerString(myutils.LogLevel.Debug, "finish calculate for image:",
						curRepo.Namespace, curRepo.Name, tagName, imageDigest)

					//resultFile.WriteString(curRepo.Namespace + "," + curRepo.Name + "," + tagName + "," + imageDigest +
					//	"," + strconv.Itoa(len(upImages)) + "," + strconv.Itoa(len(downImages)) + "," +
					//	string(upImagesStr) + "," + string(downImagesStr) + "\n")
				}
			}
		}
	}
}
