package analyzer

import "fmt"

// parseContentFromFile parses content information from local file,
// including locating local filepath of layers.
func (currI *CurrentImage) parseContentFromFile() error {
	// 从image metadata中提取有文件内容的layer信息
	currI.parseLayersWithContentFromMetadata()

	// 根据文件解压情况对应镜像文件本地位置（不需要root权限）
	if len(currI.layerWithContentList) != len(currI.layerLocalFilepathList) {
		return fmt.Errorf("count of layers from image metadata(%d) is not equal with count of layers from image tar manifest(%d)",
			len(currI.layerWithContentList), len(currI.layerLocalFilepathList))
	}
	for i, digest := range currI.layerWithContentList {
		currI.layerInfoMap[digest].localFilePath = currI.layerLocalFilepathList[i]
	}

	return nil
}

// parseContentFromDockerEnv parses content information from local Docker env,
// including locating local filepath of layers.
func (currI *CurrentImage) parseContentFromDockerEnv() error {
	currI.parseLayersWithContentFromMetadata()

	// TODO: 根据LowerDir和UpperDir获取Docker维护的本地层文件位置（需要root权限）
	//for _, graphPath := range currI.configuration.GraphDriver.Data[] {
	//
	//}

	return nil
}

// parseLayersWithContentFromMetadata extracts layer digests from layer metadata.
func (currI *CurrentImage) parseLayersWithContentFromMetadata() {
	for _, layer := range currI.metadata.imageMetadata.Layers {
		if layer.Digest != "" {
			currI.layerWithContentList = append(currI.layerWithContentList, layer.Digest)
			currI.layerInfoMap[layer.Digest] = &layerInfo{
				size:        layer.Size,
				instruction: layer.Instruction,
				digest:      layer.Digest,
			}
		}
	}
}
