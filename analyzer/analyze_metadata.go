package analyzer

import "github.com/Musso12138/dockercrawler/myutils"

func (analyzer *ImageAnalyzer) analyzeMetadata(ci *CurrentImage) ([]*myutils.Issue, error) {
	res := make([]*myutils.Issue, 0)

	rmi, err := analyzer.analyzeRepoMetadata(ci)
	if err != nil {
		return nil, err
	}
	myutils.AddIssue(res, rmi...)

	return res, nil
}

func (analyzer *ImageAnalyzer) analyzeRepoMetadata(ci *CurrentImage) ([]*myutils.Issue, error) {
	// 分析敏感参数
	// full_description中推荐的`docker run`
}

func (analyzer *ImageAnalyzer) analyzeImageMetadata() ([]*myutils.Issue, error) {

}
