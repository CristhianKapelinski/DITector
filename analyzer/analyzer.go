package analyzer

import (
	"myutils"
	"strconv"
)

type ImageAnalyzer struct {
	DockerClient
	Mongo       *myutils.MyMongo
	Neo4jDriver *myutils.MyNeo4j
	rules       Rules
	CurrentImage
}

type CurrentImage struct {
	Registry           string
	Namespace          string
	Repository         string
	RepositoryMetadata *myutils.Repository
	Tag                string
	Digest             string
	LayerLocalFileMap  map[string]string
}

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

	for _, secret := range imageAnalyzer.rules.Secrets {
		// diff parts like contents, extension, filename, and ...
		if secret.Part == part {
			matches := secret.CompiledRegex.FindAllString(s, -1)
			for _, match := range matches {
				tmp := &myutils.Result{
					RuleName:      secret.Name,
					PartToMatch:   secret.Part,
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
