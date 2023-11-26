package scripts

import (
	"bufio"
	"fmt"
	"github.com/Musso12138/docker-scan/analyzer"
	"github.com/Musso12138/docker-scan/myutils"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// BatchAnalyzeByName 批量并发检测镜像
func BatchAnalyzeByName(input string, partial bool) error {
	// 打开输入文件
	file, err := os.Open(input)
	if err != nil {
		return err
	}

	// 配置线程数
	maxThreads := runtime.NumCPU()
	if myutils.GlobalConfig.MaxThread > 0 && myutils.GlobalConfig.MaxThread < maxThreads {
		maxThreads = myutils.GlobalConfig.MaxThread
		runtime.GOMAXPROCS(maxThreads)
	}

	// 初始化控制并发线程数的管道
	imgNameCh := make(chan string)
	wg := sync.WaitGroup{}

	for w := 1; w <= maxThreads; w++ {
		wg.Add(1)
		go batchAnalyzeByNameWorker(w, imgNameCh, partial, &wg)
	}

	// 从input文件中读取每行字符串作为待检测镜像名
	scanner := bufio.NewReader(file)
	for i := 1; ; i++ {
		line, err := scanner.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				myutils.Logger.Info("BatchAnalyzeByName finished reading file", input)
				break
			}
			myutils.Logger.Error("BatchAnalyzeByName read file:", input, ", line: Line", strconv.Itoa(i), ", failed with:", err.Error())
			continue
		}
		line = strings.TrimSpace(line)
		imgNameCh <- line
		if i%100 == 0 {
			fmt.Println("BatchAnalyzeByName begin to analyze line:", strconv.Itoa(i), ", image:", line)
		}
	}

	close(imgNameCh)
	// 所有worker都分析完成后退出
	wg.Wait()

	return nil
}

func batchAnalyzeByNameWorker(workerId int, jobs <-chan string, partial bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		if partial {
			_, err := analyzer.AnalyzeImagePartialByName(job)
			if err != nil {
				myutils.Logger.Error("batchAnalyzeWorker", strconv.Itoa(workerId), "analyze partial image", job, "failed with:", err.Error())
			}
		} else {
			_, err := analyzer.AnalyzeImageByName(job, true)
			if err != nil {
				myutils.Logger.Error("batchAnalyzeWorker", strconv.Itoa(workerId), "analyze image", job, "failed with:", err.Error())
			}
		}
	}
}
