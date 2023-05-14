package buildgraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
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

	// 逐行读取文件内容直到EOF或其他错误
	scanner := bufio.NewReader(fileRepository)
	for i := 0; i < 10; i++ {
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
		var repo = new(Repository)
		err = json.Unmarshal(b, repo)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with: ", err)
			continue
		}
		chanRepository <- repo
	}
}

// ReadFileTagsByLine 用于逐行读取fileTags，并将结果转换为Tag
func ReadFileTagsByLine() {
	fmt.Println("[INFO] Begin to read fileTags")

	// 退出时结束占用的资源
	defer func() {
		fileTags.Close()
		close(chanTag)
	}()

	// 逐行读取文件内容直到EOF或其他错误
	scanner := bufio.NewReader(fileTags)
	for i := 0; i < 10; i++ {
		b, err := scanner.ReadBytes('\n')
		if err != nil {
			// 读到fileTags结尾，退出
			if err == io.EOF {
				fmt.Println("[INFO] Read fileTags done")
				break
			}
			fmt.Println("[ERROR] Fail to ReadLine in ReadFileTagsByLine: Line ", i, ", err: ", err)
			break
		}

		// 解析内容，发到管道，等待scheduler调度
		var tag = new(TagSource)
		err = json.Unmarshal(b, tag)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with: ", err)
			continue
		}
		chanTag <- tag
	}
}

// ReadFileImagesByLine 用于逐行读取fileImages，并将结果转换为Image
func ReadFileImagesByLine() {
	fmt.Println("[INFO] Begin to read fileImages")

	// 退出时结束占用的资源
	defer func() {
		fileImages.Close()
		close(chanImage)
	}()

	// 逐行读取文件内容直到EOF或其他错误
	scanner := bufio.NewReader(fileImages)
	for i := 0; i < 10; i++ {
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
		var image = new(ImageSource)
		err = json.Unmarshal(b, image)
		if err != nil {
			fmt.Println("[ERROR] json.Unmarshal failed with: ", err)
			continue
		}
		chanImage <- image
	}
}
