package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"io"
	"log"
	"testing"
)

func TestImagePull(t *testing.T) {
	ci, err := NewCurrentImage()
	if err != nil {
		log.Fatalln("create new current image got error:", err)
	}

	irc, err := ci.dockerClient.ImagePull(context.TODO(), "alpine:3", types.ImagePullOptions{})
	if err != nil {
		log.Fatalln("pull image got error:", err)
	}
	defer irc.Close()

	b, _ := io.ReadAll(irc)
	fmt.Println(string(b))
}

func TestDefer(t *testing.T) {
	var a = make(map[string]interface{})
	var err error
	defer func(err error) {
		fmt.Println("defer got error:", err)
	}(err)

	err = json.Unmarshal([]byte(`{"aaa": ["abc"}`), &a)
	fmt.Println("got err:", err)

	return
}

func TestParse(t *testing.T) {
	ci, err := NewCurrentImage()
	if err != nil {
		log.Fatalln("create new current image got error:", err)
	}
	ci.ParseFromDockerEnv()

	// 查看系统平台
	fmt.Println(ci.architecture, ci.os)

	// 查看元数据信息
	fmt.Println(ci.metadata.repositoryMetadata)

	// 查看配置信息
	fmt.Println(ci.configuration.RepoTags, ci.configuration.Architecture, ci.configuration.Variant)

	// 查看内容信息
	fmt.Println(ci.layerInfoMap[ci.layerWithContentList[0]])
}

func TestParseMetadata(t *testing.T) {
	ci, err := NewCurrentImage()
	if err != nil {
		log.Fatalln("create new current image got error:", err)
	}
	ci.parseName()
	ci.parseServerPlatform()

	if err := ci.parseMetadata(true); err != nil {
		log.Fatalln("parse metadata failed with:", err)
	}

	fmt.Println(ci.architecture, ci.os)

	fmt.Println(ci.metadata.repositoryMetadata.Namespace, ci.metadata.repositoryMetadata.Name)

	fmt.Println(ci.metadata.tagMetadata.Name, ci.metadata.tagMetadata.LastUpdated)

	fmt.Println(ci.metadata.imageMetadata.Digest)
}
