package analyzer

func NewImageAnalyzer(secretRuleFilePath string) (*ImageAnalyzer, error) {
	imageAnalyzer := new(ImageAnalyzer)

	err := imageAnalyzer.config(false, secretRuleFilePath)
	if err != nil {
		return nil, err
	}
	// 编译用于隐私泄露检测的正则表达式
	imageAnalyzer.rules.CompileSecretsRegex()

	return imageAnalyzer, nil
}

func (imageAnalyzer *ImageAnalyzer) config(initFlag bool, secretRuleFilePath string) error {
	err := imageAnalyzer.rules.LoadSecretsFromYAMLFile(secretRuleFilePath)
	if err != nil {
		return err
	}
	return nil
}
