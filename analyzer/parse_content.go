package analyzer

// parseContentFromDockerEnv parses content information,
// including locating local filepath of layers.
func (currI *CurrentImage) parseContentFromDockerEnv() error {
	currI.extractLayersFromMetadata()

	return nil
}

// extractLayersFromMetadata extracts layer digests from layer metadata.
func (currI *CurrentImage) extractLayersFromMetadata() {
	for _, layer := range currI.metadata.imageMetadata.Layers {
		if layer.Digest != "" {
			currI.layerWithContentList = append(currI.layerWithContentList, layer.Digest)
			currI.layerInfoMap[layer.Digest] = layerInfo{
				size:        layer.Size,
				instruction: layer.Instruction,
				digest:      layer.Digest,
			}
		}
	}
}
