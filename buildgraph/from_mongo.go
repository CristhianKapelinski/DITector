package buildgraph

import (
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
	"log"
	"sync"
	"time"
)

type GraphJob struct {
	Registry      string
	RepoNamespace string
	RepoName      string
	TagName       string
	ImageMeta     *myutils.Image
}

var (
	chanGraphJob = make(chan GraphJob)
	chanDone     = make(chan struct{})
)

func StartFromMongo() {
	beginTime := time.Now()
	fmt.Println("build from Mongo begin at:", myutils.GetLocalNowTimeStr())

	wg := sync.WaitGroup{}
	go loadDataFromMongo(chanGraphJob, &wg)
	go buildGraphFromMongo(chanGraphJob, chanDone)
	<-chanDone

	wg.Wait()
	fmt.Println("build from Mongo finished at:", myutils.GetLocalNowTimeStr())
	fmt.Println("total used time:", time.Since(beginTime))
	return
}

func loadDataFromMongo(ch chan GraphJob, wg *sync.WaitGroup) {
	defer close(ch)
	if !myutils.GlobalDBClient.MongoFlag {
		log.Fatalln("loadDataFromMongo got error: MongoDB not online")
	}

	// 逐页查找repo
	var repoCnt = 0
	var repoPage int64 = 1
	var pageSize int64 = 5
	for {
		repoDocs, err := myutils.GlobalDBClient.Mongo.FindRepositoriesByKeywordPaged(nil, repoPage, pageSize)
		if err != nil {
			myutils.Logger.Error(fmt.Sprintf("find repository in MongoDB page: %d, pagesize: %d, got error: %s", repoPage, pageSize, err))
			continue
		}
		// 进程结束标志，没有更多repo了
		if len(repoDocs) == 0 {
			break
		}

		// 遍历每个repo
		for _, repoDoc := range repoDocs {
			repoCnt++

			// 对repo逐页查找tag
			var tagPage int64 = 1
			for {
				var tagDocs []*myutils.Tag
				tagFromAPIFlag := false

				// 下载量大的镜像的第一页交给API获取
				if repoDoc.PullCount > 100000 && tagPage == 1 {
					tagDocs, err = myutils.ReqTagsMetadata(repoDoc.Namespace, repoDoc.Name, 1, 100)
					if err != nil {
						myutils.Logger.Error(fmt.Sprintf("request tags list of repository %s/%s, page: %d, pagesize: %d from Docker Hub API failed with: %s",
							repoDoc.Namespace, repoDoc.Name, 1, 100, err))
						continue
					}
					// 如果拿满100条，那么已拿到第10页
					tagFromAPIFlag = true
					tagPage = 10
				} else {
					tagDocs, err = myutils.GlobalDBClient.Mongo.FindTagsByRepoNamePaged(repoDoc.Namespace, repoDoc.Name, tagPage, 10)
					if err != nil {
						myutils.Logger.Error(fmt.Sprintf("find tags for repository %s/%s in MongoDB page: %d, pagesize: %d, got error: %s", repoDoc.Namespace, repoDoc.Name, tagPage, 10, err))
						break
					}

					if len(tagDocs) == 0 {
						// 还是第一页，说明数据库里没记录到tag，从API拿100个够了
						if tagPage == 1 {
							tagDocs, err = myutils.ReqTagsMetadata(repoDoc.Namespace, repoDoc.Name, 1, 100)
							if err != nil {
								myutils.Logger.Error(fmt.Sprintf("request tags list of repository %s/%s, page: %d, pagesize: %d from Docker Hub API failed with: %s",
									repoDoc.Namespace, repoDoc.Name, 1, 100, err))
								break
							}
							tagFromAPIFlag = true
						} else {
							// 不是第一页，已遍历全部tag，退出当前repo
							break
						}
					}
				}

				// 从API获取的结果，向数据库中备份一下
				if tagFromAPIFlag {
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

				// 生成存储任务
				for _, tagDoc := range tagDocs {
					imgFromAPIFlag := false
					// 遍历tag的
					for _, imgOfTag := range tagDoc.Images {
						imgDigest := imgOfTag.Digest
						// 尝试从数据库拿image元数据
						imgMeta, err := myutils.GlobalDBClient.Mongo.FindImageByDigest(imgDigest)
						if err != nil {

						}
						if imgFromAPIFlag {

						}
					}
				}

				// 从API获取且没拿满100个，直接退出
				if tagFromAPIFlag && len(tagDocs) < 100 {
					break
				}
				// tag翻页
				tagPage++
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
				fmt.Println("generated all job for repo:", repoCnt, ", page:", repoPage)
			}
		}

		// repo翻页
		repoPage++
	}
}

func buildGraphFromMongo(ch chan GraphJob, chDone chan struct{}) {
	for job := range ch {

	}
	chDone <- struct{}{}
}
