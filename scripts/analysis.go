package scripts

import (
	"analyzer"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"myutils"
	"os"
	"strconv"
	"strings"
)

var selflogger, _ = os.OpenFile("/data/docker-crawler/results/secrets-in-image-metadata.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0744)

func logself(s ...string) {
	tmp := strings.Join(s, " ")
	selflogger.WriteString(myutils.GetLocalNowTime() + " " + tmp + "\n")
}

// ScanAllSecretsInImageMetadata scan all secrets in metadata
// images, and write results to mongo.dockerhub.results.
// log to file
// /data/docker-crawler/results/secrets-in-image-metadata.log
func ScanAllSecretsInImageMetadata() {
	mymongo, _ := myutils.ConfigMongoClient(false)
	imageAnalyzer, err := analyzer.NewImageAnalyzer("rules.yaml")
	if err != nil {
		logself(myutils.LogLevel.Error, "load yaml rules failed with:", err.Error())
		log.Fatalln(err)
	}

	cursor, err := mymongo.ImagesCollection.Find(context.TODO(), bson.D{})
	if err != nil {
		logself(myutils.LogLevel.Error, "traverse images failed with:", err.Error())
		log.Fatalln(err)
	}
	defer cursor.Close(context.TODO())
	cnt := 0

	for cursor.Next(context.TODO()) {
		cnt++
		logself(myutils.LogLevel.Debug, "begin to scan", strconv.Itoa(cnt))

		targetImage := new(myutils.Image)
		err := cursor.Decode(targetImage)
		if err != nil {
			logself(myutils.LogLevel.Error, "decode image failed with:", err.Error())
			continue
		}

		imgres := new(myutils.ImageResult)
		imgres.Digest = targetImage.Digest
		imgres.LastAnalyzed = myutils.GetLocalNowTime()

		imgres.Results, err = imageAnalyzer.AnalyzeImageMetadata(targetImage)
		if err != nil {
			logself(myutils.LogLevel.Error, "analyze metadata of image", imgres.Digest, "failed with:", err.Error())
			continue
		}

		err = mymongo.InsertResult(imgres)
		if err != nil {
			logself(myutils.LogLevel.Error, "insert image result failed with:", err.Error())
			continue
		}
	}
}
