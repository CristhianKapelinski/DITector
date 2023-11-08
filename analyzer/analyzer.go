package analyzer

import (
	"github.com/Musso12138/dockercrawler/myutils"
)

type ImageAnalyzer struct {
	rules *ImageAnalyzerRules
}

// NewImageAnalyzerGlobalConfig creates a new ImageAnalyzer configured based on config.json
func NewImageAnalyzerGlobalConfig() (*ImageAnalyzer, error) {
	return NewImageAnalyzer(myutils.GlobalConfig.RulesConfig.SecretRulesFile,
		myutils.GlobalConfig.RulesConfig.SensitiveParamRulesFile)
}

// NewImageAnalyzer returns a configured ImageAnalyzer
//
// Parameters:
//
//	secretFile: file path containing rules for matching secrets
//	sensParamFile: file path containing rules for matching sensitive parameters
func NewImageAnalyzer(secretFile, sensParamFile string) (*ImageAnalyzer, error) {
	analyzer := new(ImageAnalyzer)
	var err error

	// 初始化成员变量
	analyzer.rules = newImageAnalyzerRules()

	// 配置隐私泄露规则
	if err = analyzer.rules.loadSecretsFromYAMLFile(secretFile); err != nil {
		return nil, err
	}
	analyzer.rules.compileSecretsRegex()

	// 配置敏感参数规则
	if err = analyzer.rules.loadSensitiveParamsFromYAMLFile(secretFile); err != nil {
		return nil, err
	}

	return analyzer, nil
}

// AnalyzerImagePartialByName analyzes image partially by name, including only the metadata.
//
// This will never pull the layers of the image to local env.
func (analyzer *ImageAnalyzer) AnalyzerImagePartialByName(name string) {

}

// AnalyzeMetadata analyzes metadata of repository, tag and image.
func (analyzer *ImageAnalyzer) AnalyzeMetadata() {

}

func (analyzer *ImageAnalyzer) scanSecretsInString(s string) []myutils.SecretLeakage {
	res := make([]myutils.SecretLeakage, 0)

	for _, secret := range analyzer.rules.SecretRules {
		if secret.CompiledRegex == nil {
			continue
		}
		matches := secret.CompiledRegex.FindAllString(s, -1)
		for _, match := range matches {
			tmp := myutils.SecretLeakage{
				Type:          myutils.IssueType.SecretLeakage,
				Name:          secret.Name,
				Match:         match,
				Description:   secret.Description,
				Severity:      secret.Severity,
				SeverityScore: secret.SeverityScore,
			}
			res = append(res, tmp)
		}
	}

	return res
}

func (analyzer *ImageAnalyzer) scanSecretsInBytes(b []byte) []myutils.SecretLeakage {
	res := make([]myutils.SecretLeakage, 0)

	for _, secret := range analyzer.rules.SecretRules {
		if secret.CompiledRegex == nil {
			continue
		}
		matches := secret.CompiledRegex.FindAll(b, -1)
		for _, match := range matches {
			tmp := myutils.SecretLeakage{
				Type:          myutils.IssueType.SecretLeakage,
				Name:          secret.Name,
				Match:         string(match),
				Description:   secret.Description,
				Severity:      secret.Severity,
				SeverityScore: secret.SeverityScore,
			}
			res = append(res, tmp)
		}
	}

	return res
}

func (analyzer *ImageAnalyzer) scanSensitiveParamInString(s string) []myutils.SensitiveParam {
	res := make([]myutils.SensitiveParam, 0)

	for _, sensitive := range analyzer.rules.SensitiveParamRules {
		matches := sensitive.CompiledRegex.FindAllString(s, -1)
		for _, match := range matches {
			tmp := myutils.SensitiveParam{
				Type:          myutils.IssueType.SensitiveParam,
				Name:          sensitive.Name,
				Match:         match,
				Description:   sensitive.Description,
				Severity:      sensitive.Severity,
				SeverityScore: sensitive.SeverityScore,
			}
			res = append(res, tmp)
		}
	}

	return res
}
