package buildgraph

// 用于json marshal和unmarshal的接收器模板

type Repository struct {
	User           string `json:"user"`
	Repository     string `json:"name"`
	Namespace      string `json:"namespace"`
	RepositoryType string `json:"repository_type"`
	IsPrivate      bool   `json:"is_private"`
	IsAutomated    bool   `json:"is_automated"`
	StarCount      int    `json:"star_count"`
	PullCount      int64  `json:"pull_count"`
	LastUpdated    string `json:"last_updated"`
	DateRegistered string `json:"date_registered"`
	Tags           map[string]Tag
}

type Tag struct {
	LastUpdated         string `json:"last_updated"`
	LastUpdaterUsername string `json:"last_updater_username"`
	TagLastPulled       string `json:"tag_last_pulled"`
	TagLastPushed       string `json:"tag_last_pushed"`
	MediaType           string `json:"media_type"`
	ContentType         string `json:"content_type"`
	Images              []Image
}

type Image struct {
	Architecture string
	Features     string
	Variant      string
	Digest       string
	Layers       []Layer // 只按序保存layer的digest
	OS           string
	Size         int64
	Status       string
	LastPulled   string
	LastPushed   string
}

// 以Source结尾的struct用于从数据源中直接Unmarshal接收数据

type RepositorySource struct {
	User            string `json:"user"`
	Repository      string `json:"name"`
	Namespace       string `json:"namespace"`
	RepositoryType  string `json:"repository_type"`
	Description     string `json:"description"`
	IsPrivate       bool   `json:"is_private"`
	IsAutomated     bool   `json:"is_automated"`
	StarCount       int    `json:"star_count"`
	PullCount       int64  `json:"pull_count"`
	LastUpdated     string `json:"last_updated"`
	DateRegistered  string `json:"date_registered"`
	FullDescription string `json:"full_description,omitempty"`
}

type TagSource struct {
	Namespace           string `json:"namespace"`
	Repository          string `json:"repository"`
	Tag                 string `json:"name"`
	LastUpdated         string `json:"last_updated"`
	LastUpdaterUsername string `json:"last_updater_username"`
	TagLastPulled       string `json:"tag_last_pulled"`
	TagLastPushed       string `json:"tag_last_pushed"`
	MediaType           string `json:"media_type"`
	ContentType         string `json:"content_type"`
}

type ImageSource struct {
	Namespace  string `json:"namespace"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Image      Image  `json:"arch"`
}

type Layer struct {
	Digest      string `json:"digest,omitempty"`
	Size        int    `json:"size"`
	Instruction string `json:"instruction"`
}
