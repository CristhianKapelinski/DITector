package analyzer

import (
	"myutils"
)

var myMongo, _ = myutils.ConfigMongoClient(false)
var myNeo4jDriver, _ = myutils.ConfigNewNeo4jDriverWithContext("neo4j://localhost:7687", "neo4j", "qazwsxedc")

func AnalyzeImage(name, digest, secretRuleFilePath string) (*myutils.ImageResult, error) {
	res := new(myutils.ImageResult)

	analyzer, err := NewImageAnalyzer(secretRuleFilePath)
	if err != nil {
		myutils.LogDockerCrawlerString(myutils.LogLevel.Error, "create image analyzer failed with:", err.Error())
		return nil, err
	}

	targetImage, err := myMongo.FindImageByDigest(digest)
	if err != nil {
		return nil, err
	}

	// analyze metadata of the image
	results, err := AnalyzeImageMetadata(targetImage)
	if err != nil {
		return nil, err
	}
	res.Results = append(res.Results, results...)

	// TODO: analyze contents of each layer of the image

	return res, nil
}
