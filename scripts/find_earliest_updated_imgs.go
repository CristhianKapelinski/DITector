package scripts

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Musso12138/docker-scan/myutils"
)

var baseTimeStr = "2024-04-30T23:59:59.00000Z"
var oneYear = time.Hour * 24 * 365

type ImgSecretInfo struct {
	Namespace       string `json:"namespace"`
	RepositoryName  string `json:"repository_name"`
	TagName         string `json:"tag_name"`
	Digest          string `json:"digest"`
	FilteredSecrets struct {
		Metadata      []*myutils.SecretLeakage `json:"image-metadata"`
		Configuration []*myutils.SecretLeakage `json:"configuration"`
		Content       []*myutils.SecretLeakage `json:"content"`
	} `json:"filtered_secrets"`
}

type TargetImgSecretInfo struct {
	Namespace       string `json:"namespace"`
	RepositoryName  string `json:"repository_name"`
	TagName         string `json:"tag_name"`
	Digest          string `json:"digest"`
	FilteredSecrets struct {
		Content []*myutils.SecretLeakage `json:"content"`
	} `json:"filtered_secrets"`
}

// 根据layer digest在Neo4j中找到创建该layer的镜像名称
func findSrcImgOfLayer(digest string) (string, error) {
	srcImgNames, err := myutils.GlobalDBClient.Neo4j.FindSrcImgNamesByDigest(digest)
	if err != nil {
		fmt.Println("error neo4j find src image name by digest:", digest, "fail with:", err)
		return "", err
	}

	var earliestImgName string
	var earliestImgTime time.Time = time.Now()
	for _, imgName := range srcImgNames {
		_, _, _, _, imgDigest := myutils.DivideImageName(imgName)
		img, err := myutils.GlobalDBClient.Mongo.FindImageByDigest(imgDigest)
		if err != nil {
			continue
		}

		updateTime, err := time.Parse(time.RFC3339, img.LastPushed)
		if err != nil {
			continue
		}

		if updateTime.Before(earliestImgTime) {
			earliestImgName = imgName
			earliestImgTime = updateTime
		}
	}

	if earliestImgName == "" {
		return "", fmt.Errorf("not find src image for layer digest %s", digest)
	}

	return earliestImgName, nil
}

// t1在t2之前创建
func timeIsWithinOneYear(ts1, ts2 string) bool {
	if ts1 == "" || ts2 == "" {
		return false
	}

	t1, err1 := time.Parse(time.RFC3339, ts1)
	if err1 != nil {
		fmt.Printf("parse time %s fail with %s\n", ts1, err1)
		return false
	}

	t2, err2 := time.Parse(time.RFC3339, ts2)
	if err2 != nil {
		fmt.Printf("parse time %s fail with %s\n", ts2, err2)
		return false
	}

	duration := t2.Sub(t1)
	return duration < oneYear
}

// 镜像在thres前创建
func imgIsWithinTime(imgName string) bool {
	_, _, _, _, tmpImgDigest := myutils.DivideImageName(imgName)
	tmpImg, err := myutils.GlobalDBClient.Mongo.FindImageByDigest(tmpImgDigest)
	if err != nil {
		fmt.Printf("find image by digest %s fail with error: %s\n", tmpImgDigest, err)
		return false
	}

	createdTime := tmpImg.LastPushed
	return timeIsWithinOneYear(createdTime, baseTimeStr)
}

// 只能找content里面每个secret的source image，根据layer digest
func FindEarliestUpdatedImgs(inputPath, outputPath string) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	writer := bufio.NewWriter(outputFile)

	reader := bufio.NewReader(inputFile)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		var data ImgSecretInfo
		err = json.Unmarshal([]byte(line), &data)
		if err != nil {
			fmt.Println("error parsing JSON:", err)
			continue
		}

		fmt.Println("json got img secret digest:", data.Digest)

		for _, sec := range data.FilteredSecrets.Content {
			layerDigest := sec.LayerDigest
			srcImgName, err := findSrcImgOfLayer(layerDigest)
			if err != nil {
				fmt.Println("find source img of layer", layerDigest, "fail with error:", err)
				continue
			}
			myutils.Logger.Debug("find source img of layer", layerDigest, ", img name:", srcImgName)

			if imgIsWithinTime(srcImgName) {
				_, namespace, repo, tag, digest := myutils.DivideImageName(srcImgName)
				info := TargetImgSecretInfo{
					Namespace:      namespace,
					RepositoryName: repo,
					TagName:        tag,
					Digest:         digest,
					FilteredSecrets: struct {
						Content []*myutils.SecretLeakage `json:"content"`
					}{
						Content: []*myutils.SecretLeakage{sec},
					},
				}

				b, err := json.Marshal(info)
				if err != nil {
					fmt.Printf("json Marshal for img %s failed with: %s\n", srcImgName, err)
					continue
				}

				_, err = writer.Write(b)
				if err != nil {
					fmt.Printf("write to output file %s got error: %s\n", outputPath, err)
					continue
				}

				_ = writer.WriteByte('\n')
				writer.Flush()

				myutils.Logger.Info("process layer", layerDigest, "success")
				continue
			}
		}
	}

	return nil
}
