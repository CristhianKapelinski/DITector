package myutils

// ImageResult is used to store analysis result
type ImageResult struct {
	Name         string `json:"name"`
	Registry     string `json:"registry"`
	Namespace    string `json:"namespace"`
	RepoName     string `json:"repository_name"`
	TagName      string `json:"tag_name"`
	Digest       string `json:"digest"`
	Architecture string `json:"architecture"`
	Variant      string `json:"variant"`
	OS           string `json:"os"`
	OSVersion    string `json:"os_version"`

	LastAnalyzed string `json:"last_analyzed"`
	TotalTime    string `json:"total_time"`

	MetadataAnalyzed bool     `json:"metadata_analyzed"`
	MetadataIssues   []*Issue `json:"metadata_issues"`

	ConfigurationAnalyzed bool     `json:"configuration_analyzed"`
	ConfigurationIssues   []*Issue `json:"configuration_issues"`

	ContentAnalyzed bool `json:"content_analyzed"`
	// Layers: [ layer-id1, layer-id2, ... ], from bottom to top
	Layers []string `json:"layers"`
	// LayerResults: layer-id -> LayerResult
	LayerResults map[string]*LayerResult `json:"layer_results"`
	// fileIssues: filepath -> []*Issue, issues in the file system after mounting by UnionFS
	fileIssues    map[string][]*Issue
	ContentIssues []*Issue `json:"content_issues"`
}

type LayerResult struct {
	Instruction   string   `json:"instruction"`
	Size          int64    `json:"size"`
	Digest        string   `json:"digest"`
	AnalyzedFiles []string `json:"analyzed_files"`
	// FileIssues: filepath -> []*Issue
	FileIssues map[string][]*Issue `json:"file_issues"`
}

func NewImageResult() *ImageResult {
	ir := new(ImageResult)

	ir.MetadataIssues = make([]*Issue, 0)
	ir.ConfigurationIssues = make([]*Issue, 0)
	ir.Layers = make([]string, 0)
	ir.LayerResults = make(map[string]*LayerResult)
	ir.fileIssues = make(map[string][]*Issue)
	ir.ContentIssues = make([]*Issue, 0)

	return ir
}

// Issue 表示一条发现的问题
// TODO: 需要考虑怎么统一所有检测的结果
type Issue struct {
	Type          string  `json:"type"`
	Part          string  `json:"part"` // part of image: metadata, configuration, content
	Path          string  `json:"path"`
	RuleName      string  `json:"rule_name"`
	Match         string  `json:"match"`
	Description   string  `json:"description"`
	Severity      string  `json:"severity"`
	SeverityScore float64 `json:"severity_score"`
	LayerDigest   string  `json:"layer_digest"`
}

var IssueType = struct {
	SecretLeakage     string
	SensitiveParam    string
	Vulnerability     string
	Misconfiguration  string
	MaliciousSoftware string
}{
	"secret-leakage",
	"sensitive-parameter",
	"vulnerability",
	"misconfiguration",
	"malicious-software",
}

var IssuePart = struct {
	RepoMetadata  string
	TagMetadata   string
	ImageMetadata string
	Configuration string
	Content       string
}{
	"repository-metadata",
	"tag-metadata",
	"image-metadata",
	"configuration",
	"content",
}

func AddIssue(dest []*Issue, src ...*Issue) {
	for _, i := range src {
		for _, j := range dest {
			if *i == *j {
				break
			}
		}
		dest = append(dest, i)
	}
}
