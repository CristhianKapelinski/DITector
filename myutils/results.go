package myutils

// ImageResult is used to store analysis result
type ImageResult struct {
	Namespace    string        `json:"namespace"`
	Repository   string        `json:"repository"`
	Tag          string        `json:"tag"`
	Name         string        `json:"name"`
	Digest       string        `json:"digest"`
	LastAnalyzed string        `json:"last_analyzed"`
	LayerResults []LayerResult `json:"layer_results"`
	Results      []*Result     `json:"results"`
}

type LayerResult struct {
	Instruction   string   `json:"instruction"`
	Digest        string   `json:"digest"`
	AnalyzedFiles []string `json:"analyzed_files"`
}

// Result TODO: 需要考虑怎么统一所有检测的结果
type Result struct {
	RuleName      string  `json:"rule_name"`
	Type          string  `json:"type"`
	Path          string  `json:"path"`
	PartToMatch   string  `json:"part_to_match"`
	Match         string  `json:"match"`
	Severity      string  `json:"severity"`
	SeverityScore float64 `json:"severity_score"`
	LayerDigest   string  `json:"layer_digest"`
}
