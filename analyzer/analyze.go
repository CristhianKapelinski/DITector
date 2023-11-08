package analyzer

import (
	"fmt"
	"github.com/Musso12138/dockercrawler/myutils"
	"os"
	"time"
)

var imageAnalyzer, imageAnalyzerE = NewImageAnalyzerGlobalConfig()

// AnalyzeImageByName analyzes image totally, including metadata, configuration, content of the image.
func AnalyzeImageByName(name string) (*myutils.ImageResult, error) {
	if imageAnalyzerE != nil {
		return nil, fmt.Errorf("create ImageAnalyzer failed with: %s", imageAnalyzerE)
	}

	return imageAnalyzer.AnalyzeImageByName(name)
}

// AnalyzeImagePartialByName analyzes image partially, currently only metadata.
func AnalyzeImagePartialByName(name string) (*myutils.ImageResult, error) {
	if imageAnalyzerE != nil {
		return nil, fmt.Errorf("create ImageAnalyzer failed with: %s", imageAnalyzerE)
	}

	var err error
	res := new(myutils.ImageResult)

	return res, err
}

// AnalyzeImageByName analyzes image totally by name, including analyzing metadata,
// configuration, content of the image.
//
// Image needs to be stored in the local Docker environment.
func (analyzer *ImageAnalyzer) AnalyzeImageByName(name string) (*myutils.ImageResult, error) {
	beginTime := time.Now()
	beginTimeStr := myutils.GetLocalNowTime()

	// 解析镜像信息
	ci, err := NewCurrentImage(name)
	if err != nil {
		myutils.Logger.Error("create CurrentImage for image", name, "failed with:", err.Error())
		return nil, err
	}
	if err = ci.ParseFromFile(); err != nil {
		myutils.Logger.Error("parse image", name, "failed with:", err.Error())
		return nil, err
	}
	// 结束时删除一切解压内容
	defer func(dir string) {
		e := os.RemoveAll(dir)
		if e != nil {
			myutils.Logger.Error("remove all from dir", dir, "failed with:", e.Error())
		}
	}(ci.imgFilepath)

	// 查找数据库中是否已有digest对应的镜像结果
	analyzeBeginTime := time.Now()
	if myutils.GlobalDBClient.MongoFlag {
		res, err := myutils.GlobalDBClient.Mongo.FindImgResultByDigest(ci.digest)
		if err == nil {
			res.Name = name
			res.Registry = ci.registry
			res.Namespace = ci.namespace
			res.RepoName = ci.repoName
			res.TagName = ci.tagName
			res.Architecture = ci.architecture
			res.Variant = ci.variant
			res.OS = ci.osVersion
			res.OSVersion = ci.osVersion

			res.TotalTime = time.Since(beginTime).String()
			res.AnalyzeTime = time.Since(analyzeBeginTime).String()
			return res, nil
		}
	}

	// 数据库中没有结果，创建结果对象
	res := CurrentImageToImageResult(ci)
	res.LastAnalyzed = beginTimeStr

	// 分析镜像
	// 分析镜像元数据
	metaIs, err := analyzer.analyzeMetadata(ci)
	if err != nil {
		return nil, err
	}
	res.MetadataAnalyzed = true
	res.MetadataResult = metaIs

	// 分析镜像配置信息
	configIs, err := analyzer.analyzeConfiguration(ci)
	if err != nil {
		return nil, err
	}
	res.ConfigurationAnalyzed = true
	res.ConfigurationResult = configIs

	// 分析镜像内容信息
	contentIs, err := analyzer.analyzeContent(ci, res)
	if err != nil {
		return nil, err
	}
	res.ContentAnalyzed = true
	res.ContentResult = contentIs

	// 收尾赋值工作
	res.TotalTime = time.Since(beginTime).String()
	res.AnalyzeTime = time.Since(analyzeBeginTime).String()

	return res, nil
}

func CurrentImageToImageResult(ci *CurrentImage) *myutils.ImageResult {
	ir := myutils.NewImageResult()

	ir.Name = ci.name
	ir.Registry = ci.registry
	ir.Namespace = ci.namespace
	ir.RepoName = ci.repoName
	ir.TagName = ci.tagName
	ir.Digest = ci.digest
	ir.Architecture = ci.architecture
	ir.Variant = ci.variant
	ir.OS = ci.os
	ir.OSVersion = ci.osVersion

	ir.Layers = make([]string, len(ci.layerWithContentList))
	copy(ir.Layers, ci.layerWithContentList)
	for _, digest := range ci.layerWithContentList {
		ir.LayerResults[digest] = &myutils.LayerResult{
			Instruction: ci.layerInfoMap[digest].instruction,
			Size:        ci.layerInfoMap[digest].size,
			Digest:      digest,
		}
	}

	return ir
}
