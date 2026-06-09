package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // Assuming MySQL based on your connection string
)

// DatabaseManager encapsulates the SQL database connection pool.
type DatabaseManager struct {
	db *sql.DB
}

// NewDatabaseManager initializes and verifies the database connection pool.
func NewDatabaseManager() (*DatabaseManager, error) {
	// 1. Open the database connection pool
	db, err := sql.Open("mysql", "root:dewey@tcp(127.0.0.1:3306)/deweyRecords")
	if err != nil {
		Fatal("Failed to open database connection: " + err.Error()) // Remove fatal later
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 2. Configure connection pool settings
	db.SetMaxOpenConns(maxOpenDBConnections)
	db.SetMaxIdleConns(maxIdleDBConnections)
	db.SetConnMaxLifetime(time.Minute * dbConnectionTimeoutMultiplier)

	// 3. Verify the connection is actually working
	if err := db.Ping(); err != nil {
		db.Close()                                       // Clean up if the ping fails
		Fatal("Failed to ping database: " + err.Error()) // Remove fatal later
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	Debug("Database connection pool initialized successfully")

	return &DatabaseManager{db: db}, nil
}

// createNewFileRecord manages writing a new record safely within a database transaction.
func (dm *DatabaseManager) createNewFileRecord(filename, actsID, sha256Hash, filepath, claimNumber string) {
	tx, err := dm.db.Begin()
	dm.DebugPrintAllRecords()
	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		return
	}
	// Deferring Rollback ensures resources are cleaned up if any step fails.
	// If tx.Commit() succeeds, Rollback() does nothing.
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)

	query := `INSERT INTO files (filename, acts_id, sha256_hash, created_at, filepath, is_deleted, claimNumber)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(query, filename, actsID, sha256Hash, now, filepath, 0, claimNumber)
	if err != nil {
		Warn("Transaction execution failed: " + err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Warn("Failed to commit transaction: " + err.Error())
		return
	}
}

// pullRecordByFilename pulls a single filepath.
func (dm *DatabaseManager) pullRecordByFilename(fileName string) (string, error) {
	var filepath string

	query := "SELECT filepath FROM files WHERE filename = ? AND is_deleted = 0 LIMIT 1"
	err := dm.db.QueryRow(query, fileName).Scan(&filepath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no record found for filename: %s", fileName)
		}
		return "", err
	}

	return filepath, nil
}

// pullRecordByACTsNumber pulls a single ACTS ID.
func (dm *DatabaseManager) pullRecordByACTsNumber(actsNo string) (string, error) {
	var actsID string

	query := "SELECT acts_id FROM files WHERE acts_id = ? AND is_deleted = 0 LIMIT 1"
	err := dm.db.QueryRow(query, actsNo).Scan(&actsID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no record found for acts_id: %s", actsNo)
		}
		return "", err
	}

	return actsID, nil
}

// deleteFileRecord implements a soft delete pattern by updating the flag.
func (dm *DatabaseManager) deleteFileRecord(filename string) error {
	query := "UPDATE files SET is_deleted = 1 WHERE filename = ?"
	result, err := dm.db.Exec(query, filename)
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

func (dm *DatabaseManager) DebugPrintAllRecords() {
	query := `SELECT id, filename, acts_id, sha256_hash, created_at, filepath, is_deleted, claimNumber FROM files`

	rows, err := dm.db.Query(query)
	if err != nil {
		fmt.Printf("[DEBUG ERROR] Failed to query records: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\n--- DEBUG: ALL FILE RECORDS ---")
	count := 0

	for rows.Next() {
		count++
		var r struct {
			ID          int    `json:"id"`
			Filename    string `json:"filename"`
			ActsID      string `json:"acts_id"`
			Sha256Hash  string `json:"sha256_hash"`
			CreatedAt   string `json:"created_at"`
			Filepath    string `json:"filepath"`
			IsDeleted   int    `json:"is_deleted"`
			ClaimNumber string `json:"claim_number"`
		}

		err := rows.Scan(&r.ID, &r.Filename, &r.ActsID, &r.Sha256Hash, &r.CreatedAt, &r.Filepath, &r.IsDeleted, &r.ClaimNumber)
		if err != nil {
			fmt.Printf("  [ERROR] Scanning row %d failed: %v\n", count, err)
			continue
		}

		// Print nicely formatted JSON for readability
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("  ", "  ")
		if err := encoder.Encode(r); err != nil {
			fmt.Printf("  [ERROR] Encoding json for row %d: %v\n", count, err)
		}
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("[DEBUG ERROR] Row iteration error: %v\n", err)
	}

	fmt.Printf("--- END DEBUG: TOTAL RECORDS FOUND: %d ---\n\n", count)
}
