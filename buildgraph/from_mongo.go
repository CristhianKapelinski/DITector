package buildgraph

import (
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
	"log"
	"runtime"
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
	chanGraphJob = make(chan GraphJob, runtime.NumCPU())
	chanDone     = make(chan struct{})
)

// StartFromMongo
// page是遍历mongodb时，以5个repo为1页对应的当前page
// pageSize是查询非library repo的tag API时，一页返回的tag数量
func StartFromMongo(page int64, pageSize int, pullCountThreshold int64) {
	beginTime := time.Now()
	fmt.Println("build from Mongo begin at:", myutils.GetLocalNowTimeStr())

	wg := sync.WaitGroup{}
	go loadDataFromMongo(page, pageSize, pullCountThreshold, chanGraphJob, &wg)
	go buildGraphFromMongo(chanGraphJob, chanDone)
	<-chanDone

	wg.Wait()
	fmt.Println("build from Mongo finished at:", myutils.GetLocalNowTimeStr())
	fmt.Println("total used time:", time.Since(beginTime))
	return
}

func loadDataFromMongo(page int64, pageSize int, pullCountThreshold int64, ch chan GraphJob, wg *sync.WaitGroup) {
	defer close(ch)

	beginTime := time.Now()
	if !myutils.GlobalDBClient.MongoFlag {
		log.Fatalln("loadDataFromMongo got error: MongoDB not online")
	}

	// 逐页查找repo
	var repoCnt = 0
	var repoPage int64 = page
	var repoPageSize int64 = 5
	for {
		repoDocs, err := myutils.GlobalDBClient.Mongo.FindRepositoriesByKeywordPaged(nil, repoPage, repoPageSize)
		if err != nil {
			myutils.Logger.Error(fmt.Sprintf("find repository in MongoDB page: %d, pagesize: %d, got error: %s", repoPage, repoPageSize, err))
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
			// 修改：仅对library和下载量>1000000的repo的全部tag进行构建，其他的仓库只构建前20tag
			// 修改：过滤掉windows系统的镜像
			var tagPage int64 = 1
			for {
				var tagDocs []*myutils.Tag
				tagFromAPIFlag := false

				// 下载量大的镜像/library镜像全量交给API获取
				if repoDoc.PullCount > pullCountThreshold || repoDoc.Namespace == "library" {
					//if repoDoc.PullCount > 100000 && tagPage == 1 {
					tagDocs, err = myutils.ReqTagsAllMetadata(repoDoc.Namespace, repoDoc.Name, 1, 100)
					if err != nil {
						myutils.Logger.Error(fmt.Sprintf("request all tags list of repository %s/%s, page: %d, pagesize: %d from Docker Hub API failed with: %s",
							repoDoc.Namespace, repoDoc.Name, 1, 100, err))
						continue
					}
					//// 如果拿满100条，那么已拿到第10页
					tagFromAPIFlag = true
					//tagPage = 10
				} else {
					// 其他镜像从mongodb获取
					tagDocs, err = myutils.GlobalDBClient.Mongo.FindTagsByRepoNamePaged(repoDoc.Namespace, repoDoc.Name, tagPage, int64(pageSize))
					if err != nil {
						myutils.Logger.Error(fmt.Sprintf("find tags for repository %s/%s in MongoDB page: %d, pagesize: %d, got error: %s", repoDoc.Namespace, repoDoc.Name, tagPage, pageSize, err))
						break
					}

					if len(tagDocs) == 0 {
						// 还是第一页，说明数据库里没记录到tag，从API拿pageSize个
						if tagPage == 1 {
							tagDocs, err = myutils.ReqTagsMetadata(repoDoc.Namespace, repoDoc.Name, 1, pageSize)
							if err != nil {
								myutils.Logger.Error(fmt.Sprintf("request tags list of repository %s/%s, page: %d, pagesize: %d from Docker Hub API failed with: %s",
									repoDoc.Namespace, repoDoc.Name, 1, pageSize, err))
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

				// 遍历repo的每个tag
				for _, tagDoc := range tagDocs {
					tagNeedUpdateFlag := false
					tagLastPushedTime, _ := time.Parse(time.RFC3339Nano, tagDoc.TagLastPushed)

					// 遍历tag的每个image信息
					for _, imgOfTag := range tagDoc.Images {
						// 跳过windows镜像，以及其他unknown镜像
						if imgOfTag.OS == "windows" || (imgOfTag.Architecture == "unknown" && imgOfTag.OS == "unknown") {
							continue
						}

						imgDigest := imgOfTag.Digest
						// 尝试从数据库拿image元数据
						imgMeta, err := myutils.GlobalDBClient.Mongo.FindImageByDigest(imgDigest)
						// 数据库中没有，从API获取
						if err != nil {
							imgAPIMetas, e := myutils.ReqImagesMetadata(repoDoc.Namespace, repoDoc.Name, tagDoc.Name)
							if e != nil {
								myutils.Logger.Error(fmt.Sprintf("get images metadata of tag %s/%s:%s from API failed with: %s", repoDoc.Namespace, repoDoc.Name, tagDoc.Name, e))
								continue
							} else {
								for _, imgAPIMeta := range imgAPIMetas {
									// 检查tag数据是否需要更新
									// 存在至少一个image上传时间比tag上传时间靠后，tag元数据需要更新
									imgLastPushedTime, _ := time.Parse(time.RFC3339Nano, imgAPIMeta.LastPushed)
									if imgLastPushedTime.After(tagLastPushedTime) {
										tagNeedUpdateFlag = true
									}

									// 将元数据存入数据库
									wg.Add(1)
									go func(imgMeta *myutils.Image) {
										defer wg.Done()
										if e := myutils.GlobalDBClient.Mongo.UpdateImage(imgMeta); e != nil {
											myutils.Logger.Error("update metadata of image", imgMeta.Digest, "failed with:", e.Error())
										}
									}(imgAPIMeta)
								}

								// 存在image比tag推送晚，从API重新获取tag信息并存入数据库
								if tagNeedUpdateFlag {
									tagAPIDoc, err := myutils.ReqTagMetadata(repoDoc.Namespace, repoDoc.Name, tagDoc.Name)
									if err != nil {
										myutils.Logger.Error(fmt.Sprintf("get tag metadata of tag %s/%s:%s from API failed with: %s", repoDoc.Namespace, repoDoc.Name, tagDoc.Name, err))
										break
									}
									// 获取成功后将要生产任务的tag信息刷新
									tagDoc = tagAPIDoc
									// 从API获取的tag元数据重新存入数据库
									wg.Add(1)
									go func(tagMeta *myutils.Tag) {
										defer wg.Done()
										if e := myutils.GlobalDBClient.Mongo.UpdateTag(tagMeta); e != nil {
											myutils.Logger.Error("update metadata of tag", tagMeta.RepositoryNamespace, tagMeta.RepositoryName, tagMeta.Name, "failed with:", e.Error())
										}
									}(tagDoc)
								}

								// tag已经是最新，image完全从API获取，遍历API获取的image元数据生产任务
								for _, imgAPIMeta := range imgAPIMetas {
									// 跳过windows镜像，以及其他unknown镜像
									if imgAPIMeta.OS == "windows" || (imgAPIMeta.Architecture == "unknown" && imgAPIMeta.OS == "unknown") {
										continue
									}

									ch <- GraphJob{
										Registry:      "docker.io",
										RepoNamespace: repoDoc.Namespace,
										RepoName:      repoDoc.Name,
										TagName:       tagDoc.Name,
										ImageMeta:     imgAPIMeta,
									}
								}
							}
						} else {
							// 数据库中有，生成对应的任务
							ch <- GraphJob{
								Registry:      "docker.io",
								RepoNamespace: repoDoc.Namespace,
								RepoName:      repoDoc.Name,
								TagName:       tagDoc.Name,
								ImageMeta:     imgMeta,
							}
						}
					}
				}

				// 从API获取tag列表且没拿满100个，直接退出当前repo
				if tagFromAPIFlag && len(tagDocs) < 100 {
					break
				}

				// tag翻页
				tagPage++
			}

		}

		if repoPage%2 == 0 {
			fmt.Println("generated all job for repo:", repoCnt, ", page:", repoPage, ", time used:", time.Since(beginTime))
		}

		// repo翻页
		repoPage++
	}
}

func buildGraphFromMongo(ch chan GraphJob, chDone chan struct{}) {
	wg := sync.WaitGroup{}
	for i := 0; i < myutils.GlobalConfig.MaxThread; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			buildGraphFromMongoWorker(workerId, ch, chDone)
		}(i)
	}
	wg.Wait()
	chDone <- struct{}{}
}

func buildGraphFromMongoWorker(i int, ch chan GraphJob, chDone chan struct{}) {
	for job := range ch {
		// 是否并发安全？？？？？
		// 亟需测试！！！
		myutils.GlobalDBClient.Neo4j.InsertImageToNeo4j(fmt.Sprintf("%s/%s/%s:%s@%s", job.Registry, job.RepoNamespace, job.RepoName, job.TagName, job.ImageMeta.Digest), job.ImageMeta)
	}
}
