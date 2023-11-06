package analyzer

import (
	"github.com/Musso12138/dockercrawler/myutils"
)

func (analyzer *ImageAnalyzer) analyzeContent(ci *CurrentImage, ir *myutils.ImageResult) ([]*myutils.Issue, error) {
	res := make([]*myutils.Issue, 0)

	// 逐层分析layer内容，写入对应LayerResult
	for _, ld := range ir.Layers {
		// 数据库在线，检查是否已被分析
		if myutils.GlobalDBClient.MongoFlag {
			if lr, err := myutils.GlobalDBClient.Mongo.FindLayerResultByDigest(ld); err == nil {

				continue
			}
		}
		if err := analyzer.analyzeLayer(ci.layerInfoMap[ld].localFilePath, ir.LayerResults[ld]); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// analyzeLayer traverses and analyzes files under inputted layerDir,
// and writes results directly to layerResult.
func (analyzer *ImageAnalyzer) analyzeLayer(layerDir string, layerResult *myutils.LayerResult) error {

	return nil
}
