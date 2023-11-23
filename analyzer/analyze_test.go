package analyzer

import (
	"fmt"
	"log"
	"testing"
)

func TestNewImageAnalyzerGlobalConfig(t *testing.T) {

}

func TestAnalyzeImageMetadata(t *testing.T) {
	fmt.Println(AnalyzeImagePartialByName("benjamineugenewhite/safegraph-sieve-2:early"))
}

func TestAnalyzeImagePartialByName(t *testing.T) {
	// 隐私信息泄露
	//res, err := AnalyzeImagePartialByName("benjamineugenewhite/safegraph-sieve-2:early")
	// 敏感参数
	res, err := AnalyzeImagePartialByName("phenompeople/mongodb:latest")
	if err != nil {
		log.Fatalln("AnalyzeImagePartialByName", res.Name, "failed with:", err)
	}

	return
}

func TestScanSecretsInString(t *testing.T) {
	if imageAnalyzerE != nil {
		log.Fatalln(imageAnalyzerE)
	}

	secrets := ("-----BEGIN RSA PRIVATE KEYsk_test_000011112222333344445555")
	for _, secret := range secrets {
		fmt.Println(secret)
	}
}

func TestScanSensitiveParamInString(t *testing.T) {
	if imageAnalyzerE != nil {
		log.Fatalln(imageAnalyzerE)
	}

	secrets := imageAnalyzer.scanSensitiveParamInString("")
	for _, secret := range secrets {
		fmt.Println(secret)
	}
}

func TestScanFileMalicious(t *testing.T) {
	//fmt.Println(scanFileMalicious("/Users/musso/workshop/docker-projects/docker-image-supply-chain/malware/xmrig-6.20.0-linux-static-x64.tar.gz"))
	fmt.Println(scanFileMalicious("/Users/musso/workshop/docker-projects/docker-image-supply-chain/download-images/thanhcongnhe-thanhcongnhe-latest/eed3f579a2a05c9097747ad49de87b3149de099dc2ddcd0e39df9b59908bac84/layer/etc/dao.py"))
}
