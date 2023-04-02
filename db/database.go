package db

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

type DockerDB struct {
	// 传入数据库
	DSN string
	db  *sql.DB
}

func NewDockerDB(dsn string) (*DockerDB, error) {
	d := DockerDB{DSN: dsn}
	var err error
	d.db, err = sql.Open("mysql", dsn)
	return &d, err
}

func (d *DockerDB) Ping() error {
	return d.db.Ping()
}

func (d *DockerDB) Close() error {
	return d.db.Close()
}

// EscapeString
// TODO: Escape string
func EscapeString(s string) string {
	return s
}
