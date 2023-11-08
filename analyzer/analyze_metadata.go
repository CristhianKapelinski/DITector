package analyzer

import (
	"fmt"
	"github.com/Musso12138/dockercrawler/myutils"
)

func (analyzer *ImageAnalyzer) analyzeMetadata(ci *CurrentImage) (*myutils.MetadataResult, error) {
	res := myutils.NewMetadataResult()

	repoMetaRes, err := analyzer.analyzeRepoMetadata(ci)
	if err != nil {
		return nil, err
	}
	res.SensitiveParams = repoMetaRes.SensitiveParams

	imgMetaRes, err := analyzer.analyzeImageMetadata(ci)
	if err != nil {
		return nil, err
	}
	res.SecretLeakages = imgMetaRes.SecretLeakages

	return res, nil
}

func (analyzer *ImageAnalyzer) analyzeRepoMetadata(ci *CurrentImage) (*myutils.MetadataResult, error) {
	res := myutils.NewMetadataResult()

	// 分析敏感参数
	// full_description中推荐的`docker run`
	for _, recCmd := range ci.recommendedCmd {
		is := analyzer.scanSensitiveParamInString(recCmd)
		for i, _ := range is {
			is[i].Part = myutils.IssuePart.RepoMetadata
			is[i].Path = "full_description"
		}
		res.SensitiveParams = append(res.SensitiveParams, is...)
	}

	return res, nil
}

func (analyzer *ImageAnalyzer) analyzeImageMetadata(ci *CurrentImage) (*myutils.MetadataResult, error) {
	res := myutils.NewMetadataResult()

	// 分析隐私泄露
	// 扫描layers.instruction
	for index, layer := range ci.metadata.imageMetadata.Layers {
		is := analyzer.scanSecretsInString(layer.Instruction)
		for i, _ := range is {
			is[i].Part = myutils.IssuePart.ImageMetadata
			is[i].Path = fmt.Sprintf("layers[%d].instruction", index)
			is[i].LayerDigest = layer.Digest
		}
		res.SecretLeakages = append(res.SecretLeakages, is...)
	}

	return res, nil
}
