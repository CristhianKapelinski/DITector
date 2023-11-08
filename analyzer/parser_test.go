package analyzer

import (
	"fmt"
	"github.com/Musso12138/dockercrawler/myutils"
	"log"
	"testing"
)

func TestPullSaveExtractImage(t *testing.T) {
	ci, err := NewCurrentImage("mongo:latest")
	if err != nil {
		log.Fatalln("create new current image got error:", err)
	}

	finish := make(chan downloadFinish)

	go ci.pullSaveExtractImage(myutils.GlobalConfig.TmpDir, finish)

	f := <-finish
	fmt.Println(f.imgTarPath)
	fmt.Println(f.imgDirPath)
	fmt.Println(f.err)

	fmt.Println(ci.manifest.Config)
	fmt.Println(ci.manifest.RepoTags)
	fmt.Println(ci.manifest.Layers)

	fmt.Println(ci.layerLocalFilepathList)
}

func TestParseFromFile(t *testing.T) {
	ci, err := NewCurrentImage("curlimages/curl:8.4.0")
	if err != nil {
		log.Fatalln("create new current image got error:", err)
	}
	if err = ci.ParseFromFile(); err != nil {
		log.Fatalln(err)
	}

	fmt.Println("get current image:", ci)

	return
}

func TestExtractRecommendCmd(t *testing.T) {
	for _, s := range extractRecommendCmd("```\n> docker pull curlimages/curl:8.4.0\n```\n\n### run docker image\nCheck everything works properly by running:\n```\n> docker run --rm curlimages/curl:8.4.0 --version\n```\nHere is a more specific example of running curl docker container: \n```\n> docker run --rm curlimages/curl:8.4.0 -L -v https://curl.haxx.se \n```\nTo work with files it is best to mount directory:\n```\n>  docker run --rm -it \\\n-v \"$PWD:/work\" \\\ncurlimages/curl:8.4.0 \\\n-d@/work/test.txt https://httpbin.org/post\n```") {
		fmt.Println(s)
	}
}
