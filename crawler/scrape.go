package crawler

import (
	"fmt"
	"github.com/gocolly/colly"
	"strconv"
	"sync"
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
	var (
		pages    int
		stop     bool
		stopLock sync.Mutex
	)
	// 启动一个专门处理chRegRepoList的函数，用于
	chRegRepoList := make(chan RegisterRepoList__)
	// 起始时需要将stopLock上锁
	stopLock.Lock()
	go func(ch chan RegisterRepoList__) {
		for rrl := range ch {
			// pages为0，代表i为1，即当前结果为当前keyword的第一次爬取结果
			if pages == 0 {
				cnt := rrl.Count
				// Count过大，退出
				if cnt > 9000 {
					fmt.Println("[INFO] Count > 9000 for keyword: ", keyword)
					stop = true
				} else {
					if (cnt % 100) != 0 {
						pages = rrl.Count/100 + 1
					} else {
						pages = rrl.Count / 100
					}
					ChanRegRepoList <- rrl
				}
				// 第一页判断后将stopLock解锁
				stopLock.Unlock()
			} else {
				// pages不为0，只负责转发
				ChanRegRepoList <- rrl
			}

		}
	}(chRegRepoList)

	c := GetRegRepoListCollector(chRegRepoList)
	// 第一页爬取主动进行
	i := 1
	c.Visit(GetRegURL(keyword, source, strconv.Itoa(i), "100"))

	// Count>9000，在核心调度器消费掉下一个keyword后退出。
	// stopLock阻塞当前任务的主协程，等待内部管道放行
	stopLock.Lock()
	if stop {
		stopLock.Unlock()
		fmt.Println("[INFO] Count > 9000, Stop ScrapeRegRepoListRecursive for keyword: ", keyword)
		close(chRegRepoList)
		ChanKeyword <- GenerateNextKeyword(keyword, false)
		return
	}
	stopLock.Unlock()

	// colly设计上存在问题，Visit本质上是调用内置函数scrape实现的，
	// 内部先组织请求头和数据，组织好后掉用HandleOnRequest，
	// 然后再调Cache，Cache内调用Do，Do内调time.Sleep实现请求间的时延。
	// 所以每一个goroutine的OnRequest都瞬间调用了，然后Delay，然后实际进行HTTP请求，然后HandleOnResponse。
	// 导致在输出上只有多个Visit的OnResponse间间隔了预设的Delay时间。
	wg := sync.WaitGroup{}
	for i = 2; i <= pages; i++ {
		wg.Add(1)
		go func(j int) {
			c.Visit(GetRegURL(keyword, source, strconv.Itoa(j), "100"))
			wg.Done()
		}(i)
	}

	wg.Wait()

	close(chRegRepoList)

	// 一定是函数主体都处理好才向ChanKeyword中传数据，因为ChanKeyword是无缓冲通道，在核心调度器会阻塞。
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
