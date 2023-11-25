package scripts

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

// ScanAllSecretsInImageMetadata scan all secrets in metadata
// images, and write results to mongo.dockerhub.results.
// log to file
// /data/docker-crawler/results/secrets-in-image-metadata.log
func ScanAllSecretsInImageMetadata() {
	//cursor, err := myutils.GlobalDBClient.Mongo.ImgColl.Find(context.TODO(), bson.D{})
	//if err != nil {
	//	logself(myutils.LogLevelStr.Error, "traverse images failed with:", err.Error())
	//	log.Fatalln(err)
	//}
	//defer cursor.Close(context.TODO())
	//cnt := 0
	//
	//for cursor.Next(context.TODO()) {
	//	cnt++
	//	logself(myutils.LogLevelStr.Debug, "begin to scan", strconv.Itoa(cnt))
	//
	//	targetImage := new(myutils.Image)
	//	err := cursor.Decode(targetImage)
	//	if err != nil {
	//		logself(myutils.LogLevelStr.Error, "decode image failed with:", err.Error())
	//		continue
	//	}
	//
	//	imgres := new(myutils.ImageResult)
	//	imgres.Digest = targetImage.Digest
	//	imgres.LastAnalyzed = myutils.GetLocalNowTime()
	//
	//	imgres, err = analyzer.AnalyzeImagePartialByName(targetImage)
	//	if err != nil {
	//		logself(myutils.LogLevelStr.Error, "analyze metadata of image", imgres.Digest, "failed with:", err.Error())
	//		continue
	//	}
	//
	//	err = myutils.GlobalDBClient.Mongo.UpdateImgResult(imgres)
	//	if err != nil {
	//		logself(myutils.LogLevelStr.Error, "insert image result failed with:", err.Error())
	//		continue
	//	}
	//}
}

// ScanTop100DownstreamImagesVul scan vulnerabilities of top 100
// downstream images according to file dependent-weights-top100.txt
// with SCA tools, now only supports using
// anchore/grype: https://github.com/anchore/grype
func ScanTop100DownstreamImagesVul() {
	resultFilePath := "/data/docker-crawler/results/dependent-weights/dependent-weights-downstream-top100.txt"
	resultFile, _ := os.Open(resultFilePath)
	defer func() {
		resultFile.Close()
		fmt.Println("[INFO] read done:", resultFilePath)
	}()

	scanner := bufio.NewReader(resultFile)
	for i := 0; ; i++ {
		b, err := scanner.ReadBytes('\n')
		if err != nil {
			// 读到fileRepository结尾，退出
			if err == io.EOF {
				break
			}
			fmt.Println("[ERROR] Fail to ReadLine in", resultFilePath, ": Line", i, ", err:", err)
			break
		}

		var record = new(RecordWithNodeID)
		err = json.Unmarshal(b, record)
		if err != nil {
			myutils.Logger.Error("json unmarshal failed with:", err.Error())
			continue
		}

		realTagName := strings.ReplaceAll(record.TagName, "$", ".")
		fmt.Println(record.Namespace, record.RepositoryName, realTagName, record.ImageDigest)

		imageFullName := record.Namespace + "/" + record.RepositoryName + ":" + realTagName
		resultPath := path.Join("/data/docker-crawler/results/sca-results/anchore-downstream-top100/",
			record.Namespace+"-"+record.RepositoryName+"-"+realTagName+".json")
		beginTime := time.Now()

		cmd := exec.Command("/home/hequan/anchore/grype/grype", imageFullName, "-o", "json", "--file", resultPath)
		if err = cmd.Run(); err != nil {
			myutils.Logger.Error("run shell command failed with:", err.Error())
			fmt.Println("[ERROR] run shell command failed with:", err)
		}

		fmt.Println("Finish:", i, ", Total Time:", time.Since(beginTime))
	}
}
