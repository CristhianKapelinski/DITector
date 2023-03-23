package crawler

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
)

var ConfigCrawler struct {
	MaxThread int    `json:"max_thread"`
	ProxyFile string `json:"proxy_file"`
}

var Proxies struct {
	Addresses []string `json:"proxies"`
}

func init() {
	fb, err := os.ReadFile("crawler/config.json")
	if err != nil {
		fmt.Println("[ERROR] Failed to load crawler/config.json")
	}
	if err := json.Unmarshal(fb, &ConfigCrawler); err != nil {
		fmt.Println("[ERROR] Json failed to unmarshal crawler/config.json with err: ", err)
	}
	// 默认情况下，允许启动的核心goroutine数为系统可调内核数
	if ConfigCrawler.MaxThread == 0 {
		ConfigCrawler.MaxThread = runtime.GOMAXPROCS(runtime.NumCPU())
	}
	// 初始化核心调度器的全局管道
	ChanLimitMainGoroutine = make(chan struct{}, ConfigCrawler.MaxThread)
	ChanRegRepoList = make(chan RegisterRepoList__, ConfigCrawler.MaxThread)
	// 初始化go colly Proxies
	ps, _ := os.ReadFile(ConfigCrawler.ProxyFile)
	if err := json.Unmarshal(ps, &Proxies); err != nil {
		fmt.Println("[ERROR] Json unmarshal failed while parsing proxyaddr file.")
	}

	fmt.Println("Init Crawler Config Success:\n", ConfigCrawler)
}
