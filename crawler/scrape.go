package crawler

import (
	"fmt"
	"github.com/gocolly/colly"
	"strconv"
	"strings"
	"sync"
)

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
		errCnt   int8 // 用来计数连续出错误次数
	)
	// 启动一个专门处理chRegRepoList的函数，用于处理第一页结果，转发合法结果
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

	// 第一页爬取主动进行，直到爬取成功
	i := 1
	for err := c.Visit(GetRegURL(keyword, source, strconv.Itoa(i), "100")); err != nil; err = c.Visit(GetRegURL(keyword, source, strconv.Itoa(i), "100")) {
		// keyword不存在对应的任何镜像，直接退出
		if errCnt == 0 {
			if strings.Contains(err.Error(), "Not Found") {
				fmt.Println("[-] No repository for keyword: ", keyword)
				close(ch)
				stopLock.Unlock()
				// 比较特殊，如果传false会无限延长keyword，导致无限404循环
				nxt := GenerateNextKeyword(keyword, true)
				if nxt == "" {
					// 爬虫结束信号
					chanDone <- struct{}{}
				} else {
					chanKeyword <- nxt
				}
				return
			}
		}

		// 代理问题
		errCnt++
		// 重试次数过多就退出
		// TODO: 需要考虑是否需要在次数过多时退出？如果退出下一个keyword应该传什么？
		if errCnt > 11 {
			fmt.Println("[ERROR] Getting first page failed for keyword: ", keyword, ", source: ", source, "\nError: ", err)

			close(ch)
			stopLock.Unlock()
			chanKeyword <- GenerateNextKeyword(keyword, false)
			return
		}
		// 每个ip给3次重试机会，然后就换代理
		if errCnt%3 == 0 {
			c.SetProxy(GetHTTPSProxy())
		}
	}
	errCnt = 0

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
		for err := c.Visit(GetRegURL(keyword, source, strconv.Itoa(i), "100")); err != nil; err = c.Visit(GetRegURL(keyword, source, strconv.Itoa(i), "100")) {
			// 页码不存在
			if errCnt == 0 {
				if strings.Contains(err.Error(), "Not Found") {
					fmt.Println("[-] No repository for keyword: ", keyword, ", page: ", i)
					break
				}
			}

			errCnt++
			if errCnt > 11 {
				fmt.Println("[ERROR] Getting page: ", i, " failed for keyword: ", keyword, ", source: ", source, "\nError: ", err)
				break
			}
			if errCnt%3 == 0 {
				c.SetProxy(GetHTTPSProxy())
			}
		}
		errCnt = 0
	}

	close(ch)

	// 一定是函数主体都处理好才向ChanKeyword中传数据，因为ChanKeyword是无缓冲通道，在核心调度器会阻塞。
	nxt := GenerateNextKeyword(keyword, true)
	if nxt == "" {
		// DockerCrawler结束信号
		chanDone <- struct{}{}
	} else {
		// 把当前keyword保存到进度数据库中
		_, err := dockerDB.InsertKeyword(keyword)
		if err != nil {
			fmt.Printf("[ERROR] Insert keyword '%s' into keywords failed with: %s\n", keyword, err)
		} else {
			fmt.Printf("[+] Insert keyword '%s' success.\n", keyword)
		}
		chanKeyword <- nxt
	}
}

// ScrapeRepoInfo 用于爬取仓库namespace/repository的metadata，全部tag，以及每个tag对应镜像的history信息。
// 考虑根据这里的结果进一步将metadata和tag信息持久化。
func ScrapeRepoInfo(namespace, repository string) {

	var (
		errCnt int8
		repo   Repository__
	)

	// 爬取Metadata
	cm := GetRepoMetadataCollector(&repo)
	for err := cm.Visit(GetRepoMetaURL(namespace, repository)); err != nil; err = cm.Visit(GetRepoMetaURL(namespace, repository)) {
		if errCnt == 0 {
			if strings.Contains(err.Error(), "Not Found") {
				fmt.Println("[-] Not Found repository: ", namespace, "/", repository)
				return
			}
		}

		errCnt++
		if errCnt > 11 {
			fmt.Println("[ERROR] Getting metadata failed for repository: ", namespace, "/", repository, "\nError: ", err)
			return
		}
		if errCnt%3 == 0 {
			cm.SetProxy(GetHTTPSProxy())
		}
	}
	errCnt = 0

	// 存储Metadata
	res, err := StoreRepository__(&repo)
	if err != nil {
		fmt.Println("[ERROR] Insert into repository failed with: ", err)
		return
	}
	// TODO: 后面要添加update数据库需要对修改
	if i, _ := res.RowsAffected(); i == 0 {
		fmt.Printf("[WARN] Repository '%s' already exists.", namespace+"/"+repository)
		//return
	} else {
		fmt.Printf("[INFO] Insert repository '%s' success.\n", namespace+"/"+repository)
	}

	// 爬所有Tags，可能涉及多页，建管道维护
	chTags := make(chan TagReceiver__)
	ct := GetRepoTagsCollector(chTags)
	var (
		cur      = 1
		pages    int
		pageLock sync.Mutex
	)

	pageLock.Lock()
	// 负责计算总页数与数据存储
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
				// 存储tags
				for i, _ := range tags.Results {
					res, err = StoreTag__(namespace, repository, &tags.Results[i])
					if err != nil {
						fmt.Println("[ERROR] Insert into tags failed with: ", err)
					}
					if j, _ := res.RowsAffected(); j == 0 {
						fmt.Printf("[WARN] Tag '%s' for repository '%s' already exist.\n", tags.Results[i].Name, namespace+"/"+repository)
					} else {
						fmt.Printf("[INFO] Insert tag '%s' for repository '%s' success.\n", tags.Results[i].Name, namespace+"/"+repository)
					}
				}
			} else {
				repo.Tags = append(repo.Tags, tags.Results...)
				// 存储tags
				for i, _ := range tags.Results {
					res, err = StoreTag__(namespace, repository, &tags.Results[i])
					if err != nil {
						fmt.Println("[ERROR] Insert into tags failed with: ", err)
					}
					if j, _ := res.RowsAffected(); j == 0 {
						fmt.Printf("[WARN] Tag '%s' for repository '%s' already exist.\n", tags.Results[i].Name, namespace+"/"+repository)
					} else {
						fmt.Printf("[INFO] Insert tag '%s' for repository '%s' success.\n", tags.Results[i].Name, namespace+"/"+repository)
					}
				}
			}
		}
	}(chTags)
	// 访问第一页
	for err = ct.Visit(GetRepoTagsURL(namespace, repository, strconv.Itoa(cur), "100")); err != nil; err = ct.Visit(GetRepoTagsURL(namespace, repository, strconv.Itoa(cur), "100")) {
		if errCnt == 0 {
			if strings.Contains(err.Error(), "Not Found") {
				fmt.Println("[-] Not Found tag list for repository: ", namespace, "/", repository, ", page: ", cur)
				return
			}
		}

		errCnt++
		if errCnt > 11 {
			fmt.Println("[ERROR] Getting tag list failed for repository: ", namespace, "/", repository, ", page: ", cur, "\nError: ", err)
			return
		}
		if errCnt%3 == 0 {
			cm.SetProxy(GetHTTPSProxy())
		}
	}
	errCnt = 0

	// 尝试获取锁，获取的一瞬间就可以关闭锁，做一次pages的同步
	pageLock.Lock()
	pageLock.Unlock()

	// 获取全部tags
	for cur = 2; cur <= pages; cur++ {
		for err = ct.Visit(GetRepoTagsURL(namespace, repository, strconv.Itoa(cur), "100")); err != nil; err = ct.Visit(GetRepoTagsURL(namespace, repository, strconv.Itoa(cur), "100")) {
			if errCnt == 0 {
				if strings.Contains(err.Error(), "Not Found") {
					fmt.Println("[-] Not Found tag list for repository: ", namespace, "/", repository, ", page: ", cur)
					break
				}
			}

			errCnt++
			if errCnt > 11 {
				fmt.Println("[ERROR] Getting tag list failed for repository: ", namespace, "/", repository, ", page: ", cur, "\nError: ", err)
				break
			}
			if errCnt%3 == 0 {
				cm.SetProxy(GetHTTPSProxy())
			}
		}
		errCnt = 0
	}
	// 获取后关闭通道
	close(chTags)

	// 爬每个Tag的所有Arch History
	// 随机延时可以在某一刻忽然将Rate-Limit补满，对单IP有效
	// Test显示71个tag用时142s
	for i, _ := range repo.Tags {
		//// 引入随机延时，防止快速达到限制
		//rd := time.Duration(rand.Int63n(int64(5 * time.Second)))
		//time.Sleep(rd)
		ca := GetImageHistoryCollector(&repo.Tags[i].Archs)
		for err = ca.Visit(GetImageHistoryURL(repo.Namespace, repo.Name, repo.Tags[i].Name)); err != nil; err = ca.Visit(GetImageHistoryURL(repo.Namespace, repo.Name, repo.Tags[i].Name)) {
			if errCnt == 0 {
				if strings.Contains(err.Error(), "Not Found") {
					fmt.Println("[-] Not Found tag list for repository: ", namespace, "/", repository, ", page: ", cur)
					break
				}
			}

			errCnt++
			if errCnt > 11 {
				fmt.Println("[ERROR] Getting tag list failed for repository: ", namespace, "/", repository, ", page: ", cur, "\nError: ", err)
				break
			}
			if errCnt%3 == 0 {
				cm.SetProxy(GetHTTPSProxy())
			}
		}
		// 爬取成功则将每个arch的image信息存储
		if errCnt < 12 {
			// 存储tag下每个arch的image信息
			for j, _ := range repo.Tags[i].Archs {
				res, errS := StoreArch__(namespace, repository, repo.Tags[i].Name, &repo.Tags[i].Archs[j])
				if errS != nil {
					fmt.Println("[ERROR] Insert into images failed with: ", errS)
				}
				if k, _ := res.RowsAffected(); k == 0 {
					fmt.Printf("[WARN] Image '%s' already exist, digest: %s\n",
						namespace+"/"+repository+":"+repo.Tags[i].Name, repo.Tags[i].Archs[j].Digest)
				} else {
					fmt.Printf("[INFO] Insert image '%s' success, digest: %s\n",
						namespace+"/"+repository+":"+repo.Tags[i].Name, repo.Tags[i].Archs[j].Digest)
				}
			}
		}

		errCnt = 0
	}
}

// ScrapeRepoMetadata
// TODO: 用于爬取指定repo的metadata，返回一个。
func ScrapeRepoMetadata(namespace, repo string) {

}

// ScrapeRepoTagsRecursive
// TODO: 递归爬取指定Repo的全部Tag记录。
func ScrapeRepoTagsRecursive(c *colly.Collector, namespace, repo string) {

}
