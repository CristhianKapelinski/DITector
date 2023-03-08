package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/gocolly/colly"
)

// GetDockerHubCollector 用于配置一个适用于Docker Hub的colly.Collector父版
func GetDockerHubCollector() *colly.Collector {
	// 创建新的Collector
	c := colly.NewCollector(
		colly.AllowedDomains("hub.docker.com"),
	)

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

// GetRegisterCollector 为爬取指定Register的Repo list的Collector绑定回调函数
func GetRegisterCollector() *colly.Collector {
	c := GetDockerHubCollector()

	// 绑定回调函数
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("FROM RegisterCollector-----------------------Requesting")
		fmt.Println("Visiting: ", r.URL)
		// 查看request时使用的proxy
		fmt.Println("Proxy: ", r.ProxyURL)
		// 查看Cookie，如果有要清除，否则容易封号
		fmt.Println("Cookie: ", r.Headers.Get("Cookie"))
	})

	// 处理JSON
	c.OnResponse(func(r *colly.Response) {
		fmt.Println("FROM RegisterCollector-----------------------Response From ", r.Request.URL)
		fmt.Println("Status Code", r.StatusCode)

		if err := json.Unmarshal([]byte(r.Body), &RegisterRepoList); err != nil {
			fmt.Println("Error Occurred While Doing json.Unmarshal() Response From ", r.Request.URL)
			fmt.Println(err)
		}
		ChannelRegRepoList <- RegisterRepoList
	})

	return c
}
