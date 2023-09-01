package analyzer

import (
	"fmt"
	"log"
	"testing"
)

func TestRules_LoadRulesFromYAMLFile(t *testing.T) {
	rs := Rules{}
	err := rs.LoadRulesFromYAMLFile("../rules.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(rs.Secrets)
}
