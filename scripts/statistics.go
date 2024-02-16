package scripts

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/Musso12138/docker-scan/myutils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	myNeo4jDriver, err := myutils.NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)
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
		myutils.Logger.Error("mongo find cursor failed with:", err.Error())
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
			myNeo4jDriver, err = myutils.NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)
			if err != nil {
				log.Fatalln(err)
			}

			optSkip := options.Find().SetSkip(int64(cnt))
			cursor, err = myMongo.RepositoriesCollection.Find(context.TODO(), bson.D{}, optSkip)
			if err != nil {
				myutils.Logger.Error("mongo find cursor failed with:", err.Error())
				log.Fatalln(err)
			}

			if cursor.Next(context.TODO()) {

			} else {
				myutils.Logger.Warn("final document finish.")
				log.Fatalln("final document finish.")
			}
		}

		cnt++
		fmt.Println(cnt)
		//if cnt%10 == 0 {
		//	fmt.Println(cnt)
		//}
		curRepo := new(myutils.RepositoryOld)
		err := cursor.Decode(curRepo)
		if err != nil {
			myutils.Logger.Error(err.Error())
			continue
		}
		myutils.Logger.Info("begin to calculate dependent weights of repository:", curRepo.Namespace, curRepo.Name)

		for tagName, tagMeta := range curRepo.Tags {
			if arch, ok := tagMeta.Images["amd64"]; ok {
				myutils.Logger.Debug("find amd64 images in", curRepo.Namespace, curRepo.Name, tagName)

				for _, imageDigest := range arch {

					imageMeta, err := myMongo.FindImageByDigest(imageDigest)
					if err != nil {
						myutils.Logger.Error(err.Error())
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
					accumulateHash := myutils.Sha256Str(accumulateLayerID)

					// calculate upstream and downstream images
					upImages, err := myNeo4jDriver.FindUpstreamImagesByNodeId(accumulateHash)
					if err != nil {
						myutils.Logger.Error(err.Error())
						upImages = []string{}
					}
					downImages, err := myNeo4jDriver.FindDownstreamImagesByNodeId(accumulateHash)
					if err != nil {
						myutils.Logger.Error(err.Error())
						downImages = []string{}
					}

					// write results to result file
					//upImagesStr, err := json.Marshal(upImages)
					//if err != nil {
					//	myutils.Logger.Error(err.Error())
					//}
					//downImagesStr, err := json.Marshal(downImages)
					//if err != nil {
					//	myutils.Logger.Error(err.Error())
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
						myutils.Logger.Error(err.Error())
					}
					resultFile.Write(recordBytes)
					resultFile.WriteString("\n")

					myutils.Logger.Debug("finish calculate for image:", curRepo.Namespace, curRepo.Name, tagName, imageDigest)

					//resultFile.WriteString(curRepo.Namespace + "," + curRepo.Name + "," + tagName + "," + imageDigest +
					//	"," + strconv.Itoa(len(upImages)) + "," + strconv.Itoa(len(downImages)) + "," +
					//	string(upImagesStr) + "," + string(downImagesStr) + "\n")
				}
			}
		}
	}
}

// =======================================================================
// calculate total, average and top 100 records according to upstream and downstream counts
// =======================================================================

var chanRecord = make(chan *RecordWithNodeID, runtime.NumCPU())

// RecordUpSlice used to store top 100 record according to upstream
type RecordUpSlice []*RecordWithNodeID

func (rs RecordUpSlice) Len() int { return len(rs) }
func (rs RecordUpSlice) Less(i, j int) bool {
	return rs[i].UpstreamImageCount > rs[j].UpstreamImageCount
}
func (rs RecordUpSlice) Swap(i, j int) { rs[i], rs[j] = rs[j], rs[i] }

// CheckNodeId checks whether an image with digest is unique in RecordSlice
func (rs RecordUpSlice) CheckNodeId(nodeId string) bool {
	for _, record := range rs {
		if nodeId == record.NodeId {
			return true
		}
	}
	return false
}

// RecordDownSlice used to store top 100 record according to downstream
type RecordDownSlice []*RecordWithNodeID

func (rs RecordDownSlice) Len() int { return len(rs) }
func (rs RecordDownSlice) Less(i, j int) bool {
	return rs[i].DownstreamImageCount > rs[j].DownstreamImageCount
}
func (rs RecordDownSlice) Swap(i, j int) { rs[i], rs[j] = rs[j], rs[i] }

// CheckNodeId checks whether an image with digest is unique in RecordSlice
func (rs RecordDownSlice) CheckNodeId(nodeId string) bool {
	for _, record := range rs {
		if nodeId == record.NodeId {
			return true
		}
	}
	return false
}

type RecordWithNodeID struct {
	Namespace            string `json:"namespace"`
	RepositoryName       string `json:"repository_name"`
	TagName              string `json:"tag_name"`
	ImageDigest          string `json:"image_digest"`
	NodeId               string
	UpstreamImageCount   int      `json:"upstream_image_count"`
	UpstreamImageList    []string `json:"upstream_image_list"`
	DownstreamImageCount int      `json:"downstream_image_count"`
	DownstreamImageList  []string `json:"downstream_image_list"`
}

// StatisticRepositoriesDependentWeights calculates
// dependent weight statistics of each repository
// by read and process file /data/docker-crawler/results/dependent-weights/dependent-weights.txt
func StatisticRepositoriesDependentWeights() {
	mymongo, _ := myutils.ConfigMongoClient(false)

	upstreamFile, _ := os.OpenFile("/data/docker-crawler/results/dependent-weights/dependent-weights-upstream-top100.txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0744)
	downstreamFile, _ := os.OpenFile("/data/docker-crawler/results/dependent-weights/dependent-weights-downstream-top100.txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0744)

	go readDependentWeightFileByLine()

	var total int64 = 0
	var totalUp int64 = 0
	var top100Up = make(RecordUpSlice, 100, 100)
	// assign initial value for each element
	for i := range top100Up {
		top100Up[i] = &RecordWithNodeID{}
	}
	var totalDown int64 = 0
	var top100Down = make(RecordDownSlice, 100, 100)
	for i := range top100Down {
		top100Down[i] = &RecordWithNodeID{}
	}

	fmt.Println(top100Up[0].UpstreamImageCount)

	for record := range chanRecord {
		total++
		totalUp += int64(record.UpstreamImageCount)
		totalDown += int64(record.DownstreamImageCount)

		image, err := mymongo.FindImageByDigest(record.ImageDigest)
		if err != nil {
			myutils.Logger.Error("mongo find image by digest failed with:", err.Error())
			continue
		}
		record.NodeId = myutils.CalculateImageNodeIdOld(image)

		// recalculate top 100 up
		if !top100Up.CheckNodeId(record.NodeId) {
			top100Up = append(top100Up, record)
			sort.Sort(top100Up)
			top100Up = top100Up[:100]
		}

		if !top100Down.CheckNodeId(record.NodeId) {
			top100Down = append(top100Down, record)
			sort.Sort(top100Down)
			top100Down = top100Down[:100]
		}

		if total%1000 == 0 {
			fmt.Println("process record:", total)
			fmt.Println("top 1 up:", top100Up[0].UpstreamImageCount, ", top 100 up:", top100Up[99].UpstreamImageCount)
			fmt.Println("top 1 down:", top100Down[0].DownstreamImageCount, ", top 100 down:", top100Down[99].DownstreamImageCount)
			myutils.Logger.Info(fmt.Sprintf("processing record: %d, top 1 up: %d, top 100 up: %d, top 1 down: %d, top 100 down: %d\n",
				total, top100Up[0].UpstreamImageCount, top100Up[99].UpstreamImageCount,
				top100Down[0].DownstreamImageCount, top100Down[99].DownstreamImageCount,
			),
			)
		}
	}

	averageUp := totalUp / total
	averageDown := totalDown / total

	fmt.Printf("max upstream cnt: %d, average upstream cnt: %d\n", top100Up[0].UpstreamImageCount, averageUp)
	fmt.Printf("max downstream cnt: %d, average downstream cnt: %d\n", top100Down[0].DownstreamImageCount, averageDown)

	for _, upRecord := range top100Up {
		recordBytes, err := json.Marshal(upRecord)
		if err != nil {
			myutils.Logger.Error(err.Error())
		}
		upstreamFile.Write(recordBytes)
		upstreamFile.WriteString("\n")
	}

	for _, downRecord := range top100Down {
		recordBytes, err := json.Marshal(downRecord)
		if err != nil {
			myutils.Logger.Error(err.Error())
		}
		downstreamFile.Write(recordBytes)
		downstreamFile.WriteString("\n")
	}

}

// readDependentWeightFileByLine read the file storing
// the results of dependent weights of each repository
// by line.
func readDependentWeightFileByLine() {
	fileDependentWeights, err := os.Open("/data/docker-crawler/results/dependent-weights/dependent-weights.txt")
	if err != nil {
		log.Fatalln(err)
	}
	// 退出时结束占用的资源
	defer func() {
		fileDependentWeights.Close()
		fmt.Println("[INFO] read done: /data/docker-crawler/results/dependent-weights.txt")
		close(chanRecord)
	}()

	beginTime := time.Now()

	// 逐行读取文件内容直到EOF或其他错误
	scanner := bufio.NewReader(fileDependentWeights)
	for i := 0; ; i++ {
		b, err := scanner.ReadBytes('\n')
		if err != nil {
			// 读到fileRepository结尾，退出
			if err == io.EOF {
				break
			}
			fmt.Println("[ERROR] Fail to ReadLine in /data/docker-crawler/results/dependent-weights.txt: Line", i, ", err:", err)
			myutils.Logger.Error("Fail to ReadLine in /data/docker-crawler/results/dependent-weights.txt: Line",
				strconv.Itoa(i), "err:", err.Error())
			break
		}

		// 解析内容，发到管道，等待scheduler调度
		var record = new(RecordWithNodeID)
		err = json.Unmarshal(b, record)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with:", err)
			myutils.Logger.Error("json.Unmarshal failed with:", err.Error())
			continue
		}
		chanRecord <- record

		if i%1000 == 0 {
			fmt.Println("Line", i, ", Total Time:", time.Since(beginTime))
		}
	}
	fmt.Println("File /data/docker-crawler/results/dependent-weights.txt Final Line, Total Time:", time.Since(beginTime))
	myutils.Logger.Info("load file /data/docker-crawler/results/dependent-weights.txt finished, total time:",
		time.Since(beginTime).String(),
	)
}
