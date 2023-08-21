package crawler

import (
	"fmt"
)

// 用于用于json marshal和unmarshal的接收器模板

// RegisterRepoList__ 收录单次返回的repo list（信封）
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
	User            string `json:"user"`
	Name            string `json:"name"`
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
	//MediaTypes      []string `json:"media_types"`
	//ContentTypes    []string `json:"content_types"`
	Tags []Tag__ // 不从collector中unmarshal进来，而是后续append进来
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
	//Digest              string `json:"digest"`
	Archs []Arch__
}

// Arch__ 是一个namespace/repo:tag在特定架构下的镜像信息
type Arch__ struct {
	Architecture string    `json:"architecture"`
	Features     string    `json:"features"`
	Variant      string    `json:"variant"`
	Digest       string    `json:"digest"`
	Layers       []Layer__ `json:"layers"`
	OS           string    `json:"os"`
	Size         int64     `json:"size"`
	Status       string    `json:"status"`
	LastPulled   string    `json:"last_pulled"`
	LastPushed   string    `json:"last_pushed"`
}

type Layer__ struct {
	Digest      string `json:"digest,omitempty"`
	Size        int64  `json:"size"`
	Instruction string `json:"instruction"`
}

func (r Repository__) String() string {
	return fmt.Sprintf(
		`Metadata-------------------------------------
User: %s
Name: %s
Namespace: %s
RepositoryType: %s
Description: %s
IsPrivate: %v
IsAutomated: %v
StarCount: %d
PullCount: %d
LastUpdated: %s
DateRegistered: %s
FullDescription: %s
Tags: %v`,
		r.User, r.Name, r.Namespace, r.RepositoryType,
		r.Description, r.IsPrivate, r.IsAutomated,
		r.StarCount, r.PullCount, r.LastUpdated, r.DateRegistered,
		r.FullDescription, r.Tags,
	)
}

func (t Tag__) String() string {
	return fmt.Sprintf(
		`TagName----------------------------------------
	Name: %s
	LastUpdated: %s
	LastUpdaterUsername: %s
	TagLastPulled: %s
	TagLastPushed: %s
	MediaType: %s
	ContentType: %s
	Archs: %v`,
		t.Name, t.LastUpdated, t.LastUpdaterUsername, t.TagLastPulled,
		t.TagLastPushed, t.MediaType, t.ContentType, t.Archs,
	)
}

func (a Arch__) String() string {
	return fmt.Sprintf(
		`Arch------------------------------------------
		Architecture: %s
		Features: %s
		Variant: %s
		Digest: %s
		Layers: %v
		OS: %s
		Size: %d
		Status: %s
		LastPulled: %s
		LastPushed: %s`,
		a.Architecture, a.Features, a.Variant, a.Digest, a.Layers,
		a.OS, a.Size, a.Status, a.LastPulled, a.LastPushed,
	)
}

func (l Layer__) String() string {
	return fmt.Sprintf(
		`Layer--------------------------------------
			Digest: %s
			Size: %d
			Instruction: %s`,
		l.Digest, l.Size, l.Instruction,
	)
}
