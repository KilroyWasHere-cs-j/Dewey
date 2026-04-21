package main

import (
	"time"
	// "fmt"
	// "strings"

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

type FileRecord struct {
	Filename    string
	ActsID      string
	SHA256Hash  string
	CreatedAt   string // or time.Time if you parse it
	Filepath    string
	IsDeleted   int
	ClaimNumber *string
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

func pullRecordByFilename(fileName string) (string, error) {
	rows, err := db.Query("SELECT * FROM files WHERE filename = ?", fileName)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var filepaths []string
	for rows.Next() {
		var id int
		var filename string
		var acts_id string
		var sha256_hash string
		var created_at string
		var filepath string
		var is_deleted string
		var ClaimNumber string
		err = rows.Scan(&id, &filename, &acts_id, &sha256_hash, &created_at, &filepath, &is_deleted, &ClaimNumber)
		if err != nil {
			Fatal(err.Error())
		}
		filepaths = append(filepaths, filepath)
	}
	if err = rows.Err(); err != nil {
		Fatal(err.Error())
	}
	return filepaths[0], nil
}

func deleteFileRecord() {

}

