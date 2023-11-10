package analyzer

import (
	"github.com/Musso12138/dockercrawler/myutils"
	"os"
)

func FileNeedScanSecrets(filepath string) bool {
	return false
}

func (analyzer *ImageAnalyzer) scanSecretsInFile(filepath string) ([]*myutils.SecretLeakage, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	return analyzer.scanSecretsInBytes(content), nil
}

func (analyzer *ImageAnalyzer) scanSecretsInString(s string) []*myutils.SecretLeakage {
	res := make([]*myutils.SecretLeakage, 0)

	for _, secret := range analyzer.rules.SecretRules {
		if secret.CompiledRegex == nil {
			continue
		}
		matches := secret.CompiledRegex.FindAllString(s, -1)
		for _, match := range matches {
			tmp := &myutils.SecretLeakage{
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

func (analyzer *ImageAnalyzer) scanSecretsInBytes(b []byte) []*myutils.SecretLeakage {
	res := make([]*myutils.SecretLeakage, 0)

	for _, secret := range analyzer.rules.SecretRules {
		if secret.CompiledRegex == nil {
			continue
		}
		matches := secret.CompiledRegex.FindAll(b, -1)
		for _, match := range matches {
			tmp := &myutils.SecretLeakage{
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
