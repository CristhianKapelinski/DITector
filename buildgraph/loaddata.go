package buildgraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"myutils"
	"os"
	"strconv"
	"time"
)

// 面向JSON数据源的接口

var (
	fileRepository *os.File
	fileTags       *os.File
	fileImages     *os.File
)

// ReadFileRepositoryByLine 用于逐行读取fileRepository，并将结果转换为Repository
func ReadFileRepositoryByLine() {
	fmt.Println("[INFO] Begin to read fileRepository")

	// 退出时结束占用的资源
	defer func() {
		fileRepository.Close()
		close(chanRepository)
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
		chanRepository <- repo

		if i%1000 == 0 {
			fmt.Println("File RepositoryName Line", i, ", Total Time:", time.Since(beginTime))
		}
	}
	fmt.Println("File RepositoryName Final Line, Total Time:", time.Since(beginTime))
	myutils.LogDockerCrawlerString(fmt.Sprintf("[INFO] Load File RepositoryName Finished, Total Time:%s", time.Since(beginTime)))
}

// ReadFileTagsByLine 用于逐行读取fileTags，并将结果转换为Tag
func ReadFileTagsByLine() {
	fmt.Println("[INFO] Begin to read fileTags")

	// 退出时结束占用的资源
	defer func() {
		fileTags.Close()
		close(chanTag)
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
			myutils.LogDockerCrawlerString("[ERROR] Fail to ReadLine in ReadFileTagsByLine: Line " + strconv.Itoa(i) + ", err: " + err.Error())
			break
		}

		// 解析内容，发到管道，等待scheduler调度
		var tag = new(myutils.TagSource)
		err = json.Unmarshal(b, tag)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with: ", err)
			continue
		}
		chanTag <- tag

		if i%1000 == 0 {
			fmt.Println("File Tags Line", i, ", Total Time:", time.Since(beginTime))
		}
	}
	fmt.Println("File Tags Final Line, Total Time:", time.Since(beginTime))
	myutils.LogDockerCrawlerString(fmt.Sprintf("[INFO] Load File Tags Finished, Total Time:%s", time.Since(beginTime)))
}

// ReadFileImagesByLine 用于逐行读取fileImages，并将结果转换为Image
func ReadFileImagesByLine() {
	fmt.Println("[INFO] Begin to read fileImages")

	// 退出时结束占用的资源
	defer func() {
		fileImages.Close()
		close(chanImage)
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
		chanImage <- image

		if i%1000 == 0 {
			fmt.Println("File Images Line", i, ", Total Time:", time.Since(beginTime))
		}
	}
	fmt.Println("File Images Final Line, Total Time:", time.Since(beginTime))
	myutils.LogDockerCrawlerString(fmt.Sprintf("[INFO] Load File Images Finished, Total Time:%s", time.Since(beginTime)))
}
