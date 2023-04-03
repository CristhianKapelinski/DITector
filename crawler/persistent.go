package crawler

import (
	"database/sql"
	"encoding/json"
)

// 实现数据持久化的简单接口

// StoreRepository__ 将Repository__直接组织成合适的形式存入数据库
func StoreRepository__(r *Repository__) (sql.Result, error) {
	var flag int8
	if r.IsPrivate {
		flag |= 1 << 0
	}
	if r.IsAutomated {
		flag |= 1 << 1
	}

	return dockerDB.InsertRepository(r.User, r.Name, r.Namespace, r.RepositoryType, r.Description, flag,
		r.StarCount, r.PullCount, r.LastUpdated[:19], r.DateRegistered[:19], r.FullDescription)
}

// StoreTag__ 将Tag__直接组织成合适的形式存入数据库
func StoreTag__(namespace, repository string, t *Tag__) (sql.Result, error) {

	return dockerDB.InsertTag(namespace, repository, t.Name, t.LastUpdated[:19], t.LastUpdaterUsername,
		t.TagLastPulled[:19], t.TagLastPushed[:19], t.MediaType, t.ContentType)
}

// StoreArch__ 将Arch__组织成合适的形式存入数据库
func StoreArch__(namespace, repository, tag string, a *Arch__) (sql.Result, error) {

	b, _ := json.Marshal(a.Layers)

	return dockerDB.InsertImage(namespace, repository, tag, a.Architecture, a.Features, a.Variant,
		a.Digest[7:], a.OS, a.Size, a.Status, a.LastPulled[:19], a.LastPushed[:19], string(b))
}

// StoreLayer__ 将Layer__组织成合适的形式存入数据库
func StoreLayer__(l *Layer__) (sql.Result, error) {

	return dockerDB.InsertLayer(l.Digest[7:], l.Size, l.Instruction)
}
