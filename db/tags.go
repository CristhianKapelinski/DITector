package db

import (
	"database/sql"
	"fmt"
)

// keyword.go为DockerDB绑定tag插入、删除和查询功能

const insertTag = `
INSERT IGNORE INTO
tags
(namespace,repository,name,last_updated,last_updater_username,tag_last_pulled,tag_last_pushed,media_type, content_type)
VALUES 
(
'%s','%s','%s','%s','%s','%s','%s','%s','%s'
);
`

func (d *DockerDB) InsertTag(namespace, repository, tag, lastUpdated, lastUpdaterUsername, tagLastPulled,
	tagLastPushed, mediaType, contentType string) (sql.Result, error) {

	insert := fmt.Sprintf(insertTag,
		namespace, repository, tag, lastUpdated, lastUpdaterUsername,
		tagLastPulled, tagLastPushed, mediaType, contentType)

	return d.db.Exec(insert)
}
