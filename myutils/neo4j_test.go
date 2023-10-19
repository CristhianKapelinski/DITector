package myutils

import (
	"fmt"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"log"
	"testing"
)

// TestFindMiddleSharedLayers 尝试发现从中间层开始交叉的两条image链，并将其打印出来

func TestFindUpstreamNodesByNodeId(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	tmpImage, err := mymongo.FindImageByDigest("sha256:7209d3b2285c9ca5a28051a5d8658e64e40888154d753bbd8a22eee214132a81")
	if err != nil {
		log.Fatalln(err)
	}

	accumulateLayerID := "" // 用于堆1、1-2、1-2-5，方便直接计算hash
	cnt := 0

	for _, layer := range tmpImage.Layers {
		if layer.Size == 0 {
			continue
		}
		accumulateLayerID += layer.Digest[7:]
		cnt++
	}

	accumulateHash := CalSha256(accumulateLayerID)
	fmt.Println(accumulateHash)

	neo4jDriver, err := NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)
	if err != nil {
		log.Fatalln(err)
	}
	upNodes, err := neo4jDriver.FindUpstreamLayerNodesByNodeId(accumulateHash)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(cnt)

	for _, upNode := range upNodes.([]*neo4j.Record) {
		prop := GetNodeProps(upNode)
		fmt.Println(prop)
	}

}

func TestFindDownstreamNodesByNodeId(t *testing.T) {
	neo4jDriver, err := NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)
	if err != nil {
		log.Fatalln(err)
	}
	// library/debian:buster-20210621
	upNodes, err := neo4jDriver.FindDownstreamLayerNodesByNodeId("5fa6942eb5292e363c9c3c4e7546fb8e4f78f7606fdd1ecbabe19dc2e1298c66")
	if err != nil {
		log.Fatalln(err)
	}

	for _, upNode := range upNodes.([]*neo4j.Record) {
		prop := GetNodeProps(upNode)
		fmt.Println(prop)
	}

}

func TestMyNeo4j_FindUpstreamImagesByNodeId(t *testing.T) {
	neo4jDriver, err := NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)
	if err != nil {
		log.Fatalln(err)
	}
	upImages, err := neo4jDriver.FindUpstreamImagesByNodeId("8248ea9ed48c3c1f8acc38ae91f46f759cc4d795561dd8e89e405236b9e913f4")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(upImages)
}

func TestMyNeo4j_FindDownstreamImagesByNodeId(t *testing.T) {
	neo4jDriver, err := NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)
	if err != nil {
		log.Fatalln(err)
	}
	// library/debian:buster-20210621
	downImages, err := neo4jDriver.FindDownstreamImagesByNodeId("5fa6942eb5292e363c9c3c4e7546fb8e4f78f7606fdd1ecbabe19dc2e1298c66")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(downImages)
}
