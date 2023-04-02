package db

import (
	"database/sql"
	"fmt"
)

// keyword.go为DockerDB绑定images插入、删除和查询功能

const insertImage = `
INSERT IGNORE INTO
images
(namespace,repository,tag,architecture,features,variant,digest,os,size,status,last_pulled,last_pushed,layers)
VALUES 
(
'%s','%s','%s','%s','%s','%s','%s','%s',%d,'%s','%s','%s','%s'
);
`

func (d *DockerDB) InsertImage(namespace, repository, tag, architecture, features, variant, digest, os string,
	size int64, status, lastPulled, lastPushed, layers string) (sql.Result, error) {

	insert := fmt.Sprintf(insertImage,
		namespace, repository, tag, architecture, features, variant, digest, os, size, status,
		lastPulled, lastPushed, EscapeString(layers))

	return d.db.Exec(insert)
}
