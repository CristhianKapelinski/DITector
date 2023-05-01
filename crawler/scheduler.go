package crawler

import (
	"fmt"
	"runtime"
	"strings"
)

// 负责整个爬虫的核心调度，启动goroutine等。

// 初始化一系列限制通道，通道初始化在config.go init()中实现
// 用于限制goroutine数量

var (
	// chanLimitMainGoroutine 限制核心调度器goroutine数量
	chanLimitMainGoroutine chan struct{}
)

// 以下部分为proxy稳定状态下可用的
// 定义一系列用于稳定proxy下任务调度的关键字

var (
	// chanKeyword 用于传递keyword，优先爬完有关一个keyword的所有镜像，再进入下一个keyword。
	chanKeyword = make(chan string)
	// chanRegRepoList 用于传递拿到的仓库信封，方便核心调度器解开信封处理每个信封内容
	chanRegRepoList chan RegisterRepoList__
	chanRegLimit    = make(chan struct{}, runtime.NumCPU())
	chanRegName     = make(chan string, runtime.NumCPU())
	// chanDone 用于传递DockerCrawler结束信号，即keyword为""时
	chanDone = make(chan struct{})
)

// StartRecursive 是Proxy稳定状态下整个DockerCrawler的入口函数
func StartRecursive() {
	// 启动代理监视器
	go KDLProxiesMaintainer()
	fmt.Println("[+] KDLProxiesMaintainer startup")
	// 启动核心调度器
	go CoreScheduler()
	fmt.Println("[+] CoreScheduler startup")
	// 启动RepoInfo爬取调度器
	go RepoScheduler()
	fmt.Println("[+] RepoScheduler startup")
	// 传入初始Keyword，启动整个爬取过程
	cur, err := dockerDB.GetLastKeyword()
	if err != nil && strings.Contains(err.Error(), "no rows in result set") {
		cur = "--"
	} else {
		// 有时候崩溃导致当前cur下很多repository的tag和image没爬完，暂时不自动使用下一个了
		//cur = GenerateNextKeyword(cur, true)
	}
	fmt.Println("[+] Current keyword: ", cur)
	chanKeyword <- cur
	<-chanDone
}

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
func CoreScheduler() {
	for {
		select {
		// 获取到新的keyword，将其传入ScrapeRegRepoListRecursive尝试
		case kw := <-chanKeyword:
			// 每一个核心任务开始前申请一个核心goroutine
			chanLimitMainGoroutine <- struct{}{}
			go func(kw string) {
				defer func() { <-chanLimitMainGoroutine }()
				ScrapeRegRepoListRecursive(kw, "community")
			}(kw)
		case rrl := <-chanRegRepoList:
			chanLimitMainGoroutine <- struct{}{}
			go func(rrl RegisterRepoList__) {
				defer func() { <-chanLimitMainGoroutine }()
				for _, s := range rrl.Summaries {
					//ns := strings.Split(s.Name, "/")
					//namespace, repository := ns[0], ns[1]
					//ScrapeRepoInfo(namespace, repository)
					chanRegName <- s.Name
				}
			}(rrl)
			//case <-chanDone:
			//	fmt.Println("[+] All Done")
			//	fmt.Println("[+] DockerCrawler Exit")
			//	return
		}
	}
}

// RepoScheduler 专用于处理repository，调用ScrapeRepoInfo，有的时候页数有限，没能充分发挥多线程的魅力
func RepoScheduler() {
	for {
		select {
		case s := <-chanRegName:
			chanRegLimit <- struct{}{}
			go func(rname string) {
				defer func() { <-chanRegLimit }()
				ns := strings.Split(rname, "/")
				if len(ns) != 2 {
					return
				}
				namespace, repository := ns[0], ns[1]
				ScrapeRepoInfo(namespace, repository)
			}(s)
		}
	}
}
