package main

import (
	"time"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type metadate struct {
	claimantName string;
	dateofInjury string;
	employer string;
	adjuster string;
	support string;
	claimType string;
	jurisdiction string;
	policy string;
}

var db *sql.DB

func InitDB() error {
	var err error
	db, err = sql.Open("sqlite3", fileSystemBaseDir+"/master.db")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)

	return db.Ping()
}

func createNewFileRecord(filename string, acts_id string, sha256_hash string, filepath string, claimNumber string) {
	tx, err := db.Begin()
	if err != nil {
		Warn(err.Error())
		return
	}

	now := time.Now()

	_, err = tx.Exec("INSERT INTO files (filename, acts_id, sha256_hash, created_at, filepath, is_deleted, claimNumber) VALUES (?, ?, ?, ?, ?, ?, ?)", filename, acts_id, sha256_hash, now, filepath, false, claimNumber)
	if err != nil {
		tx.Rollback()
		Warn(err.Error())
		return
	}

	tx.Commit()
	return 
}

func deleteFileRecord() {

}
