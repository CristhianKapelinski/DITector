package scripts

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/Musso12138/docker-scan/analyzer"
	"github.com/Musso12138/docker-scan/myutils"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AnalyzePullCountOverThreshold 分析pull_count > threshold时
func AnalyzePullCountOverThreshold(threshold int64, tagNum int) error {
	// 配置线程数
	maxThreads := runtime.NumCPU()
	if myutils.GlobalConfig.MaxThread > 0 && myutils.GlobalConfig.MaxThread < maxThreads {
		maxThreads = myutils.GlobalConfig.MaxThread
		runtime.GOMAXPROCS(maxThreads)
	}

	// 初始化控制并发线程数的管道
	jobCh := make(chan job)
	wg := sync.WaitGroup{}

	for w := 1; w <= maxThreads; w++ {
		wg.Add(1)
		go analyzeThresholdWorker(w, jobCh, &wg)
	}

	wg.Add(1)
	go jobGeneratorThreshold(threshold, tagNum, jobCh, &wg)

	wg.Wait()

	return nil
}

// jobGeneratorThreshold 从MongoDB读取repo数据，生成任务传入通道
func jobGeneratorThreshold(threshold int64, tagNum int, jobCh chan<- job, wg *sync.WaitGroup) {
	defer close(jobCh)
	defer wg.Done()
	if !myutils.GlobalDBClient.MongoFlag {
		log.Fatalln("jobGeneratorAll got error: MongoDB not online")
	}

	var repoCnt = 0
	var repoPage int64 = 1
	var pageSize int64 = 5
	for {
		repoDocs, err := myutils.GlobalDBClient.Mongo.FindRepositoriesByPullCountPaged(threshold, repoPage, pageSize)
		if err != nil {
			myutils.Logger.Error(fmt.Sprintf("find repository in MongoDB pull_count > %d, page: %d, pagesize: %d, got error: %s", threshold, repoPage, pageSize, err))
			continue
		}
		// 进程结束标志
		if len(repoDocs) == 0 {
			break
		}

		// 根据tag生成任务
		for _, repoDoc := range repoDocs {
			repoCnt++

			// 从API获取最近更新的tag信息
			tagDocs, err := myutils.ReqTagsMetadata(repoDoc.Namespace, repoDoc.Name, 1, tagNum)
			if err != nil {
				myutils.Logger.Error(fmt.Sprintf("request tags for repository %s/%s from API got error: %s", repoDoc.Namespace, repoDoc.Name, err))
				continue
			}

			// 向数据库中备份一下
			for _, tagDoc := range tagDocs {
				wg.Add(1)
				go func(tagMetadata *myutils.Tag) {
					defer wg.Done()
					if e := myutils.GlobalDBClient.Mongo.UpdateTag(tagMetadata); e != nil {
						myutils.Logger.Error("update metadata of tag", tagMetadata.RepositoryNamespace, tagMetadata.RepositoryName, tagMetadata.Name, "failed with:", e.Error())
					}
				}(tagDoc)
			}

			// 检查时间顺序，顺序不对从API拿新的repo元数据
			if len(tagDocs) > 0 && tagDocs[0].LastUpdated.After(repoDoc.LastUpdated) {
				repo, err := myutils.ReqRepoMetadata(repoDoc.Namespace, repoDoc.Name)
				if err != nil {
					myutils.Logger.Error(fmt.Sprintf("request metadata of repository %s/%s from API got error: %s", repoDoc.Namespace, repoDoc.Name, err))
				} else {
					if e := myutils.GlobalDBClient.Mongo.UpdateRepository(repo); e != nil {
						myutils.Logger.Error("update metadata of repo", repo.Namespace, repo.Name, "failed with:", e.Error())
					}
				}
			}

			// 生产任务
			for _, tagDoc := range tagDocs {
				jobCh <- job{
					name:    fmt.Sprintf("%s/%s:%s", repoDoc.Namespace, repoDoc.Name, tagDoc.Name),
					partial: false,
				}
			}

			if repoCnt%100 == 0 {
				fmt.Println("generated all job for repo:", repoCnt)
			}
		}

		repoPage++
	}
}

func analyzeThresholdWorker(workerId int, jobCh <-chan job, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobCh {
		if j.partial {
			_, err := analyzer.AnalyzeImagePartialByName(j.name)
			if err != nil {
				myutils.Logger.Error("analyzeThresholdWorker", strconv.Itoa(workerId), "analyze partial image", j.name, "failed with:", err.Error())
			}
		} else {
			_, err := analyzer.AnalyzeImageByName(j.name, true)
			if err != nil {
				myutils.Logger.Error("analyzeThresholdWorker", strconv.Itoa(workerId), "analyze image", j.name, "failed with:", err.Error())
			}
		}
	}
}

// ScanAllSecretsInImageMetadata scan all secrets in metadata
// images, and write results to mongo.dockerhub.results.
// log to file
// /data/docker-crawler/results/secrets-in-image-metadata.log
func ScanAllSecretsInImageMetadata() {
	//cursor, err := myutils.GlobalDBClient.Mongo.ImgColl.Find(context.TODO(), bson.D{})
	//if err != nil {
	//	logself(myutils.LogLevelStr.Error, "traverse images failed with:", err.Error())
	//	log.Fatalln(err)
	//}
	//defer cursor.Close(context.TODO())
	//cnt := 0
	//
	//for cursor.Next(context.TODO()) {
	//	cnt++
	//	logself(myutils.LogLevelStr.Debug, "begin to scan", strconv.Itoa(cnt))
	//
	//	targetImage := new(myutils.Image)
	//	err := cursor.Decode(targetImage)
	//	if err != nil {
	//		logself(myutils.LogLevelStr.Error, "decode image failed with:", err.Error())
	//		continue
	//	}
	//
	//	imgres := new(myutils.ImageResult)
	//	imgres.Digest = targetImage.Digest
	//	imgres.LastAnalyzed = myutils.GetLocalNowTimeStr()
	//
	//	imgres, err = analyzer.AnalyzeImagePartialByName(targetImage)
	//	if err != nil {
	//		logself(myutils.LogLevelStr.Error, "analyze metadata of image", imgres.Digest, "failed with:", err.Error())
	//		continue
	//	}
	//
	//	err = myutils.GlobalDBClient.Mongo.UpdateImgResult(imgres)
	//	if err != nil {
	//		logself(myutils.LogLevelStr.Error, "insert image result failed with:", err.Error())
	//		continue
	//	}
	//}
}

// ScanTop100DownstreamImagesVul scan vulnerabilities of top 100
// downstream images according to file dependent-weights-top100.txt
// with SCA tools, now only supports using
// anchore/grype: https://github.com/anchore/grype
func ScanTop100DownstreamImagesVul() {
	resultFilePath := "/data/docker-crawler/results/dependent-weights/dependent-weights-downstream-top100.txt"
	resultFile, _ := os.Open(resultFilePath)
	defer func() {
		resultFile.Close()
		fmt.Println("[INFO] read done:", resultFilePath)
	}()

	scanner := bufio.NewReader(resultFile)
	for i := 0; ; i++ {
		b, err := scanner.ReadBytes('\n')
		if err != nil {
			// 读到fileRepository结尾，退出
			if err == io.EOF {
				break
			}
			fmt.Println("[ERROR] Fail to ReadLine in", resultFilePath, ": Line", i, ", err:", err)
			break
		}

		var record = new(RecordWithNodeID)
		err = json.Unmarshal(b, record)
		if err != nil {
			myutils.Logger.Error("json unmarshal failed with:", err.Error())
			continue
		}

		realTagName := strings.ReplaceAll(record.TagName, "$", ".")
		fmt.Println(record.Namespace, record.RepositoryName, realTagName, record.ImageDigest)

		imageFullName := record.Namespace + "/" + record.RepositoryName + ":" + realTagName
		resultPath := path.Join("/data/docker-crawler/results/sca-results/anchore-downstream-top100/",
			record.Namespace+"-"+record.RepositoryName+"-"+realTagName+".json")
		beginTime := time.Now()

		cmd := exec.Command("/home/hequan/anchore/grype/grype", imageFullName, "-o", "json", "--file", resultPath)
		if err = cmd.Run(); err != nil {
			myutils.Logger.Error("run shell command failed with:", err.Error())
			fmt.Println("[ERROR] run shell command failed with:", err)
		}

		fmt.Println("Finish:", i, ", Total Time:", time.Since(beginTime))
	}
}
