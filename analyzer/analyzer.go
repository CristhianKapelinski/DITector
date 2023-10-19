package analyzer

import (
	"github.com/docker/docker/client"
	"myutils"
	"strconv"
)

type ImageAnalyzer struct {
	DockerClient *client.Client
	rules        Rules
	CurrentImage
}

type CurrentImage struct {
	Registry           string              `json:"registry"`
	Namespace          string              `json:"namespace"`
	RepositoryName     string              `json:"repository_name"`
	RepositoryMetadata *myutils.Repository `json:"repository_metadata"`
	TagName            string              `json:"tag_name"`
	TagMetadata        *myutils.Tag        `json:"tag_metadata"`
	ImageMetadata      *myutils.Image      `json:"image_metadata"`
	Digest             string              `json:"digest"`
	LayerLocalFileMap  map[string]string   `json:"layer_local_file_map"`
}

func NewImageAnalyzer(secretRuleFilePath string) (*ImageAnalyzer, error) {
	analyzer := new(ImageAnalyzer)
	var err error

	// 连接本地Docker环境
	analyzer.DockerClient, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	// 配置隐私泄露、敏感参数检测规则
	err = analyzer.loadRules(false, secretRuleFilePath)
	if err != nil {
		return nil, err
	}

	return analyzer, nil
}

func (imageAnalyzer *ImageAnalyzer) loadRules(initFlag bool, secretRuleFilePath string) error {
	// 加载隐私泄露检测规则文件
	err := imageAnalyzer.rules.loadSecretsFromYAMLFile(secretRuleFilePath)
	if err != nil {
		return err
	}

	// 编译用于隐私泄露检测的正则表达式
	imageAnalyzer.rules.compileSecretsRegex()

	return nil
}

// AnalyzeImageByName
func (imageAnalyzer *ImageAnalyzer) AnalyzeImageByName(name string) {

}

// AnalyzeImageMetadata analyze instruction of layers to
func (imageAnalyzer *ImageAnalyzer) AnalyzeImageMetadata(image *myutils.ImageOld) ([]*myutils.Result, error) {
	res := make([]*myutils.Result, 0)

	for index, layer := range image.Layers {
		digest := ""
		if layer.Size != 0 {
			digest = layer.Digest
		}
		results, err := imageAnalyzer.scanSecretsInString(layer.Instruction, "contents")
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

func (imageAnalyzer *ImageAnalyzer) scanSecretsInString(s, part string) ([]*myutils.Result, error) {
	res := make([]*myutils.Result, 0)

	for _, secret := range imageAnalyzer.rules.SecretRules {
		// diff parts like contents, extension, filename, and ...
		if secret.Part == part {
			matches := secret.CompiledRegex.FindAllString(s, -1)
			for _, match := range matches {
				tmp := &myutils.Result{
					RuleName:      secret.Name,
					Part:          secret.Part,
					Match:         match,
					Severity:      secret.Severity,
					SeverityScore: secret.SeverityScore,
				}
				res = append(res, tmp)
			}
		}
	}

	return res, nil
}
