package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/gocolly/colly"
	"math/rand"
	"net/http"
	"time"
)

// GetDockerHubCollector 用于配置一个适用于Docker Hub的colly.Collector父版
func GetDockerHubCollector() *colly.Collector {
	// 创建新的Collector
	c := colly.NewCollector(
		//colly.AllowedDomains("hub.docker.com"),
		colly.UserAgent(UserAgents[rand.Intn(len(UserAgents))]),
		colly.AllowURLRevisit(), // 因为代理网络问题，经常需要重复爬取页面
	)

	// 配置Collector
	// 关keep-alive
	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})

	return c
}

// GetDockerHubAsyncCollector 返回进行了Async配置的DockerHubCollector
func GetDockerHubAsyncCollector() *colly.Collector {
	c := GetDockerHubCollector()

	// 配置异步
	c.Async = true

	// 配置并发
	// 设置HTTP请求间随机延时
	// 设置colly Collector线程数
	c.Limit(&colly.LimitRule{
		// 必须配置DomainGlob与DomainRegexp之一，否则limit不生效
		DomainGlob:  "hub.docker.com",
		RandomDelay: 2 * time.Second,
		Parallelism: 2,
	})

	return c
}

// GetRegRepoListCollector 为爬取指定Register的Repo list的Collector绑定回调函数。
// 爬取顺利的情况下，向chRegRepoList通道中传入爬到的RegisterRepoList__结果。
func GetRegRepoListCollector(ch chan RegisterRepoList__) *colly.Collector {
	c := GetDockerHubCollector()

	// 绑定回调函数
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("FROM RegRepoListCollector-----------------------Request")
		fmt.Println("Visiting: ", r.URL)
		// 查看request时使用的proxy
		fmt.Println("Proxy: ", r.ProxyURL)
		// 查看Cookie，如果有要清除，否则容易封号
		fmt.Println("Cookie: ", r.Headers.Get("Cookie"))
	})

	// 处理JSON
	c.OnResponse(func(r *colly.Response) {
		fmt.Println("FROM RegRepoListCollector-----------------------Response")
		fmt.Println("From: ", r.Request.URL)
		fmt.Println("Proxy: ", r.Request.ProxyURL)
		fmt.Println("Status Code ", r.StatusCode)
		fmt.Println("X-Ratelimit-Remaining: ", r.Headers.Get("X-Ratelimit-Remaining"))

		var RegRepoList RegisterRepoList__
		if err := json.Unmarshal([]byte(r.Body), &RegRepoList); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		ch <- RegRepoList
	})

	return c
}

// GetRepoMetadataCollector 为爬取指定Repository的Tag list的Collector绑定回调函数。
func GetRepoMetadataCollector(repo *Repository__) *colly.Collector {
	c := GetDockerHubCollector()

	// 绑定回调函数
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("FROM RepoMetadataCollector-----------------------Request")
		fmt.Println("Visiting: ", r.URL)
		// 查看request时使用的proxy
		fmt.Println("Proxy: ", r.ProxyURL)
		// 查看Cookie，如果有要清除，否则容易封号
		fmt.Println("Cookie: ", r.Headers.Get("Cookie"))
	})

	// 处理JSON
	c.OnResponse(func(r *colly.Response) {
		fmt.Println("FROM RepoMetadataCollector-----------------------Response")
		fmt.Println("From: ", r.Request.URL)
		fmt.Println("Status Code", r.StatusCode)
		fmt.Println("X-Ratelimit-Remaining: ", r.Headers.Get("X-Ratelimit-Remaining"))

		//var Repo Repository__
		if err := json.Unmarshal([]byte(r.Body), &repo); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
	})

	return c
}

// GetRepoTagsCollector 为爬取指定Repository的Tag list的Collector绑定回调函数。
func GetRepoTagsCollector(ch chan TagReceiver__) *colly.Collector {
	// Tags有很强的时间顺序需求，不用Async Collector
	c := GetDockerHubCollector()

	// 绑定回调函数
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("FROM RepoTagsCollector-----------------------Request")
		fmt.Println("Visiting: ", r.URL)
		// 查看request时使用的proxy
		fmt.Println("Proxy: ", r.ProxyURL)
		// 查看Cookie，如果有要清除，否则容易封号
		fmt.Println("Cookie: ", r.Headers.Get("Cookie"))
	})

	// 处理JSON
	c.OnResponse(func(r *colly.Response) {
		fmt.Println("FROM RepoTagsCollector-----------------------Response")
		fmt.Println("From: ", r.Request.URL)
		fmt.Println("Status Code", r.StatusCode)
		fmt.Println("X-Ratelimit-Remaining: ", r.Headers.Get("X-Ratelimit-Remaining"))

		var TagRec TagReceiver__
		if err := json.Unmarshal([]byte(r.Body), &TagRec); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		ch <- TagRec
		//fmt.Println(TagRec)
	})

	return c
}

// GetImageHistoryCollector 为爬取指定Namespace/Repository:tag Image的构建命令的Collector绑定回调函数。
func GetImageHistoryCollector(Arch *[]Arch__) *colly.Collector {
	c := GetDockerHubCollector()

	// 绑定回调函数
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("FROM ImageHistoryCollector-----------------------Request")
		fmt.Println("Visiting: ", r.URL)
		// 查看request时使用的proxy
		fmt.Println("Proxy: ", r.ProxyURL)
		// 查看Cookie，如果有要清除，否则容易封号
		fmt.Println("Cookie: ", r.Headers.Get("Cookie"))
	})

	// 处理JSON
	c.OnResponse(func(r *colly.Response) {
		fmt.Println("FROM ImageHistoryCollector-----------------------Response")
		fmt.Println("From: ", r.Request.URL)
		fmt.Println("Status Code", r.StatusCode)
		fmt.Println("X-Ratelimit-Remaining: ", r.Headers.Get("X-Ratelimit-Remaining"))

		//var TagRec TagReceiver__
		if err := json.Unmarshal([]byte(r.Body), &Arch); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		//fmt.Println(Arch)
	})

	return c
}
