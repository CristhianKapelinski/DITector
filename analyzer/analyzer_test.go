package analyzer

import (
	"fmt"
	"myutils"
	"testing"
)

func TestAnalyzeImageMetadata(t *testing.T) {
	mymongo, _ := myutils.NewMongoClient(false)
	imageAnalyzer, _ := NewImageAnalyzer("../rules/secret_rules.yaml")

	targetImages, _ := mymongo.FindImagesByText("", 1, 10)
	targetImages = append(targetImages, &myutils.ImageOld{
		Layers: []myutils.LayerSource{
			myutils.LayerSource{},
			myutils.LayerSource{Digest: "123456", Instruction: "-----BEGIN RSA PRIVATE KEYsk_test_000011112222333344445555", Size: 10},
		},
	})
	for _, targetImage := range targetImages {
		results, _ := imageAnalyzer.AnalyzeImageMetadata(targetImage)
		for _, result := range results {
			fmt.Println(result)
		}
	}
}

func TestScanSecretsInString(t *testing.T) {
	imageAnalyzer := new(ImageAnalyzer)
	imageAnalyzer.loadRules(false, "../rules/secret_rules.yaml")
	imageAnalyzer.rules.CompileSecretsRegex()

	secrets, _ := imageAnalyzer.scanSecretsInString("-----BEGIN RSA PRIVATE KEYsk_test_000011112222333344445555", "contents")
	for _, secret := range secrets {
		fmt.Println(secret)
	}
}
