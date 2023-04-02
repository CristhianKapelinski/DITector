package crawler

import (
	"db"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
)

var ConfigCrawler struct {
	MaxThread  int    `json:"max_thread"`
	LocalProxy bool   `json:"local_proxy"`
	ProxyFile  string `json:"proxy_file"`
}

var Proxies struct {
	Addresses []string `json:"proxies"`
	Banned    []string
}

var dockerDB *db.DockerDB

func init() {
	// 获取程序根目录
	_, filename, _, _ := runtime.Caller(0)
	root := path.Dir(path.Dir(filename))
	configFile := root + "/config.json"
	// 加载DockerCrawler Config
	fb, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println("[ERROR] Failed to load ", configFile)
	}
	if err := json.Unmarshal(fb, &ConfigCrawler); err != nil {
		fmt.Printf("[ERROR] Json failed to unmarshal %s with err: %v\n", configFile, err)
	}
	// 默认情况下，允许启动的核心goroutine数为系统可调内核数
	if ConfigCrawler.MaxThread <= 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
		// runtime.GOMAXPROCS 返回的是设置成功之前的GOMAXPROCS，所以要再设一次获取上一次获取成功的数
		ConfigCrawler.MaxThread = runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(ConfigCrawler.MaxThread)
	}

	fmt.Println("Init Crawler Config Success: ", ConfigCrawler)

	// 初始化核心调度器的全局管道
	chanLimitMainGoroutine = make(chan struct{}, ConfigCrawler.MaxThread)
	chanRegRepoList = make(chan RegisterRepoList__, ConfigCrawler.MaxThread)

	// 初始化go colly Proxies
	if ConfigCrawler.LocalProxy {
		// 获取proxy文件位置
		proxyFile := root + "/" + ConfigCrawler.ProxyFile
		ps, _ := os.ReadFile(proxyFile)
		if err := json.Unmarshal(ps, &Proxies); err != nil {
			fmt.Println("[ERROR] Json unmarshal failed while parsing proxyaddr file: ", ConfigCrawler.ProxyFile)
		}
	} else {
		UpdateProxiesFrom("")
	}
	fmt.Println("Init Proxies Success: ", Proxies)

	// 初始化数据库连接
	dockerDB, err = db.NewDockerDB("docker:docker@/dockerhub")
	if err != nil {
		log.Fatalln("[ERROR] Open mysql database failed with: ", err)
	}
	err = dockerDB.Ping()
	if err != nil {
		log.Fatalln("[ERROR] Ping mysql database failed with: ", err)
	}
}

func UpdateProxiesFrom(url string) {
	Proxies.Addresses = []string{
		"https://117.50.175.76:1081",
		"https://112.124.38.70:3128",
	}
}
