package main

import (
	"fmt"
	"time"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

type metadate struct {
	claimantName string
	dateofInjury string
	employer     string
	adjuster     string
	support      string
	claimType    string
	jurisdiction string
	policy       string
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

type DatabaseManager struct {
	db *sql.DB
}

func NewDatabaseManager() (*DatabaseManager, error) {
	// 1. Open the database connection pool
	db, err := sql.Open("mysql", "user:password@tcp(127.0.0.1:3306)/dbname")
	if err != nil {
		Fatal("Failed to open database connection: " + err.Error()) // Remove fatal later
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 2. Configure connection pool settings (Highly Recommended)
	db.SetMaxOpenConns(maxOpenDBConnections)
	db.SetMaxIdleConns(maxIdleDBConnections)
	db.SetConnMaxLifetime(time.Minute * dbConnectionTimeoutMultiplier)

	// 3. Verify the connection is actually working
	if err := db.Ping(); err != nil {
		db.Close() // Clean up if the ping fails
		Fatal("Failed to ping database: " + err.Error()) // Remove fatal later
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DatabaseManager{
		db: db,
	}, nil
}

func InitDB() error {
	return nil
}

func createNewFileRecord(filename string, acts_id string, sha256_hash string, filepath string, claimNumber string) {
}

func pullRecordByFilename(fileName string) (string, error) {
	return "", nil
}

func pullRecordByACTsNumber(acts_no string) (string, error) {
	return "", nil
}
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Dummy logging functions to mimic yours without breaking compilation
func Warn(msg string)  { fmt.Println("WARN:", msg) }
func Error(msg string) { fmt.Println("ERROR:", msg) }

// Provided placeholder configuration string
var fileSystemBaseDir = "."

type metadate struct {
	claimantName string
	dateofInjury string
	employer     string
	adjuster     string
	support      string
	claimType    string
	jurisdiction string
	policy       string
}

type FileRecord struct {
	Filename    string
	ActsID      string
	SHA256Hash  string
	CreatedAt   string
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
	// Correct setting for basic SQLite setups to prevent concurrent lock issues
	db.SetMaxOpenConns(1)

	return db.Ping()
}

// createNewFileRecord manages writing a new record safely within a database transaction
func createNewFileRecord(filename string, acts_id string, sha256_hash string, filepath string, claimNumber string) {
	tx, err := db.Begin()
	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		return
	}

	now := time.Now().Format(time.RFC3339) // Explicitly format time as string for SQLite consistency

	_, err = tx.Exec(
		"INSERT INTO files (filename, acts_id, sha256_hash, created_at, filepath, is_deleted, claimNumber) VALUES (?, ?, ?, ?, ?, ?, ?)",
		filename, acts_id, sha256_hash, now, filepath, 0, claimNumber,
	)
	if err != nil {
		tx.Rollback()
		Warn("Transaction failed, rolled back: " + err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Warn("Failed to commit transaction: " + err.Error())
	}
}

// pullRecordByFilename pulls a single filepath. Used db.QueryRow to safely avoid array indexing panics.
func pullRecordByFilename(fileName string) (string, error) {
	var filepath string

	// Explicitly querying the column we want instead of relying on SELECT *
	query := "SELECT filepath FROM files WHERE filename = ? AND is_deleted = 0 LIMIT 1"
	err := db.QueryRow(query, fileName).Scan(&filepath)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no record found for filename: %s", fileName)
		}
		return "", err
	}

	return filepath, nil
}

// pullRecordByACTsNumber pulls a single ACTS ID.
func pullRecordByACTsNumber(acts_no string) (string, error) {
	var acts_id string

	query := "SELECT acts_id FROM files WHERE acts_id = ? AND is_deleted = 0 LIMIT 1"
	err := db.QueryRow(query, acts_no).Scan(&acts_id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no record found for acts_id: %s", acts_no)
		}
		return "", err
	}

	return acts_id, nil
}

// deleteFileRecord implements a soft delete pattern by updating the flag.
// In file tracking structures, soft deletion is highly preferred over hard deletions.
func deleteFileRecord(filename string) error {
	query := "UPDATE files SET is_deleted = 1 WHERE filename = ?"
	result, err := db.Exec(query, filename)
	if err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no record found to delete with filename: %s", filename)
	}

	return nil
}
