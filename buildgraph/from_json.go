package buildgraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"os"
	"runtime"
	"strconv"
	"time"
)

// 放在这防止package编译不通过
var (
	myMongo       *myutils.MyMongoOld
	myNeo4jDriver *myutils.MyNeo4j
)

var (
	// chanLimitMainGoroutine 限制goroutine数量
	chanLimitMainGoroutine chan struct{}
	fileRepository         *os.File
	fileTags               *os.File
	fileImages             *os.File
)

var (
	chanRepositoryOld  = make(chan *myutils.RepositoryOld, runtime.NumCPU())
	chanTagSource      = make(chan *myutils.TagSource, runtime.NumCPU())
	chanImageSource    = make(chan *myutils.ImageSource, runtime.NumCPU())
	chanDoneRepository = make(chan struct{})
	chanDoneTag        = make(chan struct{})
	chanDoneImage      = make(chan struct{})
)

// StartFromJSON 启动以JSON文件为数据源的信息库建设过程
func StartFromJSON() {
	go StoreRepositoryScheduler()
	ReadFileRepositoryByLine()
	<-chanDoneRepository

	go StoreTagScheduler()
	ReadFileTagsByLine()
	<-chanDoneTag

	go StoreImageScheduler()
	ReadFileImagesByLine()
	<-chanDoneImage
}

// ReadFileRepositoryByLine 用于逐行读取fileRepository，并将结果转换为Repository
func ReadFileRepositoryByLine() {
	fmt.Println("[INFO] Begin to read fileRepository")

	// 退出时结束占用的资源
	defer func() {
		fileRepository.Close()
		close(chanRepositoryOld)
	}()

	beginTime := time.Now()

	// 逐行读取文件内容直到EOF或其他错误
	scanner := bufio.NewReader(fileRepository)
	for i := 0; ; i++ {
		b, err := scanner.ReadBytes('\n')
		if err != nil {
			// 读到fileRepository结尾，退出
			if err == io.EOF {
				fmt.Println("[INFO] Read fileRepository done")
				break
			}
			fmt.Println("[ERROR] Fail to ReadLine in ReadFileRepositoryByLine: Line ", i, ", err: ", err)
			break
		}

		// 解析内容，发到管道，等待scheduler调度
		var repo = new(myutils.RepositoryOld)
		err = json.Unmarshal(b, repo)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with: ", err)
			continue
		}
		chanRepositoryOld <- repo

		if i%1000 == 0 {
			fmt.Println("File RepositoryName Line", i, ", Total Time:", time.Since(beginTime))
		}
	}
	fmt.Println("File RepositoryName Final Line, Total Time:", time.Since(beginTime))
	myutils.Logger.Info(fmt.Sprintf("Load File RepositoryName Finished, Total Time:%s", time.Since(beginTime)))
}

// ReadFileTagsByLine 用于逐行读取fileTags，并将结果转换为Tag
func ReadFileTagsByLine() {
	fmt.Println("[INFO] Begin to read fileTags")

	// 退出时结束占用的资源
	defer func() {
		fileTags.Close()
		close(chanTagSource)
	}()

	beginTime := time.Now()

	// 逐行读取文件内容直到EOF或其他错误
	scanner := bufio.NewReader(fileTags)
	for i := 0; ; i++ {
		b, err := scanner.ReadBytes('\n')
		if err != nil {
			// 读到fileTags结尾，退出
			if err == io.EOF {
				fmt.Println("[INFO] Read fileTags done")
				break
			}
			fmt.Println("[ERROR] Fail to ReadLine in ReadFileTagsByLine: Line ", i, ", err: ", err)
			myutils.Logger.Error("Fail to ReadLine in ReadFileTagsByLine: Line", strconv.Itoa(i), ", err:", err.Error())
			break
		}

		// 解析内容，发到管道，等待scheduler调度
		var tag = new(myutils.TagSource)
		err = json.Unmarshal(b, tag)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with: ", err)
			continue
		}
		chanTagSource <- tag

		if i%1000 == 0 {
			fmt.Println("File Tags Line", i, ", Total Time:", time.Since(beginTime))
		}
	}
	fmt.Println("File Tags Final Line, Total Time:", time.Since(beginTime))
	myutils.Logger.Info(fmt.Sprintf("Load File Tags Finished, Total Time:%s", time.Since(beginTime)))
}

// ReadFileImagesByLine 用于逐行读取fileImages，并将结果转换为Image
func ReadFileImagesByLine() {
	fmt.Println("[INFO] Begin to read fileImages")

	// 退出时结束占用的资源
	defer func() {
		fileImages.Close()
		close(chanImageSource)
	}()

	beginTime := time.Now()

	// 逐行读取文件内容直到EOF或其他错误
	scanner := bufio.NewReader(fileImages)
	for i := 0; ; i++ {
		b, err := scanner.ReadBytes('\n')
		if err != nil {
			// 读到文件结尾，退出
			if err == io.EOF {
				fmt.Println("[INFO] Read fileImages done")
				break
			}
			fmt.Println("[ERROR] Fail to ReadLine in ReadFileRepositoryByLine: Line ", i, ", err: ", err)
			break
		}

		// 解析内容，发到管道，等待scheduler调度
		var image = new(myutils.ImageSource)
		err = json.Unmarshal(b, image)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with: ", err)
			continue
		}
		chanImageSource <- image

		if i%1000 == 0 {
			fmt.Println("File Images Line", i, ", Total Time:", time.Since(beginTime))
		}
	}
	fmt.Println("File Images Final Line, Total Time:", time.Since(beginTime))
	myutils.Logger.Info(fmt.Sprintf("Load File Images Finished, Total Time:%s", time.Since(beginTime)))
}

func StoreRepositoryScheduler() {
	for repo := range chanRepositoryOld {
		err := myMongo.InsertRepository(repo)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				//fmt.Println("[WARN] Mongo Duplicate when inserting repository", repo.Namespace, repo.RepositoryName, ", repository already exists")
			} else {
				myutils.Logger.Error("Mongo insert repository", repo.Namespace+"/"+repo.Name, "failed with err:", err.Error())
			}
		}
	}
	chanDoneRepository <- struct{}{}
}

func StoreTagScheduler() {
	for tag := range chanTagSource {
		err := myMongo.InsertTag(tag)
		if err != nil {
			myutils.Logger.Error("Mongo insert tag", tag.Namespace+"/"+tag.RepositoryName+"/"+tag.Name, "failed with err:", err.Error())
		}
	}
	chanDoneTag <- struct{}{}
}

func StoreImageScheduler() {
	for image := range chanImageSource {
		err := myMongo.InsertImage(image)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				//fmt.Println("[WARN] Mongo Duplicate when inserting repository", repo.Namespace, repo.RepositoryName, ", repository already exists")
			} else {
				myutils.Logger.Error("Mongo insert image", image.Image.Digest, "failed with err:"+err.Error())
			}
		}
		myNeo4jDriver.InsertImageToNeo4j(image)
	}
	chanDoneImage <- struct{}{}
}
