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
	AnalyzeTime  string `json:"analyze_time"`

	MetadataAnalyzed bool            `json:"metadata_analyzed"`
	MetadataResult   *MetadataResult `json:"metadata_result"`

	ConfigurationAnalyzed bool                 `json:"configuration_analyzed"`
	ConfigurationResult   *ConfigurationResult `json:"configuration_result"`

	ContentAnalyzed bool `json:"content_analyzed"`
	// Layers: [ layer-id1, layer-id2, ... ], from bottom to top
	Layers []string `json:"layers"`
	// LayerResults: layer-id -> LayerResult
	LayerResults map[string]*LayerResult `json:"layer_results"`
	// FileWithIssues: filepath -> bool, true: 文件包含问题为隐私泄露，false:文件包含问题不是隐私泄露
	//FileWithIssues map[string]bool `json:"-"`
	ContentResult *ContentResult `json:"content_result"`
}

type MetadataResult struct {
	SecretLeakages  []*SecretLeakage  `json:"secret_leakages"`
	SensitiveParams []*SensitiveParam `json:"sensitive_params"`
}

type ConfigurationResult struct {
	SecretLeakages []*SecretLeakage `json:"secret_leakages"`
}

type ContentResult struct {
	Components []*Component `json:"components"`

	SecretLeakages    []*SecretLeakage    `json:"secret_leakages"`
	Vulnerabilities   []*Vulnerability    `json:"vulnerabilities"`
	Misconfigurations []*Misconfiguration `json:"misconfiguration"`
	MaliciousFiles    []*MaliciousFile    `json:"malicious_files"`
}

type LayerResult struct {
	Instruction string `json:"instruction"`
	Size        int64  `json:"size"`
	Digest      string `json:"digest"`

	Total        int          `json:"total"` // from qianxin asky
	ComponentNum int          `json:"component_num"`
	Components   []*Component `json:"components"`

	SecretLeakages    []*SecretLeakage    `json:"secret_leakages"`
	Vulnerabilities   []*Vulnerability    `json:"vulnerabilities"`
	Misconfigurations []*Misconfiguration `json:"misconfiguration"`
	MaliciousFiles    []*MaliciousFile    `json:"malicious_files"`

	// 奇安信扫描taskid
	TaskID string `json:"task_id"`
}

type Component struct {
	Filename    string `json:"filename"`
	Codetype    string `json:"codetype"`
	Filepath    string `json:"filepath"`
	FileSha1    string `json:"file_sha1"`
	FileMd5     string `json:"file_md5"`
	FileVersion string `json:"file_version"`
	OpenSource  string `json:"open_source"`
}

func NewImageResult() *ImageResult {
	ir := new(ImageResult)

	ir.MetadataResult = NewMetadataResult()
	ir.ConfigurationResult = NewConfigurationResult()
	ir.Layers = make([]string, 0)
	ir.LayerResults = make(map[string]*LayerResult)
	//ir.FileWithIssues = make(map[string]bool)
	ir.ContentResult = NewContentResult()

	return ir
}

func NewMetadataResult() *MetadataResult {
	res := new(MetadataResult)

	res.SecretLeakages = make([]*SecretLeakage, 0)
	res.SensitiveParams = make([]*SensitiveParam, 0)

	return res
}

func NewConfigurationResult() *ConfigurationResult {
	res := new(ConfigurationResult)

	res.SecretLeakages = make([]*SecretLeakage, 0)

	return res
}

func NewContentResult() *ContentResult {
	res := new(ContentResult)

	res.Components = make([]*Component, 0)

	res.SecretLeakages = make([]*SecretLeakage, 0)
	res.Vulnerabilities = make([]*Vulnerability, 0)
	res.Misconfigurations = make([]*Misconfiguration, 0)
	res.MaliciousFiles = make([]*MaliciousFile, 0)

	return res
}

func NewLayerResult() *LayerResult {
	res := new(LayerResult)

	res.Components = make([]*Component, 0)

	res.SecretLeakages = make([]*SecretLeakage, 0)
	res.Vulnerabilities = make([]*Vulnerability, 0)
	res.Misconfigurations = make([]*Misconfiguration, 0)
	res.MaliciousFiles = make([]*MaliciousFile, 0)

	return res
}

type SecretLeakage struct {
	Type             string           `json:"type"`
	Name             string           `json:"name"`
	Part             string           `json:"part"` // part of image: metadata, configuration, content
	Path             string           `json:"path"`
	Match            string           `json:"match"`
	Description      string           `json:"description"`
	Severity         string           `json:"severity"`
	SeverityScore    float64          `json:"severity_score"`
	LayerDigest      string           `json:"layer_digest,omitempty"`
	TrufflehogResult TrufflehogResult `json:"trufflehog_result"`
}

type TrufflehogResult struct {
	SourceMetadata struct {
		Data struct {
			Filesystem struct {
				File string `json:"file"`
			} `json:"Filesystem"`
		} `json:"Data"`
	} `json:"SourceMetadata"`
	SourceID     int    `json:"SourceID"`
	SourceType   int    `json:"SourceType"`
	SourceName   string `json:"SourceName"`
	DetectorType int    `json:"DetectorType"`
	DetectorName string `json:"DetectorName"`
	DecoderName  string `json:"DecoderName"`
	Verified     bool   `json:"Verified"`
	Raw          string `json:"Raw"`
	RawV2        string `json:"RawV2"`
	Redacted     string `json:"Redacted"`
	ExtraData    struct {
		RotationGuide string `json:"rotation_guide"`
	} `json:"ExtraData"`
	StructuredData interface{} `json:"StructuredData"`
}

type SensitiveParam struct {
	Type          string  `json:"type"`
	Name          string  `json:"name"`
	Part          string  `json:"part"` // part of image: metadata, configuration, content
	Path          string  `json:"path"`
	Match         string  `json:"match"`
	SensitiveType string  `json:"sensitive_type"`
	Description   string  `json:"description"`
	Severity      string  `json:"severity"`
	SeverityScore float64 `json:"severity_score"`
}

type Vulnerability struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Part        string `json:"part"` // part of image: metadata, configuration, content
	Path        string `json:"path"`
	LayerDigest string `json:"layer_digest"`

	CVEID           string   `json:"cve_id"`
	Filename        string   `json:"filename"`
	ProductName     string   `json:"product_name"`
	VendorName      string   `json:"vendor_name"`
	Version         string   `json:"version"`
	VulnType        string   `json:"vuln_type"`
	ThrType         string   `json:"thr_type"`
	PublishedTime   string   `json:"published_time"`
	Description     string   `json:"description"`
	Severity        string   `json:"severity"`
	CVSSScore       float64  `json:"cvss_score"`
	AffectComponent []string `json:"affect_component"`
	AffectFile      []string `json:"affect_file"`
}

type Misconfiguration struct {
	Type          string  `json:"type"`
	AppName       string  `json:"app_name"`
	MisConfType   string  `json:"mis_conf_type"`
	Part          string  `json:"part"` // part of image: metadata, configuration, content
	Path          string  `json:"path"`
	Match         string  `json:"match"`
	Description   string  `json:"description"`
	Severity      string  `json:"severity"`
	SeverityScore float64 `json:"severity_score"`
	LayerDigest   string  `json:"layer_digest"`
}

type MaliciousFile struct {
	Type          string  `json:"type"`
	Name          string  `json:"name"`
	Part          string  `json:"part"` // part of image: metadata, configuration, content
	Path          string  `json:"path"`
	Description   string  `json:"description"`
	Severity      string  `json:"severity"`
	SeverityScore float64 `json:"severity_score"`
	LayerDigest   string  `json:"layer_digest"`

	Sha256          string  `json:"sha256"` // sha256 of file, only for malicious file
	Level           int     `json:"level"`
	MalwareTypeName string  `json:"malware_type_name"`
	FileDesc        string  `json:"file_desc"`
	Describe        string  `json:"describe"`
	MaliciousFamily string  `json:"malicious_family"`
	SandboxScore    float64 `json:"sandbox_score"`
}

// Deprecated: 不统一实现了
//// Issue 表示一条发现的问题
//// TODO: 需要考虑怎么统一所有检测的结果
//type Issue struct {
//	Type          string  `json:"type"`
//	Name          string  `json:"name"`
//	Part          string  `json:"part"` // part of image: metadata, configuration, content
//	Path          string  `json:"path"`
//	Sha256        string  `json:"sha256,omitempty"`  // sha256 of file, only for malicious file
//	Version       string  `json:"version,omitempty"` // version of the product, only for vulnerability
//	Match         string  `json:"match,omitempty"`
//	Description   string  `json:"description"`
//	Severity      string  `json:"severity"`
//	SeverityScore float64 `json:"severity_score"`
//	LayerDigest   string  `json:"layer_digest,omitempty"`
//}

var IssueType = struct {
	SecretLeakage    string
	SensitiveParam   string
	Vulnerability    string
	Misconfiguration string
	MaliciousFile    string
}{
	"secret-leakage",
	"sensitive-parameter",
	"vulnerability",
	"misconfiguration",
	"malicious-file",
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
