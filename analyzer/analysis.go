package analyzer

import (
	"fmt"
	"myutils"
)

var (
	rules         = Rules{}
	myMongo       *myutils.MyMongo
	myNeo4jDriver *myutils.MyNeo4j
)

func AnalyzeImage(digest string) (*myutils.ImageResult, error) {
	res := new(myutils.ImageResult)
	// configure image analyzer
	config(false, "../rules.yaml")

	targetImage, err := myMongo.FindImageByDigest(digest)
	if err != nil {
		return nil, err
	}
	results, err := AnalyzeImageMetadata(targetImage)
	if err != nil {
		return nil, err
	}
	res.Results = append(res.Results, results...)

	return res, nil
}

// AnalyzeImageMetadata analyze instruction of layers to
func AnalyzeImageMetadata(image *myutils.Image) ([]myutils.Result, error) {
	res := make([]myutils.Result, 0)

	for _, layer := range image.Layers {
		digest := ""
		if layer.Size != 0 {
			digest = layer.Digest
		}
		results, err := scanSecretsInString(digest, layer.Instruction)
		if err != nil {
			myutils.LogDockerCrawlerString(myutils.LogLevel.Error, "scan secrets in layer", digest, "failed with:", err.Error())
			continue
		}
		res = append(res, results...)
	}

	return res, nil
}

func scanSecretsInString(digest, s string) ([]myutils.Result, error) {
	res := make([]myutils.Result, 0)

	for _, secret := range rules.Secrets {
		fmt.Println(secret)
	}

	return res, nil
}
