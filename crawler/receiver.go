package crawler

// 用于实现json marshal和unmarshal的各个结构体

// 初始化一系列接收器
var (
	RegisterRepoList RegisterRepoList__
)

var (
	ChannelRegRepoList = make(chan RegisterRepoList__)
)

// RegisterRepoList__ 收录爬到的repo list数量
type RegisterRepoList__ struct {
	PageSize  int       `json:"page_size"`
	Next      string    `json:"next"`     // 记录下一页
	Previous  string    `json:"previous"` // 记录上一页
	Page      int       `json:"page"`
	Count     int       `json:"count"`
	Summaries []Summary `json:"summaries"`
}

type Summary struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type NamespaceRepoList__ struct {
}

type Repository__ struct {
	User            string   `json:"user"`
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	RepositoryType  string   `json:"repository_type"`
	Description     string   `json:"description"`
	IsPrivate       bool     `json:"is_private"`
	IsAutomated     bool     `json:"is_automated"`
	CanEdit         bool     `json:"can_edit"`
	StarCount       int      `json:"star_count"`
	PullCount       int      `json:"pull_count"`
	LastUpdated     string   `json:"last_updated"`
	DateRegistered  string   `json:"date_registered"`
	FullDescription string   `json:"full_description"`
	MediaTypes      []string `json:"media_types"`
	ContentTypes    []string `json:"content_types"`
}

type Tag__ struct {
	Archs []Arch__
}

type Arch__ struct {
	Architecture string    `json:"architecture"`
	Digest       string    `json:"digest"`
	Layers       []Layer__ `json:"layers"`
	OS           string    `json:"os"`
	Size         int       `json:"size"`
	Status       string    `json:"status"`
	LastPulled   string    `json:"last_pulled"`
	LastPushed   string    `json:"last_pushed"`
}

type Layer__ struct {
	Digest      string `json:"digest,omitempty"`
	Size        int    `json:"size"`
	Instruction string `json:"instruction"`
}
