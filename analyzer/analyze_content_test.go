package analyzer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"testing"
)

func TestAsky(t *testing.T) {
	scaResFilepath := "/Users/musso/workshop/docker-projects/test/sca.json"
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	task, err := postCreateAskYTask(client, scaResFilepath)
	if err != nil {
		log.Fatalln("postCreateAskYTask got err:", err)
	}

	// 获取检测报告
	report, err := checkGetAskYReport(client, task)
	if err != nil {
		log.Fatalln("checkGetAskYReport got err:", err)
	}

	fmt.Println(report)
}
