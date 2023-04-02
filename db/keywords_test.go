package db

import (
	"fmt"
	"testing"
)

func TestDockerDB_InsertKeyword(t *testing.T) {
	db, err := NewDockerDB("docker:docker@/dockerhub")
	defer db.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}
	r, err := db.InsertKeyword("zzzz")
	if err != nil {
		t.Fatal("[ERROR] insert keyword failed with: ", err)
	}
	fmt.Println(r.RowsAffected())

	r, err = db.InsertKeyword("zzzzz")
	if err != nil {
		t.Fatal("[ERROR] insert keyword failed with: ", err)
	}
	fmt.Println(r.RowsAffected())

	r, err = db.InsertKeyword("zzzza")
	if err != nil {
		t.Fatal("[ERROR] insert keyword failed with: ", err)
	}
	fmt.Println(r.RowsAffected())
}

func TestDockerDB_GetLastKeyword(t *testing.T) {
	db, err := NewDockerDB("docker:docker@/dockerhub")
	defer db.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(db.GetLastKeyword())
}
