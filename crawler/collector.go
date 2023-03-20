package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/gocolly/colly"
	"net/http"
)

// GetDockerHubCollector 用于配置一个适用于Docker Hub的colly.Collector父版
func GetDockerHubCollector() *colly.Collector {
	// 创建新的Collector
	c := colly.NewCollector(
		colly.AllowedDomains("hub.docker.com"),
	)

	// 关keep-alive
	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})

	// 配置Collector
	// 配置代理池
	//if p, err := proxy.RoundRobinProxySwitcher(
	//	"https://127.0.0.1:8080",
	//	"https://127.0.0.1:8081",
	//	"https://127.0.0.1:8082",
	//); err == nil {
	//	c.SetProxyFunc(p)
	//}

	return c
}

// GetRegRepoListCollector 为爬取指定Register的Repo list的Collector绑定回调函数。
// 爬取顺利的情况下，向chRegRepoList通道中传入爬到的RegisterRepoList__结果。
// 测试通过！！！
func GetRegRepoListCollector(chRegRepoList chan RegisterRepoList__) *colly.Collector {
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
		fmt.Println("Status Code", r.StatusCode)

		var RegRepoList RegisterRepoList__
		if err := json.Unmarshal([]byte(r.Body), &RegRepoList); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		chRegRepoList <- RegRepoList
	})

	return c
}

// GetRepoMetadataCollector 为爬取指定Repository的Tag list的Collector绑定回调函数。
// 测试通过！！！
func GetRepoMetadataCollector(Repo Repository__) *colly.Collector {
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

		//var Repo Repository__
		if err := json.Unmarshal([]byte(r.Body), &Repo); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		fmt.Println(Repo)
	})

	return c
}

// GetRepoTagsCollector 为爬取指定Repository的Tag list的Collector绑定回调函数。
// 测试通过！！！
func GetRepoTagsCollector(TagRec *TagReceiver__) *colly.Collector {
	c := GetDockerHubCollector()

	// 爬Tag不需要考虑keep-alive
	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})

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

		//var TagRec TagReceiver__
		if err := json.Unmarshal([]byte(r.Body), &TagRec); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		//fmt.Println(TagRec)
	})

	return c
}

// GetImageHistoryCollector 为爬取指定Namespace/Repository:tag Image的构建命令的Collector绑定回调函数。
// 测试成功！！！
func GetImageHistoryCollector(Arch *[]Arch__) *colly.Collector {
	c := GetDockerHubCollector()

	// 爬Tag不需要考虑keep-alive
	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})

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

		//var TagRec TagReceiver__
		if err := json.Unmarshal([]byte(r.Body), &Arch); err != nil {
			fmt.Println("[ERROR] Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		//fmt.Println(Arch)
	})

	return c
}
