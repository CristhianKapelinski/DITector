package db

import (
	"database/sql"
	"fmt"
)

// keyword.go为DockerDB绑定repository插入、删除和查询功能

const insertRepository = `
INSERT IGNORE INTO 
repository 
(user,name,namespace,repository_type,description,flag,star_count,pull_count,last_updated,date_registered,full_description)
VALUES
(
'%s','%s','%s','%s','%s',%d,%d,%d,'%s','%s','%s'
);
`

// InsertRepository 根据向repository表插入数据库记录
func (d *DockerDB) InsertRepository(user, name, namespace, repositoryType, description string, flag int8, starCount int,
	pullCount int64, lastUpdated, dateRegistered, fullDescription string) (sql.Result, error) {

	insert := fmt.Sprintf(insertRepository,
		user, name, namespace, repositoryType, EscapeString(description), flag,
		starCount, pullCount, lastUpdated, dateRegistered, EscapeString(fullDescription))

	return d.db.Exec(insert)
}
