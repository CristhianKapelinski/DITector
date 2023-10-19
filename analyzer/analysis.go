package analyzer

import (
	"myutils"
)

var myNeo4jDriver, _ = myutils.NewNeo4jDriver("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)

// AnalyzeImage 彻底分析镜像：描述、配置、内容
func AnalyzeImage(name, secretRuleFilePath string) (*myutils.ImageResult, error) {
	var err error
	res := new(myutils.ImageResult)

	return res, err
}
