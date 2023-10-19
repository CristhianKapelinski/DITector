package analyzer

import (
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
)

type Rules struct {
	SecretRules []*SecretRule `yaml:"secrets"`
}

type SecretRule struct {
	Name          string         `yaml:"name" json:"name"`
	Regex         string         `yaml:"regex" json:"regex"`
	RegexType     string         `yaml:"regex_type"`
	CompiledRegex *regexp.Regexp `yaml:"-" json:"-"`
	Severity      string         `yaml:"severity"`
	SeverityScore float64        `yaml:"severity_score"`
}

func (rs *Rules) loadSecretsFromYAMLFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(content, &rs); err != nil {
		return err
	}

	return nil
}

func (rs *Rules) compileSecretsRegex() {
	for _, secret := range rs.SecretRules {
		secret.CompiledRegex, _ = regexp.Compile(secret.Regex)
	}
}
