package crawler

import (
	"fmt"
	"github.com/gocolly/colly"
	"math/rand"
	"strconv"
	"sync"
	"time"
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
	ch := make(chan RegisterRepoList__)
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
						pages = cnt/100 + 1
					} else {
						pages = cnt / 100
					}
					chanRegRepoList <- rrl
				}
				// 第一页判断后将stopLock解锁
				stopLock.Unlock()
			} else {
				// pages不为0，只负责转发
				chanRegRepoList <- rrl
			}

		}
	}(ch)

	c := GetRegRepoListCollector(ch)
	// 第一页爬取主动进行
	i := 1
	c.Visit(GetRegURL(keyword, source, strconv.Itoa(i), "100"))

	// Count>9000，在核心调度器消费掉下一个keyword后退出。
	// stopLock阻塞当前任务的主协程，等待内部管道放行
	stopLock.Lock()
	if stop {
		stopLock.Unlock()
		fmt.Println("[INFO] Count > 9000, Stop ScrapeRegRepoListRecursive for keyword: ", keyword)
		close(ch)
		chanKeyword <- GenerateNextKeyword(keyword, false)
		return
	}
	stopLock.Unlock()

	// colly设计上存在问题，Visit本质上是调用内置函数scrape实现的，
	// 内部先组织请求头和数据，组织好后掉用HandleOnRequest，
	// 然后再调Cache，Cache内调用Do，Do内调time.Sleep实现请求间的时延。
	// 所以每一个goroutine的OnRequest都瞬间调用了，然后Delay，然后实际进行HTTP请求，然后HandleOnResponse。
	// 导致在输出上只有多个Visit的OnResponse间间隔了预设的Delay时间。
	for i = 2; i <= pages; i++ {
		if err := c.Visit(GetRegURL(keyword, source, strconv.Itoa(i), "100")); err != nil {
			fmt.Println("[ERROR] In ScrapeRegRepoListRecursive while visiting: ",
				GetRegURL(keyword, source, strconv.Itoa(i), "100"))
		}
	}

	c.Wait()

	close(ch)

	// 一定是函数主体都处理好才向ChanKeyword中传数据，因为ChanKeyword是无缓冲通道，在核心调度器会阻塞。
	chanKeyword <- GenerateNextKeyword(keyword, true)
}

// ScrapeRepoInfo 用于爬取仓库namespace/repository的metadata，全部tag，以及每个tag对应镜像的history信息。
// 考虑根据这里的结果进一步将metadata和tag信息持久化。
func ScrapeRepoInfo(namespace, repository string) {
	var repo Repository__

	// 爬取Metadata
	cm := GetRepoMetadataCollector(&repo)
	cm.Visit(GetRepoMetaURL(namespace, repository))

	// 爬所有Tags，可能涉及多页，建管道维护
	chTags := make(chan TagReceiver__)
	ct := GetRepoTagsCollector(chTags)
	var (
		cur      = 1
		pages    int
		pageLock sync.Mutex
	)

	pageLock.Lock()
	go func(ch chan TagReceiver__) {
		for tags := range ch {
			if pages == 0 {
				cnt := tags.Count
				if cnt > 100 {
					if (cnt % 100) != 0 {
						pages = cnt/100 + 1
					} else {
						pages = cnt / 100
					}
				}
				pageLock.Unlock()
				// 第一页的结果别忘了存进来
				repo.Tags = append(repo.Tags, tags.Results...)
			} else {
				repo.Tags = append(repo.Tags, tags.Results...)
			}
		}
	}(chTags)
	// 访问第一页
	ct.Visit(GetRepoTagsURL(namespace, repository, strconv.Itoa(cur), "100"))

	// 尝试获取锁，获取的一瞬间就可以关闭锁，做一次pages的同步
	pageLock.Lock()
	pageLock.Unlock()

	// 获取全部tags
	for cur = 2; cur <= pages; cur++ {
		ct.Visit(GetRepoTagsURL(namespace, repository, strconv.Itoa(cur), "100"))
	}
	// 获取后关闭通道
	close(chTags)

	// 爬每个Tag的所有Arch History
	// 并发导致随机延时只能短暂延缓达到Rate-Limit的速度
	//limit := make(chan struct{}, 5)
	//wg := sync.WaitGroup{}
	//for i, _ := range repo.Tags {
	//	limit <- struct{}{}
	//	wg.Add(1)
	//	go func(i int) {
	//		defer func() {
	//			wg.Done()
	//			<-limit
	//		}()
	//		// 引入随机延时，防止快速达到限制
	//		rd := time.Duration(rand.Int63n(int64(5 * time.Second)))
	//		time.Sleep(rd)
	//		ca := GetImageHistoryCollector(&repo.Tags[i].Archs)
	//		ca.Visit(GetImageHistoryURL(repo.Namespace, repo.Name, repo.Tags[i].Name))
	//	}(i)
	//}
	//wg.Wait()

	// 随机延时可以在某一刻忽然将Rate-Limit补满，对单IP有效
	// Test显示71个tag用时142s
	for i, _ := range repo.Tags {
		// 引入随机延时，防止快速达到限制
		rd := time.Duration(rand.Int63n(int64(2 * time.Second)))
		time.Sleep(rd)
		ca := GetImageHistoryCollector(&repo.Tags[i].Archs)
		ca.Visit(GetImageHistoryURL(repo.Namespace, repo.Name, repo.Tags[i].Name))
	}

	// 爬取结束，做存储工作
	fmt.Println(repo)
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
