package buildgraph

import "github.com/NSSL-SJTU/DITector/myutils"

func Build(format string, tagCnt int, threshold int64, ip myutils.IdentityProvider, dataDir string) {
	config(format)
	switch format {
	case "mongo":
		StartFromMongo(tagCnt, threshold, ip, dataDir)
	}
}
