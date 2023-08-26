package buildgraph

import (
	"go.mongodb.org/mongo-driver/mongo"
	"myutils"
	"runtime"
)

// scheduler.go 负责分发任务，从chan中获取内容并组织为合适的形式存入数据库

var (
	// chanLimitMainGoroutine 限制goroutine数量
	chanLimitMainGoroutine chan struct{}
)

var (
	chanRepository     = make(chan *myutils.Repository, runtime.NumCPU())
	chanTag            = make(chan *myutils.TagSource, runtime.NumCPU())
	chanImage          = make(chan *myutils.ImageSource, runtime.NumCPU())
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

func StoreRepositoryScheduler() {
	for repo := range chanRepository {
		err := myMongo.InsertRepository(repo)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				//fmt.Println("[WARN] Mongo Duplicate when inserting repository", repo.Namespace, repo.RepositoryName, ", repository already exists")
			} else {
				myutils.LogDockerCrawlerString("[ERROR] Mongo insert repository", repo.Namespace+"/"+repo.Name, "failed with err:", err.Error())
			}
		}
	}
	chanDoneRepository <- struct{}{}
}

func StoreTagScheduler() {
	for tag := range chanTag {
		err := myMongo.InsertTag(tag)
		if err != nil {
			myutils.LogDockerCrawlerString("[ERROR] Mongo insert tag", tag.Namespace+"/"+tag.RepositoryName+"/"+tag.Name, "failed with err:", err.Error())
		}
	}
	chanDoneTag <- struct{}{}
}

func StoreImageScheduler() {
	for image := range chanImage {
		err := myMongo.InsertImage(image)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				//fmt.Println("[WARN] Mongo Duplicate when inserting repository", repo.Namespace, repo.RepositoryName, ", repository already exists")
			} else {
				myutils.LogDockerCrawlerString("[ERROR] Mongo insert image", image.Image.Digest, "failed with err:"+err.Error())
			}
		}
		myNeo4jDriver.InsertImageToNeo4j(image)
	}
	chanDoneImage <- struct{}{}
}
