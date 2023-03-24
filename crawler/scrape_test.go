package crawler

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestScrapeRegRepoListRecursive(t *testing.T) {
	asst := assert.New(t)
	// 模拟一个核心调度器，但是不继续分发任务
	go func() {
		pagenum := 0
		for {
			select {
			// 获取到新的keyword，将其传入ScrapeRegRepoListRecursive尝试
			case kw := <-chanKeyword:
				// 每一个核心任务开始前申请一个核心goroutine
				chanLimitMainGoroutine <- struct{}{}
				go func(kw string) {
					defer func() { <-chanLimitMainGoroutine }()
					fmt.Println("CoreScheduler received keyword: ", kw)
					asst.Equal("001", kw, "Next keyword for 0000 should be 0001.")
					//asst.Equal("mo0", kw, "Next keyword for mo0 should be mo.")
				}(kw)
			case rrl := <-chanRegRepoList:
				chanLimitMainGoroutine <- struct{}{}
				pagenum++
				go func(rrl RegisterRepoList__) {
					defer func() { <-chanLimitMainGoroutine }()
					fmt.Println("CoreScheduler received rrl begin with: ", rrl.Summaries[0])
				}(rrl)
			}
		}
	}()
	ScrapeRegRepoListRecursive("000", "community")
	//ScrapeRegRepoListRecursive("mo", "community")

	time.Sleep(time.Second)
}
