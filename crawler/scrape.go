package crawler

import (
	"github.com/gocolly/colly"
)

// 实现一些统一的遍历爬取

// ScrapeDockerHubRecursive 用于自动化爬取Docker Hub仓库。
// 主要工作流程为:
// 根据当前关键字visit一次第一页，拿到count。
// 如果count<9000，把关键字的分发下去进一步调用ScrapeRegRepoListRecursive；
// 如果count>9000，则继续根据当前关键字生成下一个关键字。
func ScrapeDockerHubRecursive() {

}

// ScrapeRegRepoListRecursive 根据keyword返回结果进入不同的分支：
//
// count>=9000时，将GenerateNextKeyword(keyword, false)传入ChanKeyword，然后退出函数。
//
// count<9000时，将GenerateNextKeyword(keyword, true)传入ChanKeyword，同时递归爬取该keyword对应的全部RegisterRepoList__，
// 传入ChanRegRepoList。
func ScrapeRegRepoListRecursive(keyword, source string) {
	// 启动一个专门处理chRegRepoList的函数，用来判断是否应在第一轮退出程序
	chRegRepoList := make(chan RegisterRepoList__)
	go func(chRegRepoList chan RegisterRepoList__) {
		for rrl := range chRegRepoList {
			// Count过大，退出
			if rrl.Count > 9000 {
				// 一定是函数主体都处理好才向ChanKeyword中传数据，因为ChanKeyword是无缓冲通道，在核心调度器会阻塞。
				ChanKeyword <- GenerateNextKeyword(keyword, false)
			} else {
				ChanRegRepoList <- rrl
			}
		}
	}(chRegRepoList)

	// page_size=100情况下，一般会有很多页，所以可以新建一个
	c := GetRegRepoListCollector(chRegRepoList)

	for _, i := range []string{"1", "2", "3"} {
		if err := c.Visit(GetRegURL(keyword, source, i, "4")); err != nil {
			continue
		}
	}

	close(chRegRepoList)
	ChanKeyword <- GenerateNextKeyword(keyword, true)
}

// ScrapeRepoInfo 用于爬取指定repo的metadata，全部tag，以及每个tag对应镜像的history信息。
// 考虑在内部进一步将metadata和tag信息持久化。
func ScrapeRepoInfo(namespace, repo string) {
	// 思路1-------------------------
	// 建立有效管道每阶段都在传数据
	// ch1 := make(chan Repository__)
	// ScrapeRepoMetadata 爬Metadata，结果传进ch1

	// 读ch1
	// ch2 := make(chan TagReceiver__ 收tag list)
	// ScrapeRepoTagsRecursive爬tag list传进ch2

	// 思路2-------------------------
	// GetCollector时候传入&Repository__
	// ch := make(chan TagReceiver__ 收tag list)
	// ScrapeRepoTagsRecursive爬tag list传进ch
	// 读ch
	// for t := range ch {
	// 	for _, tag := range t.Results {
	// 		进一步爬每个tag的Archs
	//
	//	}
	// }
	// 后续都在这个基础上
}

// ScrapeRepoMetadata 用于爬取指定repo的metadata，返回一个。
func ScrapeRepoMetadata(namespace, repo string) {

}

// ScrapeRepoTagsRecursive 递归爬取指定Repo的全部Tag记录。
func ScrapeRepoTagsRecursive(c *colly.Collector, namespace, repo string) {
	for _, i := range []string{"1"} {
		if err := c.Visit(GetRepoTagsURL(namespace, repo, i, "100")); err != nil {
			continue
		}
	}
}
