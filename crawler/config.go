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

var UserAgents = [...]string{
	// chrome
	`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36`,
	`Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36`,
	`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36`,
	`Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36`,
	`Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`,
	`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36`,
	`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`,
	// edge
	`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36 Edg/111.0.1661.44`,
	// firefox
	`Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/111.0`,
	`Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/111.0`,
	// safari
	`Mozilla/5.0 (Windows; U; Windows NT 5.1; zh-CN) AppleWebKit/533.21.1 (KHTML, like Gecko) Version/5.0.5 Safari/533.21.1`,
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
	if err = json.Unmarshal(fb, &ConfigCrawler); err != nil {
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
		if err = json.Unmarshal(ps, &Proxies); err != nil {
			fmt.Println("[ERROR] Json unmarshal failed while parsing proxyaddr file: ", ConfigCrawler.ProxyFile)
		} else {
			fmt.Println("[+] Init Proxies From Local Success: ", Proxies)
		}
	} else {
		UpdateProxies()
		fmt.Println("[+] Init Proxies From Remote Success: ", Proxies)
	}

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
