package analyzer

import (
	"myutils"
)

var myNeo4jDriver, _ = myutils.ConfigNewNeo4jDriverWithContext("neo4j://localhost:7687", "neo4j", "qazwsxedc", false)

func AnalyzeImage(name, secretRuleFilePath string) (*myutils.ImageResult, error) {

}
