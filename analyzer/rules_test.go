package analyzer

import (
	"fmt"
	"log"
	"testing"
)

func TestRules_LoadRulesFromYAMLFile(t *testing.T) {
	rs := Rules{}
	err := rs.LoadSecretsFromYAMLFile("../secret_rules.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(rs.Secrets)
}

func TestRules_CompileSecretsRegex(t *testing.T) {
	rs := Rules{}
	err := rs.LoadSecretsFromYAMLFile("../secret_rules.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	rs.CompileSecretsRegex()
	fmt.Println(rs.Secrets)
}
