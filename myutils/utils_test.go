package myutils

import (
	"fmt"
	"log"
	"path/filepath"
	"testing"
	"time"
)

func TestDivideImageName(t *testing.T) {
	fmt.Println(DivideImageName("hello-world"))
	fmt.Println(DivideImageName("minio/minio"))
	fmt.Println(DivideImageName("docker.io/library/mongo"))
	fmt.Println(DivideImageName("hello-world@sha256:88ec0acaa3ec199d3b7eaf73588f4518c25f9d34f58ce9a0df68429c5af48e8d"))
	fmt.Println(DivideImageName("library/hello-world@sha256:88ec0acaa3ec199d3b7eaf73588f4518c25f9d34f58ce9a0df68429c5af48e8d"))
	fmt.Println(DivideImageName("hello-world:latest@sha256:88ec0acaa3ec199d3b7eaf73588f4518c25f9d34f58ce9a0df68429c5af48e8d"))
	fmt.Println(DivideImageName("library/hello-world:latest@sha256:88ec0acaa3ec199d3b7eaf73588f4518c25f9d34f58ce9a0df68429c5af48e8d"))
	fmt.Println(DivideImageName("docker.io/library/hello-world:latest@sha256:88ec0acaa3ec199d3b7eaf73588f4518c25f9d34f58ce9a0df68429c5af48e8d"))
}

func TestSha256File(t *testing.T) {
	begin := time.Now()
	h, e := Sha256File("/data/tmp/docker-proj/library-mongo-latest.tar")
	if e != nil {
		log.Fatalln("got error:", e)
	}
	fmt.Println(h, time.Since(begin).String())
}

func TestRelPath(t *testing.T) {
	fmt.Println(filepath.Rel("/aaa/bbb/ccc/layer/", "/aaa/bbb/ccc/layer/etc/library"))
}
