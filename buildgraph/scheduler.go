package buildgraph

import (
	"runtime"
)

// scheduler.go 负责分发任务，从chan中获取内容并组织为合适的形式存入数据库

var (
	// chanLimitMainGoroutine 限制goroutine数量
	chanLimitMainGoroutine chan struct{}
)

var (
	chanRepository     = make(chan *Repository, runtime.NumCPU())
	chanTag            = make(chan *TagSource, runtime.NumCPU())
	chanImage          = make(chan *ImageSource, runtime.NumCPU())
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
		InsertRepositoryToMongo(repo)
	}
	chanDoneRepository <- struct{}{}
}

func StoreTagScheduler() {
	for tag := range chanTag {
		InsertTagToMongo(tag)
	}
	chanDoneTag <- struct{}{}
}

func StoreImageScheduler() {
	for image := range chanImage {
		InsertImageToMongo(image)
		InsertImageToNeo4j(image)
	}
	chanDoneImage <- struct{}{}
}
