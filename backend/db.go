package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
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
// Reads connection string from DB_DSN env var, falls back to local dev default.
func NewDatabaseManager() (*DatabaseManager, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:dewey@tcp(127.0.0.1:3306)/deweyRecords"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		Warn("Failed to open database connection: " + err.Error())
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 2. Configure connection pool settings
	db.SetMaxOpenConns(maxOpenDBConnections)
	db.SetMaxIdleConns(maxIdleDBConnections)
	db.SetConnMaxLifetime(time.Minute * dbConnectionTimeoutMultiplier)

	// 3. Verify the connection is actually working
	if err := db.Ping(); err != nil {
		db.Close()
		Warn("Failed to ping database: " + err.Error())
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	Debug("Database connection pool initialized successfully")

	return &DatabaseManager{db: db}, nil
}

// createNewFileRecord manages writing a new record safely within a database transaction.
func (dm *DatabaseManager) createNewFileRecord(entry DBEntry) {
	tx, err := dm.db.Begin()

	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return
	}
	// Deferring Rollback ensures resources are cleaned up if any step fails.
	// If tx.Commit() succeeds, Rollback() does nothing.
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)

	query := `INSERT INTO files (filename, acts_id, sha256_hash, created_at, filepath, is_deleted, barcode)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(query, entry.Filename, entry.Act, entry.Hash, now, entry.Path, 0, entry.Barcode)
	if err != nil {
		Warn("Transaction execution failed: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return
	}

	if err := tx.Commit(); err != nil {
		Warn("Failed to commit transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return
	}
}

func (dm *DatabaseManager) CreateNewMetaDataRecord(metaData MetaData) {
	tx, err := dm.db.Begin()

	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
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
		atomic.AddInt64(&DBErrors, 1)
		return
	}

	if err := tx.Commit(); err != nil {
		Warn("Failed to commit transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
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

// pullMetaByFilename retrieves the metadata record linked to the given filename.
// Joins files and meta on acts_id so a single query resolves both tables.
func (dm *DatabaseManager) pullMetaByFilename(filename string) (MetaData, error) {
	var m MetaData

	query := `
		SELECT m.claim_number, m.claimant_name, m.date_of_injury, m.employer,
		       m.adjuster, m.support, m.claim_type, m.jurisdiction, m.policy_number, m.acts_id
		FROM files f
		JOIN meta m ON f.acts_id = m.acts_id
		WHERE f.filename = ? AND f.is_deleted = 0
		LIMIT 1`

	err := dm.db.QueryRow(query, filename).Scan(
		&m.ClaimNumber, &m.ClaimantName, &m.DateOfInjury, &m.Employer,
		&m.Adjuster, &m.Support, &m.ClaimType, &m.Jurisdiction, &m.PolicyNumber, &m.ACTsID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MetaData{}, fmt.Errorf("no metadata found for filename: %s", filename)
		}
		return MetaData{}, err
	}

	return m, nil
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
	query := `SELECT id, filename, acts_id, sha256_hash, created_at, filepath, is_deleted, barcode FROM files`

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
			ID         int    `json:"id"`
			Filename   string `json:"filename"`
			ActsID     string `json:"acts_id"`
			Sha256Hash string `json:"sha256_hash"`
			CreatedAt  string `json:"created_at"`
			Filepath   string `json:"filepath"`
			IsDeleted  int    `json:"is_deleted"`
			Barcode    string `json:"barcode"`
		}

		err := rows.Scan(&r.ID, &r.Filename, &r.ActsID, &r.Sha256Hash, &r.CreatedAt, &r.Filepath, &r.IsDeleted, &r.Barcode)
		if err != nil {
			Warn(fmt.Sprintf("  [ERROR] Scanning row %d failed: %v\n", count, err))
			continue
		}

		// Print nicely formatted JSON for readability
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("  ", "  ")
		if err := encoder.Encode(r); err != nil {
			Warn(fmt.Sprintf("  [ERROR] Encoding json for row %d: %v\n", count, err))
		}
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("[DEBUG ERROR] Row iteration error: %v\n", err)
	}

	fmt.Printf("--- END DEBUG: TOTAL RECORDS FOUND: %d ---\n\n", count)
}

// checkKnownMachine looks up ip in the known_machines allowlist and returns
// its label if registered. sql.ErrNoRows means the IP isn't authorized to
// talk to this server.
func (dm *DatabaseManager) checkKnownMachine(ip string) (string, error) {
	var label string

	query := `SELECT label FROM known_machines WHERE ip = ?`
	err := dm.db.QueryRow(query, ip).Scan(&label)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("machine not registered: %s", ip)
		}
		return "", err
	}

	return label, nil
}

func (dm *DatabaseManager) logMachineIP(ip string) error {
	query := `UPDATE known_machines SET last_seen_at = NOW() WHERE ip = ?`
	_, err := dm.db.Exec(query, ip)
	if err != nil {
		return fmt.Errorf("failed to log machine IP: %w", err)
	}
	return nil
}

func (dm *DatabaseManager) addKnownMachine(ip, label string) error {
	query := `INSERT INTO known_machines (ip, label) VALUES (?, ?)`
	_, err := dm.db.Exec(query, ip, label)
	if err != nil {
		return fmt.Errorf("failed to add known machine: %w", err)
	}
	return nil
}

func (dm *DatabaseManager) removeKnownMachine(ip string) error {
	query := `DELETE FROM known_machines WHERE ip = ?`
	_, err := dm.db.Exec(query, ip)
	if err != nil {
		return fmt.Errorf("failed to remove known machine: %w", err)
	}
	return nil
}

func (dm *DatabaseManager) getKnownMachines() ([]struct {
	IP       string  `json:"ip"`
	Label    string  `json:"label"`
	AddedAt  string  `json:"added_at"`
	LastSeen *string `json:"last_seen_at"`
}, error) {
	query := `SELECT ip, label, added_at, last_seen_at FROM known_machines`
	rows, err := dm.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve known machines: %w", err)
	}
	defer rows.Close()

	var machines []struct {
		IP       string  `json:"ip"`
		Label    string  `json:"label"`
		AddedAt  string  `json:"added_at"`
		LastSeen *string `json:"last_seen_at"`
	}

	for rows.Next() {
		var m struct {
			IP       string  `json:"ip"`
			Label    string  `json:"label"`
			AddedAt  string  `json:"added_at"`
			LastSeen *string `json:"last_seen_at"`
		}
		// last_seen_at is nullable — a machine that's been added but never
		// yet seen has no value, so scan through sql.NullString rather than
		// straight into a string, which errors out on NULL.
		var lastSeen sql.NullString
		if err := rows.Scan(&m.IP, &m.Label, &m.AddedAt, &lastSeen); err != nil {
			return nil, fmt.Errorf("failed to scan known machine row: %w", err)
		}
		if lastSeen.Valid {
			m.LastSeen = &lastSeen.String
		}
		machines = append(machines, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return machines, nil
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

	machinesQuery := `
	CREATE TABLE IF NOT EXISTS known_machines (
      id INT AUTO_INCREMENT PRIMARY KEY,
      ip VARCHAR(45) NOT NULL UNIQUE,
      label VARCHAR(255) NOT NULL,
      added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
      last_seen_at DATETIME NULL
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	if _, err := dm.db.Exec(machinesQuery); err != nil {
		return fmt.Errorf("failed to create known_machines table: %w", err)
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
	msg := err.Error()
	return strings.Contains(msg, "1061") || strings.Contains(msg, "Duplicate key")
}
