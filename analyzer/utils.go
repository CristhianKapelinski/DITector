package analyzer

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Rules struct {
	Secrets []ConfigSecret `yaml:"secrets"`
}

type ConfigSecret struct {
	Name  string `yaml:"name"`
	Part  string `yaml:"part"`
	Regex string `yaml:"regex"`
}

func (rs *Rules) LoadRulesFromYAMLFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(content, &rs); err != nil {
		return err
	}

	return nil
}
