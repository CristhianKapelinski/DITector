package analyzer

func (imageAnalyzer *ImageAnalyzer) config(initFlag bool, ruleFilePath string) error {
	err := imageAnalyzer.rules.LoadRulesFromYAMLFile(ruleFilePath)
	if err != nil {
		return err
	}
	return nil
}

func NewImageAnalyzer(ruleFilePath string) (*ImageAnalyzer, error) {
	imageAnalyzer := new(ImageAnalyzer)

	err := imageAnalyzer.config(false, ruleFilePath)
	if err != nil {
		return nil, err
	}
	imageAnalyzer.rules.CompileSecretsRegex()

	return imageAnalyzer, nil
}
