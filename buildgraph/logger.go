package buildgraph

import (
	"os"
	"sync"
	"time"
)

// logger.go 记录build日志

var (
	fileBuilderLogger     *os.File
	lockFileBuilderLogger = sync.Mutex{}
)

func logBuilderString(s string) {
	lockFileBuilderLogger.Lock()
	defer lockFileBuilderLogger.Unlock()
	fileBuilderLogger.WriteString(time.Now().Format(time.DateTime) + " " + s + "\n")
}
