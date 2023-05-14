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
		//fmt.Println(repo.Namespace, repo.RepositorySource, repo.Tags)
		InsertRepositoryToMongo(repo)
		//r, e := FindRepositoryFromMongoByName(repo.Namespace, repo.RepositorySource)
		//if e != nil {
		//	fmt.Println(e)
		//} else {
		//	fmt.Println("Find: ", *r)
		//}
	}
	chanDoneRepository <- struct{}{}
}

func StoreTagScheduler() {
	for tag := range chanTag {
		//fmt.Println(tag.Namespace, tag.RepositorySource, tag.TagSource)
		InsertTagToMongo(tag)
		//fmt.Sprintf(tag.Namespace)
	}
	chanDoneTag <- struct{}{}
}

func StoreImageScheduler() {
	for image := range chanImage {
		//fmt.Println(image.Namespace, image.RepositorySource, image.ArchSource.Digest)
		InsertImageToMongo(image)
		//fmt.Sprintf(image.Tag)
	}
	chanDoneImage <- struct{}{}
}
