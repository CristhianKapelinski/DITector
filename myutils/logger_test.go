package myutils

import (
	"fmt"
	"testing"
)

func TestLogDockerCrawlerString(t *testing.T) {
	Logger.Error("this is error")
	Logger.Warn("this is warn")
	Logger.Info("this is info")
	Logger.Debug("this is debug")
}

func TestGetLocalNowTime(t *testing.T) {
	fmt.Println(GetLocalNowTimeStr())
}
