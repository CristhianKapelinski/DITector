package main

import (
	"crawler"
	"fmt"
)

// done 用于标识整个爬虫结束
var done = make(chan struct{})

func main() {
	// 创建Register爬虫，爬repo list
	rc := crawler.GetRegisterCollector()
	// 递归[]string{"00"-"zz"}, 不停尝试直到RepoList.Count < 9500, 只需要制定一个轮换规则, 记录当前状态即可

	// 当找到关键字使RepoList.Count < 9000时，遍历每一页，爬取仓库信息
	go crawler.ScrapeRegRepoListRecursive(rc, "mongo", "community")

	// 处理Repo list，对每个Repo递归找Tag
	for r := range crawler.ChannelRegRepoList {
		fmt.Println(r)
	}

	//fmt.Println(crawler.GetNamespaceURL("aa281916", "1", "4"))
	//fmt.Println(crawler.GetRepoMetaURL("aa281916", "getting-started"))
	//fmt.Println(crawler.GetRepoTagsURL("aa281916", "getting-started", "1", "4"))
	//fmt.Println(crawler.GetImageMetaURL("aa281916", "getting-started", "latest"))
	//fmt.Println(crawler.GetImageHistoryURL("aa281916", "getting-started", "latest"))
	// 访问地址
	//for _, i := range []string{"1"} {
	//	c.Visit(strings.Replace(RegURLTemplate, "{PAGE}", i, 1))
	//}

	done <- struct{}{}
	// 退出程序
	<-done
}
