package analyzer

import (
	"encoding/json"
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
	"os"
	"path"
	"strings"
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

	// 将image元数据中的layer信息写入临时文件
	metaFilepath := path.Join(myutils.GlobalConfig.TmpDir, fmt.Sprintf("%s-%s-%s-meta.json", ci.namespace, ci.repoName, ci.tagName))
	layerData, err := json.MarshalIndent(ci.metadata.imageMetadata.Layers, "", "    ")
	if err != nil {
		myutils.Logger.Error("json marshal layer metadata of image", ci.name, "failed with:", err.Error())
		return nil, err
	}
	err = os.WriteFile(metaFilepath, layerData, 0644)
	if err != nil {
		myutils.Logger.Error("write layer metadata of image", ci.name, "to file", metaFilepath, "failed with:", err.Error())
		return nil, err
	}
	defer os.Remove(metaFilepath)

	// 调用trufflehog扫描临时文件
	secrets, err := scanSecretsInFilepath(metaFilepath)
	if err != nil {
		myutils.Logger.Error("scanSecretsInFilepath", metaFilepath, "failed with:", err.Error())
		return nil, err
	}
	for _, secret := range secrets {
		secret.Part = myutils.IssuePart.ImageMetadata
		// 定位泄露的隐私位于哪个层命令中
		for index, layer := range ci.metadata.imageMetadata.Layers {
			if strings.Contains(layer.Instruction, secret.Match) {
				secret.Path = fmt.Sprintf("layers[%d].instruction", index)
				break
			}
		}
	}

	res.SecretLeakages = secrets

	return res, nil
}

//func (analyzer *ImageAnalyzer) analyzeImageMetadata(ci *CurrentImage) (*myutils.MetadataResult, error) {
//	res := myutils.NewMetadataResult()
//
//	// 分析隐私泄露
//	// 扫描layers.instruction
//	for index, layer := range ci.metadata.imageMetadata.Layers {
//		is := analyzer.scanSecretsInString(layer.Instruction)
//		for i, _ := range is {
//			is[i].Part = myutils.IssuePart.ImageMetadata
//			is[i].Path = fmt.Sprintf("layers[%d].instruction", index)
//			is[i].LayerDigest = layer.Digest
//		}
//		res.SecretLeakages = append(res.SecretLeakages, is...)
//	}
//
//	return res, nil
//}
