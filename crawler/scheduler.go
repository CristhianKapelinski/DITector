package crawler

import "fmt"

// 负责整个爬虫的核心调度，启动goroutine等。

// 初始化一系列限制通道，通道初始化在config.go init()中实现
// 用于限制goroutine数量

var (
	// ChanLimitMainGoroutine 限制核心调度器goroutine数量
	ChanLimitMainGoroutine chan struct{}
)

// 定义一系列用于任务调度的关键字

var (
	// ChanKeyword 用于传递keyword，优先爬完有关一个keyword的所有镜像，再进入下一个keyword。
	ChanKeyword     = make(chan string)
	ChanRegRepoList chan RegisterRepoList__
)

// CrawlDockerHubStaged 划分阶段进行整个DockerHub的爬取。
// 未必进行实现，在实际实现中仍考虑任务分发全过程并发，而非分阶段在阶段内并发。
func CrawlDockerHubStaged() {
	// Stage1 发现所有可用关键字
	// Stage2 根据可用关键字爬取keyword->Repo List，发现全部Repo Name
	// Stage3 根据Repo Name爬取Repo的全部Tags
	// Stage4(可并入Stage3) 根据Tag爬取Tag对应的Arch History
}

// CoreScheduler 作为crawler运行时必须启动的goroutine，负责整个crawler内scraper的调度。
//
// 核心调度包括：
//
// 将keyword分发给ScrapeRegRepoListRecursive，生成下一个keyword，爬取当前keyword的仓库列表。
//
// 将regrepolist分发给ScrapeRepoInfo，爬取仓库metadata，爬取仓库所有tag的所有arch history。
func CoreScheduler(initKw string) {
	go func() {
		ChanKeyword <- initKw
	}()
	for {
		select {
		// 获取到新的keyword，将其传入ScrapeRegRepoListRecursive尝试
		case kw := <-ChanKeyword:
			// 每一个核心任务开始前申请一个核心goroutine
			ChanLimitMainGoroutine <- struct{}{}
			go func(kw string) {
				defer func() { <-ChanLimitMainGoroutine }()
				ScrapeRegRepoListRecursive(kw, "community")
			}(kw)
		case rrl := <-ChanRegRepoList:
			ChanLimitMainGoroutine <- struct{}{}
			go func(rrl RegisterRepoList__) {
				defer func() { <-ChanLimitMainGoroutine }()
				for _, s := range rrl.Summaries {
					ScrapeRepoInfo(s.Name, s.Source)
				}
			}(rrl)
		}
	}
}

// DistributeKeywordToScrapeRegRepoList 负责具体将Repo count<9000的keyword分发给ScrapeRegRepoListRecursive。
func DistributeKeywordToScrapeRegRepoList(kc chan string) {
	for k := range kc {
		// 尝试拿到
		fmt.Println(k)
	}
}
