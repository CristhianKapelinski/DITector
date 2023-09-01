package analyzer

import (
	"fmt"
	"log"
	"myutils"
)

func config(initFlag bool, ruleFilePath string) {
	err := rules.LoadRulesFromYAMLFile(ruleFilePath)
	if err != nil {
		myutils.LogDockerCrawlerString(myutils.LogLevel.Error, "load rules from yaml file failed with:", err.Error())
		log.Fatalln("[ERROR] load rules from yaml file failed with:", err)
	}

	myMongo, err = myutils.ConfigMongoClient(initFlag)
	if err != nil {
		log.Fatalln("[ERROR] connect to and config MongoDB failed with err: ", err)
	}
	fmt.Println("[+] Connect to MongoDB succeed")

	myNeo4jDriver, err = myutils.ConfigNewNeo4jDriverWithContext("neo4j://localhost:7687", "neo4j", "qazwsxedc")
	if err != nil {
		log.Fatalln("[ERROR] Connect to neo4j failed with:", err)
	}
	fmt.Println("[+] Connect to Neo4j succeed")
}
