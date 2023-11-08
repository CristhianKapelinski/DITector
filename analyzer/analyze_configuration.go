package analyzer

import (
	"fmt"
	"github.com/Musso12138/dockercrawler/myutils"
	"strings"
)

func (analyzer *ImageAnalyzer) analyzeConfiguration(ci *CurrentImage) (*myutils.ConfigurationResult, error) {
	res := myutils.NewConfigurationResult()

	envSecrets := analyzer.analyzeEnvConfig(ci)
	res.SecretLeakages = envSecrets

	return res, nil
}

func (analyzer *ImageAnalyzer) analyzeEnvConfig(ci *CurrentImage) []myutils.SecretLeakage {
	res := make([]myutils.SecretLeakage, 0)

	// 分析隐私泄露
	// 扫描镜像环境变量
	for _, env := range ci.configuration.Config.Env {
		is := analyzer.scanSecretsInString(env)
		for i, _ := range is {
			is[i].Part = myutils.IssuePart.Configuration
			is[i].Path = fmt.Sprintf("Env[%s]", strings.Split(env, "=")[0])
		}

		res = append(res, is...)
	}

	return res
}
