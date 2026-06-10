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

type MetaData struct {
	ClaimNumber  string `json:"claim_number"`
	ClaimantName string `json:"claimant_name"`
	DateOfInjury string `json:"date_of_injury"`
	Employer     string `json:"employer"`
	Adjuster     string `json:"adjuster"`
	Support      string `json:"support"`
	ClaimType    string `json:"claim_type"`
	Jurisdiction string `json:"jurisdiction"`
	PolicyNumber string `json:"policy_number"`
	ACTsID       string `json:"acts_id"`
}

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
func (dm *DatabaseManager) createNewFileRecord(filename, actsID, sha256Hash, filepath, claimNumber string, barcode string) {
	tx, err := dm.db.Begin()


	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		return
	}
	// Deferring Rollback ensures resources are cleaned up if any step fails.
	// If tx.Commit() succeeds, Rollback() does nothing.
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)

	query := `INSERT INTO files (filename, acts_id, sha256_hash, created_at, filepath, is_deleted, claimNumber, barcode)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(query, filename, actsID, sha256Hash, now, filepath, 0, claimNumber, barcode)
	if err != nil {
		Warn("Transaction execution failed: " + err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Warn("Failed to commit transaction: " + err.Error())
		return
	}
}

func (dm *DatabaseManager) CreateNewMetaDataRecord(metaData MetaData) {
	tx, err := dm.db.Begin()

	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		return
	}
	// Deferring Rollback ensures resources are cleaned up if any step fails.
	// If tx.Commit() succeeds, Rollback() does nothing.
	defer tx.Rollback()

	query := `INSERT INTO meta (claim_number, claimant_name, date_of_injury, employer, adjuster, support, claim_type, jurisdiction, policy_number, acts_id)
		          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(query, metaData.ClaimNumber, metaData.ClaimantName, metaData.DateOfInjury, metaData.Employer, metaData.Adjuster, metaData.Support, metaData.ClaimType, metaData.Jurisdiction, metaData.PolicyNumber, metaData.ACTsID)

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
	query := `SELECT id, filename, acts_id, sha256_hash, created_at, filepath, is_deleted, claimNumber, barcode FROM files`

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
			Barcode     string `json:"barcode"`
		}

		err := rows.Scan(&r.ID, &r.Filename, &r.ActsID, &r.Sha256Hash, &r.CreatedAt, &r.Filepath, &r.IsDeleted, &r.ClaimNumber, &r.Barcode)
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

func (dm *DatabaseManager) debugPrintMetaRecords() {
	query := `SELECT id, claim_number, claimant_name, date_of_injury, employer, adjuster, support, claim_type, jurisdiction, policy_number, acts_id FROM meta`

	rows, err := dm.db.Query(query)
	if err != nil {
		fmt.Printf("[DEBUG ERROR] Failed to query meta records: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\n--- DEBUG: ALL META RECORDS ---")
	count := 0

	for rows.Next() {
		count++
		var r struct {
			ID           int    `json:"id"`
			ClaimNumber  string `json:"claim_number"`
			ClaimantName string `json:"claimant_name"`
			DateOfInjury string `json:"date_of_injury"`
			Employer     string `json:"employer"`
			Adjuster     string `json:"adjuster"`
			Support      string `json:"support"`
			ClaimType    string `json:"claim_type"`
			Jurisdiction string `json:"jurisdiction"`
			PolicyNumber string `json:"policy_number"`
			ActsID       string `json:"acts_id"`
		}

		err := rows.Scan(
			&r.ID,
			&r.ClaimNumber,
			&r.ClaimantName,
			&r.DateOfInjury,
			&r.Employer,
			&r.Adjuster,
			&r.Support,
			&r.ClaimType,
			&r.Jurisdiction,
			&r.PolicyNumber,
			&r.ActsID,
		)
		if err != nil {
			fmt.Printf("  [ERROR] Scanning meta row %d failed: %v\n", count, err)
			continue
		}

		// Print nicely formatted JSON for readability
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("  ", "  ")
		if err := encoder.Encode(r); err != nil {
			fmt.Printf("  [ERROR] Encoding json for meta row %d: %v\n", count, err)
		}
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("[DEBUG ERROR] Error encountered during iteration: %v\n", err)
	}
}

// Migrate executes the DDL script to ensure all tables ('files' and 'meta')
// and their performance indexes exist.
func (dm *DatabaseManager) Migrate() error {
	// --- 1. CREATE FILES TABLE ---
	filesQuery := `
	CREATE TABLE IF NOT EXISTS files (
		id INT AUTO_INCREMENT PRIMARY KEY,
		filename VARCHAR(255) NOT NULL,
		acts_id VARCHAR(100) NOT NULL,
		sha256_hash CHAR(64) NOT NULL,
		created_at VARCHAR(35) NOT NULL,
		filepath TEXT NOT NULL,
		is_deleted TINYINT(1) DEFAULT 0 NOT NULL,
		claimNumber VARCHAR(100) NOT NULL,
		barcode VARCHAR(100) NOT NULL,
		INDEX idx_acts_id (acts_id) -- Needed for foreign key reference in meta
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	if _, err := dm.db.Exec(filesQuery); err != nil {
		return fmt.Errorf("failed to create files table: %w", err)
	}

	// --- 2. CREATE META TABLE ---
	metaQuery := `
	CREATE TABLE IF NOT EXISTS meta (
		id INT AUTO_INCREMENT PRIMARY KEY,
		claim_number VARCHAR(100) NOT NULL,
		claimant_name VARCHAR(255) NOT NULL,
		date_of_injury DATE NOT NULL,
		employer VARCHAR(255) NOT NULL,
		adjuster VARCHAR(255) NOT NULL,
		support VARCHAR(255) NOT NULL,
		claim_type VARCHAR(100) NOT NULL,
		jurisdiction VARCHAR(100) NOT NULL,
		policy_number VARCHAR(100) NOT NULL,
		acts_id VARCHAR(100) NOT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	if _, err := dm.db.Exec(metaQuery); err != nil {
		return fmt.Errorf("failed to create meta table: %w", err)
	}

	// --- 3. CREATE INDEXES ---
	// File Table Indexes
	dm.createIndexSafe("idx_files_filename_deleted", "files (filename, is_deleted)")
	dm.createIndexSafe("idx_files_acts_deleted", "files (acts_id, is_deleted)")

	// Meta Table Indexes (Optimized for pulling metadata by Claim Number or Acts ID)
	dm.createIndexSafe("idx_meta_claim_number", "meta (claim_number)")
	dm.createIndexSafe("idx_meta_acts_id", "meta (acts_id)")

	return nil
}

// createIndexSafe handles creating an index and gracefully ignores standard
// MySQL "Duplicate Key Name" errors (Error 1061) if it already exists.
func (dm *DatabaseManager) createIndexSafe(indexName, tableAndColumns string) {
	query := fmt.Sprintf("CREATE INDEX %s ON %s", indexName, tableAndColumns)
	_, err := dm.db.Exec(query)
	if err != nil {
		// If it's not a duplicate key error, we log it (or you can return it)
		if !isDuplicateKeyError(err) {
			fmt.Printf("[MIGRATION WARNING] Could not create index %s: %v\n", indexName, err)
		}
	}
}

// Helper function to handle duplicate index gracefully in MySQL
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL error code 1061: Duplicate key name
	return fmt.Errorf("%w", err).Error() != "" && (contains(err.Error(), "1061") || contains(err.Error(), "Duplicate key"))
}

// Simple string matcher helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
