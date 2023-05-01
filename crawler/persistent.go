package crawler

import (
	"database/sql"
	"encoding/json"
	"os"
	"sync"
)

// 实现数据持久化的简单接口

// 先不直接存储到数据库了，改成添加到文件中，以下内容在config.go中初始化
var (
	fileRepository *os.File
	lockRepository = sync.Mutex{}
	fileTags       *os.File
	lockTags       = sync.Mutex{}
	fileImages     *os.File
	lockImages     = sync.Mutex{}
)

// StoreRepository__ToFile 将Repository__存储到文件fileRepository中
func StoreRepository__ToFile(r *Repository__) (int, error) {
	lockRepository.Lock()
	defer lockRepository.Unlock()
	tmp := struct {
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
	}{r.User, r.Name, r.Namespace, r.RepositoryType, r.Description, r.IsPrivate,
		r.IsAutomated, r.StarCount, r.PullCount, r.LastUpdated, r.DateRegistered, r.FullDescription}
	b, err := json.Marshal(tmp)
	if err != nil {
		return 0, err
	}
	n, err := fileRepository.Write(b)
	if err != nil {
		return 0, err
	}
	fileRepository.WriteString("\n")
	return n, err
}

// StoreTag__ToFile 将Tag__存储到文件fileTags中
func StoreTag__ToFile(namespace, repository string, t *Tag__) (int, error) {
	lockTags.Lock()
	defer lockTags.Unlock()
	tmp := struct {
		Namespace           string
		Repository          string
		Name                string `json:"name"`
		LastUpdated         string `json:"last_updated"`
		LastUpdaterUsername string `json:"last_updater_username"`
		TagLastPulled       string `json:"tag_last_pulled"`
		TagLastPushed       string `json:"tag_last_pushed"`
		MediaType           string `json:"media_type"`
		ContentType         string `json:"content_type"`
	}{namespace, repository, t.Name, t.LastUpdated, t.LastUpdaterUsername,
		t.TagLastPulled, t.TagLastPushed, t.MediaType, t.ContentType}
	b, err := json.Marshal(tmp)
	if err != nil {
		return 0, err
	}
	n, err := fileTags.Write(b)
	if err != nil {
		return 0, err
	}
	fileTags.WriteString("\n")
	return n, err
}

// StoreArch__ToFile 将image存储到文件fileImages中
func StoreArch__ToFile(namespace, repository, tag string, a *Arch__) (int, error) {
	lockImages.Lock()
	defer lockImages.Unlock()
	tmp := struct {
		Namespace  string
		Repository string
		Tag        string
		Arch       *Arch__
	}{namespace, repository, tag, a}
	b, err := json.Marshal(tmp)

	if err != nil {
		return 0, err
	}
	n, err := fileImages.Write(b)
	if err != nil {
		return 0, err
	}
	fileImages.WriteString("\n")
	return n, err
}

// StoreRepository__ 将Repository__直接组织成合适的形式存入数据库
func StoreRepository__(r *Repository__) (sql.Result, error) {
	var flag int8
	if r.IsPrivate {
		flag |= 1 << 0
	}
	if r.IsAutomated {
		flag |= 1 << 1
	}

	var lu, dr string
	if len(r.LastUpdated) > 19 {
		lu = r.LastUpdated[:19]
	}
	if len(r.DateRegistered) > 19 {
		dr = r.DateRegistered[:19]
	}

	return dockerDB.InsertRepository(r.User, r.Name, r.Namespace, r.RepositoryType, r.Description, flag,
		r.StarCount, r.PullCount, lu, dr, r.FullDescription)
}

// StoreTag__ 将Tag__直接组织成合适的形式存入数据库
func StoreTag__(namespace, repository string, t *Tag__) (sql.Result, error) {

	var lu, lpull, lpush string

	if len(t.LastUpdated) > 19 {
		lu = t.LastUpdated[:19]
	}
	if len(t.TagLastPulled) > 19 {
		lpull = t.TagLastPulled[:19]
	}
	if len(t.TagLastPushed) > 19 {
		lpush = t.TagLastPushed[:19]
	}

	return dockerDB.InsertTag(namespace, repository, t.Name, lu, t.LastUpdaterUsername,
		lpull, lpush, t.MediaType, t.ContentType)
}

// StoreArch__ 将Arch__组织成合适的形式存入数据库
func StoreArch__(namespace, repository, tag string, a *Arch__) (sql.Result, error) {

	b, _ := json.Marshal(a.Layers)

	var d, lpull, lpush string

	if len(a.Digest) > 8 {
		d = a.Digest[7:]
	}
	if len(a.LastPulled) > 19 {
		lpull = a.LastPulled[:19]
	}
	if len(a.LastPushed) > 19 {
		lpush = a.LastPushed[:19]
	}

	return dockerDB.InsertImage(namespace, repository, tag, a.Architecture, a.Features, a.Variant,
		d, a.OS, a.Size, a.Status, lpull, lpush, string(b))
}

// StoreLayer__ 将Layer__组织成合适的形式存入数据库
func StoreLayer__(l *Layer__) (sql.Result, error) {

	var d string

	if len(l.Digest) > 8 {
		d = l.Digest[7:]
	}

	return dockerDB.InsertLayer(d, l.Size, l.Instruction)
}
