package crawler

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestScrapeRegRepoListRecursive(t *testing.T) {
	//asst := assert.New(t)
	now := time.Now()
	// 模拟一个核心调度器，但是不继续分发任务
	go func() {
		for {
			select {
			// 获取到新的keyword，将其传入ScrapeRegRepoListRecursive尝试
			case kw := <-chanKeyword:
				// 每一个核心任务开始前申请一个核心goroutine
				chanLimitMainGoroutine <- struct{}{}
				go func(kw string) {
					defer func() { <-chanLimitMainGoroutine }()
					fmt.Println("CoreScheduler received keyword: ", kw)
					//asst.Equal("0000000000000001", kw, "Next keyword for -- should be -0.")
					//asst.Equal("mo0", kw, "Next keyword for mo0 should be mo.")
				}(kw)
			case rrl := <-chanRegRepoList:
				chanLimitMainGoroutine <- struct{}{}
				go func(rrl RegisterRepoList__) {
					defer func() { <-chanLimitMainGoroutine }()
					for _, s := range rrl.Summaries {
						ns := strings.Split(s.Name, "/")
						namespace, repository := ns[0], ns[1]
						fmt.Println(namespace, repository)
					}
					//fmt.Println("CoreScheduler received rrl begin with: ", rrl.Summaries[0])
				}(rrl)
			}
		}
	}()
	ScrapeRegRepoListRecursive("xmrig2", "community")
	//ScrapeRegRepoListRecursive("mo", "community")

	fmt.Println("time used: ", time.Since(now))

	time.Sleep(time.Second)
}

func TestScrapeRepoInfo(t *testing.T) {
	// 只有1个tag的基本验证
	//ScrapeRepoInfo("xmrig2021", "r2021")
	// 有更多tags的仓库
	ScrapeRepoInfo("patsissons", "xmrig")
}
