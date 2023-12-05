package scripts

import (
	"fmt"
	"github.com/Musso12138/docker-scan/analyzer"
	"github.com/Musso12138/docker-scan/myutils"
	"log"
	"runtime"
	"strconv"
	"sync"
)

type job struct {
	name    string
	partial bool
}

func AnalyzeAll() error {
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
		go analyzeAllWorker(w, jobCh, &wg)
	}

	wg.Add(1)
	go jobGeneratorAll(jobCh, &wg)

	wg.Wait()

	return nil
}

// jobGeneratorAll 从MongoDB读取repo数据
func jobGeneratorAll(jobCh chan<- job, wg *sync.WaitGroup) {
	defer close(jobCh)
	defer wg.Done()
	if !myutils.GlobalDBClient.MongoFlag {
		log.Fatalln("jobGeneratorAll got error: MongoDB not online")
	}

	var repoCnt = 0
	var repoPage int64 = 1
	var pageSize int64 = 5
	for {
		repoDocs, err := myutils.GlobalDBClient.Mongo.FindRepositoriesByKeywordPaged(nil, repoPage, pageSize)
		if err != nil {
			myutils.Logger.Error(fmt.Sprintf("find repository in MongoDB page: %d, pagesize: %d, got error: %s", repoPage, pageSize, err))
			continue
		}
		// 进程结束标志
		if len(repoDocs) == 0 {
			break
		}

		// 根据tag生成任务
		for _, repoDoc := range repoDocs {
			repoCnt++
			tagDocs, err := myutils.GlobalDBClient.Mongo.FindTagsByRepoNamePaged(repoDoc.Namespace, repoDoc.Name, 1, 10)
			if err != nil {
				myutils.Logger.Error(fmt.Sprintf("find tags for repository %s/%s in MongoDB page: %d, pagesize: %d, got error: %s", repoDoc.Namespace, repoDoc.Name, 1, 10, err))
				continue
			}

			// 集合中没有tag信息，从API获取
			fromAPIFlag := false
			if len(tagDocs) == 0 {
				tagDocs, err = myutils.ReqTagsMetadata(repoDoc.Namespace, repoDoc.Name, 1, 10)
				if err != nil {
					myutils.Logger.Error(fmt.Sprintf("request tags for repository %s/%s from API got error: %s", repoDoc.Namespace, repoDoc.Name, err))
					continue
				}
				fromAPIFlag = true
				// 从API获取的部分向数据库中备份一下
				for _, tagDoc := range tagDocs {
					wg.Add(1)
					go func(tagMetadata *myutils.Tag) {
						defer wg.Done()
						if e := myutils.GlobalDBClient.Mongo.UpdateTag(tagMetadata); e != nil {
							myutils.Logger.Error("update metadata of tag", tagMetadata.RepositoryNamespace, tagMetadata.RepositoryName, tagMetadata.Name, "failed with:", e.Error())
						}
					}(tagDoc)
				}
			}

			// 检查时间顺序，顺序不对从API拿新的repo和tags
			if len(tagDocs) > 0 && tagDocs[0].LastUpdated.After(repoDoc.LastUpdated) {
				repo, err := myutils.ReqRepoMetadata(repoDoc.Namespace, repoDoc.Name)
				if err != nil {
					myutils.Logger.Error(fmt.Sprintf("request metadata of repository %s/%s from API got error: %s", repoDoc.Namespace, repoDoc.Name, err))
				} else {
					if e := myutils.GlobalDBClient.Mongo.UpdateRepository(repo); e != nil {
						myutils.Logger.Error("update metadata of repo", repo.Namespace, repo.Name, "failed with:", e.Error())
					}
					// tag已经是从API获取的了，无需重复获取
					if !fromAPIFlag {
						tagDocs, err = myutils.ReqTagsMetadata(repoDoc.Namespace, repoDoc.Name, 1, 10)
						if err != nil {
							myutils.Logger.Error(fmt.Sprintf("request tags for repository %s/%s from API got error: %s", repoDoc.Namespace, repoDoc.Name, err))
							continue
						}
						// 从API获取的部分向数据库中备份一下
						for _, tagDoc := range tagDocs {
							wg.Add(1)
							go func(tagMetadata *myutils.Tag) {
								defer wg.Done()
								if e := myutils.GlobalDBClient.Mongo.UpdateTag(tagMetadata); e != nil {
									myutils.Logger.Error("update metadata of tag", tagMetadata.RepositoryNamespace, tagMetadata.RepositoryName, tagMetadata.Name, "failed with:", e.Error())
								}
							}(tagDoc)
						}
					}
				}
			}

			// 生产任务
			// 根据下载量，>10000的repo的最近3个tag分析内容，其他全部tag都是部分分析
			tagCnt := 1
			var partial bool
			for _, tagDoc := range tagDocs {
				if repoDoc.PullCount > 10000 && tagCnt <= 3 {
					partial = false
				} else {
					partial = true
				}
				jobCh <- job{
					name:    fmt.Sprintf("%s/%s:%s", repoDoc.Namespace, repoDoc.Name, tagDoc.Name),
					partial: partial,
				}
				tagCnt++
			}

			if repoCnt%100 == 0 {
				fmt.Println("generated all job for repo:", repoCnt)
			}
		}

		repoPage++
	}
}

func analyzeAllWorker(workerId int, jobCh <-chan job, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobCh {
		if j.partial {
			_, err := analyzer.AnalyzeImagePartialByName(j.name)
			if err != nil {
				myutils.Logger.Error("analyzeAllWorker", strconv.Itoa(workerId), "analyze partial image", j.name, "failed with:", err.Error())
			}
		} else {
			_, err := analyzer.AnalyzeImageByName(j.name, true)
			if err != nil {
				myutils.Logger.Error("analyzeAllWorker", strconv.Itoa(workerId), "analyze image", j.name, "failed with:", err.Error())
			}
		}
	}
}
