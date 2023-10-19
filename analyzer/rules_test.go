package analyzer

import (
	"fmt"
	"log"
	"testing"
)

func TestRules_LoadRulesFromYAMLFile(t *testing.T) {
	rs := Rules{}
	err := rs.loadSecretsFromYAMLFile("../secret_rules.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(rs.SecretRules)
}

func TestRules_CompileSecretsRegex(t *testing.T) {
	rs := Rules{}
	err := rs.loadSecretsFromYAMLFile("../secret_rules.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	rs.compileSecretsRegex()
	fmt.Println(rs.SecretRules)
}
