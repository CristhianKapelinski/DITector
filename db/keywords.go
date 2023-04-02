package db

import (
	"database/sql"
	"fmt"
)

// keyword.go为DockerDB绑定keyword插入和查询功能

const insertKeyword = `INSERT IGNORE INTO keywords (name) VALUE ('%s');`
const getLastKeyword = `SELECT name FROM keywords ORDER BY name DESC LIMIT 1;`

func (d *DockerDB) InsertKeyword(keyword string) (sql.Result, error) {
	return d.db.Exec(fmt.Sprintf(insertKeyword, keyword))
}

func (d *DockerDB) GetLastKeyword() (string, error) {
	var r struct{ k string }
	err := d.db.QueryRow(getLastKeyword).Scan(&r.k)
	return r.k, err
}
