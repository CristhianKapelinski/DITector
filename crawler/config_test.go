package crawler

import (
	"github.com/stretchr/testify/assert"
	"runtime"
	"strconv"
	"testing"
)

func TestConfig(t *testing.T) {
	Config()
	ass := assert.New(t)
	ass.Equal(runtime.NumCPU(), ConfigCrawler.MaxThread,
		"MaxThread should be ", strconv.Itoa(runtime.NumCPU()), " by default")
	ass.Equal("crawler/proxyaddr.json", ConfigCrawler.ProxyFile,
		"ProxyFile should be crawler/proxyaddr.json by default")
}
