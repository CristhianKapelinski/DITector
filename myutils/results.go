package myutils

// ImageResult is used to store analysis result
type ImageResult struct {
	Digest       string   `json:"digest"`
	LastAnalyzed string   `json:"last_analyzed"`
	Results      []Result `json:"results"`
}

type Result struct {
	RuleName      string  `json:"rule_name"`
	Type          string  `json:"type"`
	Path          string  `json:"path"`
	PartToMatch   string  `json:"part_to_match"`
	Match         string  `json:"match"`
	Severity      string  `json:"severity"`
	SeverityScore float64 `json:"severity_score"`
	LayerDigest   string  `json:"layer_digest,omitempty"`
}
