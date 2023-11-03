package analyzer

import (
	"myutils"
	"strconv"
)

type ImageAnalyzer struct {
	rules         *ImageAnalyzerRules
	CurrentImage  *CurrentImage
	CurrentResult *myutils.ImageResult
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

	// 配置隐私泄露、敏感参数检测规则
	err = analyzer.loadRules(secretFile, sensParamFile)
	if err != nil {
		return nil, err
	}

	return analyzer, nil
}

func (analyzer *ImageAnalyzer) loadRules(secretFile, sensParamFile string) error {
	// 加载隐私泄露检测规则文件
	if err := analyzer.rules.loadSecretsFromYAMLFile(secretFile); err != nil {
		return err
	}

	// 编译用于隐私泄露检测的正则表达式
	analyzer.rules.compileSecretsRegex()

	// 加载敏感参数规则文件
	if err := analyzer.rules.loadSensitiveParamsFromYAMLFile(sensParamFile); err != nil {
		return err
	}

	return nil
}

// AnalyzeImageByName analyzes image totally by name, including analyzing metadata,
// configuration, content of the image.
//
// Image needs to be stored in the local Docker environment.
func (analyzer *ImageAnalyzer) AnalyzeImageByName(name string) {
	var err error

	analyzer.CurrentImage, err = NewCurrentImage()
	if err != nil {
		myutils.Logger.Error("")
		return
	}
	// 解析镜像信息
	analyzer.CurrentImage.ParseFromDockerEnv()
}

// AnalyzerImagePartialByName analyzes image partially by name, including only the metadata.
//
// This will never pull the layers of the image to local env.
func (analyzer *ImageAnalyzer) AnalyzerImagePartialByName(name string) {

}

// AnalyzeMetadata analyzes metadata of repository, tag and image.
func (analyzer *ImageAnalyzer) AnalyzeMetadata() {

}

// AnalyzeImageMetadata analyze instruction of layers to
func (analyzer *ImageAnalyzer) AnalyzeImageMetadata(image *myutils.Image) ([]*myutils.Issue, error) {
	res := make([]*myutils.Issue, 0)

	for index, layer := range image.Layers {
		digest := ""
		if layer.Size != 0 {
			digest = layer.Digest
		}
		results, err := analyzer.scanSecretsInString(layer.Instruction)
		if err != nil {
			continue
		}
		for _, result := range results {
			result.Type = "in-dockerfile-command"
			result.Path = "layer[" + strconv.Itoa(index) + "].instruction"
			result.LayerDigest = digest
		}
		res = append(res, results...)
	}

	return res, nil
}

func (analyzer *ImageAnalyzer) scanSecretsInString(s string) ([]*myutils.Issue, error) {
	res := make([]*myutils.Issue, 0)

	for _, secret := range analyzer.rules.SecretRules {
		matches := secret.CompiledRegex.FindAllString(s, -1)
		for _, match := range matches {
			tmp := &myutils.Issue{
				Type:          myutils.IssueType.SecretLeakage,
				Rule:          secret,
				Match:         match,
				Severity:      secret.Severity,
				SeverityScore: secret.SeverityScore,
			}
			res = append(res, tmp)
		}
	}

	return res, nil
}
