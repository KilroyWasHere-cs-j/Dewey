package main

import (
	"time"
	// "fmt"
	"strings"

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

func pullRecordByFilename(filename string) ([]FileRecord, error) {
	rows, err := db.Query("SELECT * FROM files WHERE filename = ?", filename)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []FileRecord

	for rows.Next() {
		var rec FileRecord

		err := rows.Scan(
			&rec.Filename,
			&rec.ActsID,
			&rec.SHA256Hash,
			&rec.CreatedAt,
			&rec.Filepath,
			&rec.IsDeleted,
			&rec.ClaimNumber,
		)
		if err != nil {
			return nil, err
		}

		results = append(results, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func deleteFileRecord() {

}





func parseRow(input string) map[string]interface{} {
	result := make(map[string]interface{})

	// Split by tabs
	pairs := strings.Split(input, "\t")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split only on first colon
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Handle <nil>
		if val == "<nil>" {
			result[key] = nil
		} else {
			result[key] = val
		}
	}

	return result
}
