package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/gocolly/colly"
	"myutils"
	"net/http"
	"strconv"
	"testing"
)

// TestSetProxy 测一下proxy配置能否生效
func TestSetProxy(t *testing.T) {
	c := GetDockerHubCollector()

	//if p, err := proxy.RoundRobinProxySwitcher(
	//	Proxies.Addresses...,
	//); err != nil {
	//	fmt.Println("[ERROR] collector SetProxy Failed with: ", err)
	//} else {
	//	c.SetProxyFunc(p)
	//}
	//if err := c.SetProxy("http://36.6.145.210:8089"); err != nil {
	//	fmt.Println("[ERROR] Set proxy failed with: ", err)
	//}
	fmt.Println("Set proxy success.")

	//c.SetRequestTimeout(time.Second * 20)

	// 绑定回调函数
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("FROM TestSetProxy-----------------------Request")
		fmt.Println("Visiting: ", r.URL)
		// 查看request时使用的proxy
		fmt.Println("Proxy: ", r.ProxyURL)
		fmt.Println("UserAgent: ", r.Headers.Get("User-Agent"))
		// 查看Cookie，如果有要清除，否则容易封号
		fmt.Println("Cookie: ", r.Headers.Get("Cookie"))
	})

	// 处理JSON
	c.OnResponse(func(r *colly.Response) {
		fmt.Println("FROM TestSetProxy-----------------------Response")
		fmt.Println("From: ", r.Request.URL)
		fmt.Println("Proxy: ", r.Request.ProxyURL)
		fmt.Println("Status Code: ", r.StatusCode)
		if r.StatusCode != http.StatusOK {
			fmt.Println("[-] status not ok: ", r.StatusCode)
		} else {
			fmt.Println("[+] status ok")
		}
		fmt.Printf("X-Ratelimit-Remaining: %s, type: %T\n",
			r.Headers.Get("X-Ratelimit-Remaining"),
			r.Headers.Get("X-Ratelimit-Remaining"))

		var tagr TagReceiver__
		json.Unmarshal(r.Body, &tagr)
		fmt.Println("Tags count: ", tagr.Count)
	})

	testurl := myutils.GetRepoTagsURL("library", "mongo", "1", "4")

	fmt.Println(testurl)

	for i := 1; i < 10; i++ {
		for err := c.Visit(myutils.GetRepoTagsURL("library", "mongo", strconv.Itoa(i), "4")); err != nil; err = c.Visit(myutils.GetRepoTagsURL("library", "mongo", strconv.Itoa(i), "4")) {
			fmt.Println("[ERROR] Colly visit failed with: ", err)
		}
	}
}
