package crawler

// 用于实现json marshal和unmarshal的各个结构体

// 初始化一系列接收器模板与通道

var (
	ChannelRegRepoList = make(chan RegisterRepoList__, 5)
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

// Summary 是RegURL获取到的对单个Repo的描述，有效信息仅包括name和source
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
	Tags            []Tag__  // 不从collector中unmarshal进来，而是后续append进来
}

// TagReceiver__ 用于ScrapeRepoTagsRecursive，得到的Results要append到Repository__中。
// 纯粹用于接受Tag List，方便collector接收，用于做unmarshal的模板。
type TagReceiver__ struct {
	Count    int     `json:"count"`
	Next     string  `json:"next"`
	Previous string  `json:"previous"`
	Results  []Tag__ `json:"results"`
}

// Tag__ 是一个Tag对应的所有镜像的集合。
// 具体对镜像的digest、layer history描述在Archs中。
type Tag__ struct {
	Name                string `json:"name"`
	LastUpdated         string `json:"last_updated"`
	LastUpdaterUsername string `json:"last_updater_username"`
	TagLastPulled       string `json:"tag_last_pulled"`
	TagLastPushed       string `json:"tag_last_pushed"`
	MediaType           string `json:"media_type"`
	ContentType         string `json:"content_type"`
	Archs               []Arch__
}

// Arch__ 是一个namespace/repo:tag在特定架构下的镜像信息
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
