package db

import (
	"crawler"
	"database/sql"
	"fmt"
)

// keyword.go为DockerDB绑定repository插入、删除和查询功能

const insertRepository = `
INSERT INTO 
repository 
(user,name,namespace,repository_type,description,flag,star_count,pull_count,last_updated,date_registered,full_description)
VALUES
(
'%s','%s','%s','%s','%s',%d,%d,%d,'%s','%s','%s'
);
`

// InsertRepository__ 根据crawler.Repository__插入数据库记录
func (d *DockerDB) InsertRepository__(r crawler.Repository__) (sql.Result, error) {
	var flag int8
	if r.IsPrivate {
		flag |= 1 << 0
	}
	if r.IsAutomated {
		flag |= 1 << 1
	}

	return d.db.Exec(fmt.Sprintf(insertRepository,
		r.User, r.Name, r.Namespace, r.RepositoryType, r.Description, flag,
		r.StarCount, r.PullCount, r.LastUpdated[:19], r.DateRegistered[:19],
		r.FullDescription),
	)
}
