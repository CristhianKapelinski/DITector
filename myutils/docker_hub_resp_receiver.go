package myutils

// Docker Hub API返回的直接结果格式

type Repository struct {
	User              string      `json:"user"`
	Name              string      `json:"name"`
	Namespace         string      `json:"namespace"`
	RepositoryType    string      `json:"repository_type"`
	Status            int         `json:"status"`
	StatusDescription string      `json:"status_description"`
	Description       string      `json:"description"`
	IsPrivate         bool        `json:"is_private"`
	IsAutomated       bool        `json:"is_automated"`
	StarCount         int64       `json:"star_count"`
	PullCount         int64       `json:"pull_count"`
	LastUpdated       string      `json:"last_updated"`
	DateRegistered    string      `json:"date_registered"`
	FullDescription   string      `json:"full_description"`
	Permissions       Permissions `json:"permissions"`
	MediaTypes        []string    `json:"media_types"`
	ContentTypes      []string    `json:"content_types"`
}

type Permissions struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
	Admin bool `json:"admin"`
}

type Tag struct {
	RepositoryNamespace string `json:"repositories_namespace"`
	RepositoryName      string `json:"repositories_name"`

	Creator             int          `json:"creator"`
	Id                  int          `json:"id"`
	Images              []ImageInTag `json:"images"`
	LastUpdated         string       `json:"last_updated"`
	LastUpdater         int          `json:"last_updater"`
	LastUpdaterUsername string       `json:"last_updater_username"`
	Name                string       `json:"name"`
	FullSize            int64        `json:"full_size"`
	V2                  bool         `json:"v2"`
	TagStatus           string       `json:"tag_status"`
	TagLastPulled       string       `json:"tag_last_pulled"`
	TagLastPushed       string       `json:"tag_last_pushed"`
	MediaType           string       `json:"media_type"`
	ContentType         string       `json:"content_type"`
	Digest              string       `json:"digest"`
}

type ImageInTag struct {
	Architecture string `json:"architecture"`
	Features     string `json:"features"`
	Variant      string `json:"variant"`
	Digest       string `json:"digest"`
	OS           string `json:"os"`
	OSFeatures   string `json:"os_features"`
	OSVersion    string `json:"os_version"`
	Size         int64  `json:"size"`
	Status       string `json:"status"`
	LastPulled   string `json:"last_pulled"`
	LastPushed   string `json:"last_pushed"`
}

type Image struct {
	Architecture string  `json:"architecture"`
	Features     string  `json:"features"`
	Variant      string  `json:"variant"`
	Digest       string  `json:"digest"`
	Layers       []Layer `json:"layers"`
	OS           string  `json:"os"`
	OSFeatures   string  `json:"os_features"`
	OSVersion    string  `json:"os_version"`
	Size         int64   `json:"size"`
	Status       string  `json:"status"`
	LastPulled   string  `json:"last_pulled"`
	LastPushed   string  `json:"last_pushed"`
}

type Layer struct {
	Digest      string `json:"digest,omitempty"`
	Size        int64  `json:"size"`
	Instruction string `json:"instruction"`
}
