package crawler

import "github.com/gocolly/colly"

// 实现一些统一的遍历爬取

// ScrapeRegRepoListRecursive 在已经确定source下q=keyword时，匹配条目数count<9500时，
// 递归遍历该关键字的repo结果，拿到全部的repo名。
func ScrapeRegRepoListRecursive(c *colly.Collector, keyword, source string) {
	for _, i := range []string{"1", "2", "3"} {
		if err := c.Visit(GetRegURL(keyword, source, i, "4")); err != nil {
			continue
		}
	}
	close(ChannelRegRepoList)
}

// ScrapeRepoInfo 用于爬取指定repo的metadata和全部tag的信息。
// 考虑进一步将metadata和tag持久化。
func ScrapeRepoInfo(namespace, repo string) {

}

// ScrapeRepoMetadata 用于爬取指定repo的metadata，返回一个。
func ScrapeRepoMetadata(namespace, repo string) {

}

// ScrapeRepoTagsRecursive 递归爬取指定Repo的全部Tag记录。
func ScrapeRepoTagsRecursive(c *colly.Collector, namespace, repo string) {
	for _, i := range []string{"1"} {
		if err := c.Visit(GetRepoTagsURL(namespace, repo, i, "100")); err != nil {
			continue
		}
	}
}
