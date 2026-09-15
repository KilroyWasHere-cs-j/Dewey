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

	"github.com/go-sql-driver/mysql"
)

// ErrMachineExists is returned by addKnownMachine when the ip is already
// registered, so callers can distinguish "already exists" from a genuine
// DB failure and respond accordingly (409 vs 500).
var ErrMachineExists = errors.New("machine already registered")

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

// newDatabaseManager initializes and verifies the database connection pool.
// Reads connection string from the required DB_DSN env var — no hardcoded
// fallback (issue #200), since a fallback credential baked into the binary
// would be the same password for every deployment that forgets to set one.
func newDatabaseManager() (*DatabaseManager, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN environment variable is required")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		Warn("Failed to open database connection: " + err.Error())
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 2. Configure connection pool settings
	db.SetMaxOpenConns(maxOpenDBConnections)
	db.SetMaxIdleConns(maxIdleDBConnections)
	db.SetConnMaxLifetime(time.Minute * time.Duration(dbConnectionTimeoutMultiplier))

	// 3. Verify the connection is actually working
	if err := db.Ping(); err != nil {
		db.Close()
		Warn("Failed to ping database: " + err.Error())
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	Debug("Database connection pool initialized successfully")

	return &DatabaseManager{db: db}, nil
}

// Close shuts down the connection pool. Only safe to call once request
// draining has finished — closing it earlier would break any in-flight
// request still waiting on a query.
func (dbm *DatabaseManager) Close() error {
	return dbm.db.Close()
}

// createNewFileRecord manages writing a new record safely within a database transaction.
// Returns the row's auto-generated id so the caller can link a MetaData
// record to this exact file via a real foreign key (issue #228), instead of
// the client-suppliable acts_id string previously used to join files and
// meta — that string had no uniqueness guarantee, so two uploads sharing an
// acts_id (or both leaving it blank) could return the wrong claimant's
// metadata.
func (dm *DatabaseManager) createNewFileRecord(entry DBEntry) (int64, error) {
	tx, err := dm.db.Begin()

	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return 0, err
	}
	// Deferring Rollback ensures resources are cleaned up if any step fails.
	// If tx.Commit() succeeds, Rollback() does nothing.
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)

	query := `INSERT INTO files (filename, acts_id, sha256_hash, created_at, filepath, is_deleted, barcode)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := tx.Exec(query, entry.Filename, entry.Act, entry.Hash, now, entry.Path, 0, entry.Barcode)
	if err != nil {
		Warn("Transaction execution failed: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return 0, err
	}

	fileID, err := result.LastInsertId()
	if err != nil {
		Warn("Failed to read inserted file id: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		Warn("Failed to commit transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return 0, err
	}

	return fileID, nil
}

// createNewMetaDataRecord inserts a claim metadata row linked to fileID via
// meta.file_id (issue #228) — a real foreign key populated from the files
// row's own auto-increment id, rather than the client-suppliable acts_id
// string previously used to join the two tables.
func (dm *DatabaseManager) createNewMetaDataRecord(metaData MetaData, fileID int64) error {
	tx, err := dm.db.Begin()

	if err != nil {
		Warn("Failed to start transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return err
	}
	// Deferring Rollback ensures resources are cleaned up if any step fails.
	// If tx.Commit() succeeds, Rollback() does nothing.
	defer tx.Rollback()

	query := `INSERT INTO meta (claim_number, claimant_name, date_of_injury, employer, adjuster, support, claim_type, jurisdiction, policy_number, acts_id, file_id)
		          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(query, metaData.ClaimNumber, metaData.ClaimantName, metaData.DateOfInjury, metaData.Employer, metaData.Adjuster, metaData.Support, metaData.ClaimType, metaData.Jurisdiction, metaData.PolicyNumber, metaData.ACTsID, fileID)

	if err != nil {
		Warn("Transaction execution failed: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return err
	}

	if err := tx.Commit(); err != nil {
		Warn("Failed to commit transaction: " + err.Error())
		atomic.AddInt64(&DBErrors, 1)
		return err
	}
	return nil
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

// pullFileRecord retrieves a file's full record as a DBEntry — filename,
// acts_id, sha256 hash, filepath, and barcode — matching the same fields
// idAndSort populates before its own OnFilter call at upload time. Used to
// rerun the filter pipeline against an already-stored file (issue #324).
func (dm *DatabaseManager) pullFileRecord(filename string) (DBEntry, error) {
	var entry DBEntry
	var barcode sql.NullString

	query := "SELECT filename, acts_id, sha256_hash, filepath, barcode FROM files WHERE filename = ? AND is_deleted = 0 LIMIT 1"
	err := dm.db.QueryRow(query, filename).Scan(&entry.Filename, &entry.Act, &entry.Hash, &entry.Path, &barcode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DBEntry{}, fmt.Errorf("no record found for filename: %s", filename)
		}
		return DBEntry{}, err
	}
	entry.Barcode = barcode

	return entry, nil
}

// pullMetaByFilename retrieves the metadata record linked to the given filename.
// Joins files and meta on files.id = meta.file_id (issue #228) — a real
// foreign key populated at insert time — rather than the client-suppliable
// acts_id string, which had no uniqueness guarantee and could return an
// arbitrary/wrong claimant's metadata when two uploads shared (or both
// omitted) the same acts_id.
func (dm *DatabaseManager) pullMetaByFilename(filename string) (MetaData, error) {
	var m MetaData

	query := `
		SELECT m.claim_number, m.claimant_name, m.date_of_injury, m.employer,
		       m.adjuster, m.support, m.claim_type, m.jurisdiction, m.policy_number, m.acts_id
		FROM files f
		JOIN meta m ON f.id = m.file_id
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

// updateFilePath changes a file's stored filepath (its location under
// fileSystemBaseDir) — e.g. for a manual move/reorganize (issue #333).
// Excludes soft-deleted records, matching pullRecordByFilename/
// pullMetaByFilename's read-side convention: a deleted file's path
// shouldn't be silently updated.
func (dm *DatabaseManager) updateFilePath(filename, newPath string) error {
	query := "UPDATE files SET filepath = ? WHERE filename = ? AND is_deleted = 0"
	result, err := dm.db.Exec(query, newPath, filename)
	if err != nil {
		return fmt.Errorf("failed to update file path: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no record found to update with filename: %s", filename)
	}

	return nil
}

// pullFileStatus retrieves a file's id, filepath, and soft-delete flag by
// filename. Unlike pullRecordByFilename, this deliberately doesn't filter
// on is_deleted = 0 — the point is to check a record's status regardless
// of whether it's currently marked deleted.
func (dm *DatabaseManager) pullFileStatus(filename string) (struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	Filepath  string `json:"filepath"`
	IsDeleted bool   `json:"is_deleted"`
}, error) {
	var s struct {
		ID        int64  `json:"id"`
		Filename  string `json:"filename"`
		Filepath  string `json:"filepath"`
		IsDeleted bool   `json:"is_deleted"`
	}

	query := "SELECT id, filename, filepath, is_deleted FROM files WHERE filename = ? LIMIT 1"
	err := dm.db.QueryRow(query, filename).Scan(&s.ID, &s.Filename, &s.Filepath, &s.IsDeleted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s, fmt.Errorf("no record found for filename: %s", filename)
		}
		return s, err
	}

	return s, nil
}

// undeleteFileRecord reverses deleteFileRecord's soft delete, clearing the
// is_deleted flag so the record is visible again to the is_deleted = 0
// filter every other read/update query in this file applies.
func (dm *DatabaseManager) undeleteFileRecord(filename string) error {
	query := "UPDATE files SET is_deleted = 0 WHERE filename = ?"
	result, err := dm.db.Exec(query, filename)
	if err != nil {
		return fmt.Errorf("failed to undelete file record: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no record found to undelete with filename: %s", filename)
	}

	return nil
}

// getDeletedFilenames returns every filename currently soft-deleted, so
// listFiles (routes.go) can filter them back out of its disk-walk results —
// deleteStoredFile leaves the physical file in place (issue #324) so
// undeleteFileRecord can actually restore it, which means disk presence
// alone can no longer be trusted to mean "still active."
// FileListEntry is one row of listActiveFiles' result — id is the cursor
// value a caller pages from, filepath is the store-relative path
// listFiles has always returned to clients (issue #456).
type FileListEntry struct {
	ID       int64
	Filepath string
}

// listActiveFiles returns active (non-deleted) files with id > afterID,
// ordered by id ascending — id-cursor pagination rather than OFFSET, so a
// concurrent insert can't shift what "the next page" means for a client
// already paging through results (issue #456). limit <= 0 means
// unbounded: every remaining active file is returned in one query, used
// when the Files page needs the complete list to search over rather than
// one page to browse.
//
// Queries the files table directly instead of walking the store
// directory (the previous approach, replaced by this issue) — the table
// already carries is_deleted and an indexed, auto-increment id perfectly
// suited to cursor pagination, and one indexed query beats a full
// filesystem walk plus a separate deleted-filenames cross-reference on
// every request.
func (dm *DatabaseManager) listActiveFiles(afterID int64, limit int) ([]FileListEntry, error) {
	query := "SELECT id, filepath FROM files WHERE is_deleted = 0 AND id > ? ORDER BY id"
	args := []any{afterID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := dm.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve active files: %w", err)
	}
	defer rows.Close()

	var files []FileListEntry
	for rows.Next() {
		var f FileListEntry
		if err := rows.Scan(&f.ID, &f.Filepath); err != nil {
			return nil, fmt.Errorf("failed to scan file row: %w", err)
		}
		files = append(files, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return files, nil
}

// debugPrintAllRecords dumps every row of the files table to stdout —
// unused by any current caller, kept as an ad hoc debugging aid.
func (dm *DatabaseManager) debugPrintAllRecords() {
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
			ID         int            `json:"id"`
			Filename   string         `json:"filename"`
			ActsID     string         `json:"acts_id"`
			Sha256Hash string         `json:"sha256_hash"`
			CreatedAt  string         `json:"created_at"`
			Filepath   string         `json:"filepath"`
			IsDeleted  int            `json:"is_deleted"`
			Barcode    sql.NullString `json:"barcode"`
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
		// MySQL error 1062: duplicate entry — ip is UNIQUE, so this means
		// the machine is already registered, not a real failure.
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrMachineExists
		}
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

// migrate executes the DDL script to ensure all tables ('files' and 'meta')
// and their performance indexes exist.
func (dm *DatabaseManager) migrate() error {
	// --- 1. CREATE FILES TABLE ---
	filesQuery := `
	CREATE TABLE IF NOT EXISTS files (
		id INT AUTO_INCREMENT PRIMARY KEY,
		filename VARCHAR(255) NOT NULL UNIQUE,
		acts_id VARCHAR(100) NOT NULL,
		sha256_hash CHAR(64) NOT NULL,
		created_at VARCHAR(35) NOT NULL,
		filepath TEXT NOT NULL,
		is_deleted TINYINT(1) DEFAULT 0 NOT NULL,
		barcode VARCHAR(100),
		INDEX idx_acts_id (acts_id), -- Needed for foreign key reference in meta
		UNIQUE INDEX idx_filepath_unique (filepath(255))
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	if _, err := dm.db.Exec(filesQuery); err != nil {
		return fmt.Errorf("failed to create files table: %w", err)
	}

	// barcode was originally NOT NULL, back when the "no barcode" sentinel
	// was the literal string "Nil" rather than a real SQL NULL (issue #336).
	// filesQuery's CREATE TABLE IF NOT EXISTS above is a no-op against a
	// table that already exists from before that change, so a deployment
	// whose mysql-data volume predates it stays stuck rejecting every
	// non-barcode upload's genuine NULL until this runs.
	dm.modifyColumnSafe("files", "barcode", "VARCHAR(100) NULL")

	// filepath previously had no uniqueness guarantee at the DB layer, so a
	// generated-filename collision (issue #367) could insert two rows
	// pointing at the same on-disk path with no error. Added via the same
	// safe-ALTER pattern as the rest of this function since filesQuery's
	// CREATE TABLE IF NOT EXISTS is a no-op against a deployment that
	// already has the files table. filepath is TEXT, so MySQL requires a
	// prefix length for the index rather than the full column.
	dm.createUniqueIndexSafe("idx_filepath_unique", "files(filepath(255))")

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

	// meta previously had no real link to files — pullMetaByFilename joined
	// on the client-suppliable acts_id string, which had no uniqueness
	// guarantee and could match the wrong claimant's row (issue #228).
	// file_id is populated at insert time from the files row's own
	// auto-increment id (see createNewMetaDataRecord), so this join is now
	// deterministic. Added via ALTER rather than in metaQuery's CREATE TABLE
	// IF NOT EXISTS above, since that statement is a no-op against a table
	// that already exists from before this fix.
	dm.addColumnSafe("meta", "file_id", "INT NULL")
	dm.addForeignKeySafe("meta", "fk_meta_file_id", "file_id", "files", "id")

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

	// Seed the loopback addresses so the server can talk to itself (e.g. the
	// test suite hitting the API from the same pod) without a manual
	// addMachine call first. INSERT IGNORE keeps this idempotent across
	// restarts and won't overwrite a label an admin already set.
	loopbackQuery := `INSERT IGNORE INTO known_machines (ip, label) VALUES ('127.0.0.1', 'localhost'), ('::1', 'localhost')`
	if _, err := dm.db.Exec(loopbackQuery); err != nil {
		return fmt.Errorf("failed to seed loopback known_machines: %w", err)
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

// createUniqueIndexSafe mirrors createIndexSafe but for a UNIQUE index.
// Gracefully ignores MySQL's "Duplicate key name" error (1061) if it
// already exists. If the target column already has duplicate values (a
// deployment carrying pre-fix data), MySQL rejects the whole ALTER with a
// "Duplicate entry" error (1062) rather than creating a partial index —
// that case falls through to the warning below instead of being swallowed,
// since it needs a human to actually deduplicate the data first.
func (dm *DatabaseManager) createUniqueIndexSafe(indexName, tableAndColumns string) {
	query := fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s", indexName, tableAndColumns)
	_, err := dm.db.Exec(query)
	if err != nil {
		if !isDuplicateKeyError(err) {
			fmt.Printf("[MIGRATION WARNING] Could not create unique index %s: %v\n", indexName, err)
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

// addColumnSafe adds a column to an existing table and gracefully ignores
// MySQL's "Duplicate column name" error (1060) if it already exists —
// mirrors createIndexSafe's re-run-safe pattern for migrations that ALTER
// a table created by an earlier version of migrate.
func (dm *DatabaseManager) addColumnSafe(table, column, definition string) {
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	_, err := dm.db.Exec(query)
	if err != nil && !isDuplicateColumnError(err) {
		fmt.Printf("[MIGRATION WARNING] Could not add column %s.%s: %v\n", table, column, err)
	}
}

// Helper function to handle a duplicate column gracefully in MySQL
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL error code 1060: Duplicate column name
	msg := err.Error()
	return strings.Contains(msg, "1060") || strings.Contains(msg, "Duplicate column")
}

// modifyColumnSafe changes an existing column's type/nullability. Unlike
// createIndexSafe/addColumnSafe, MODIFY COLUMN is naturally idempotent —
// there's no "already applied" error to swallow, since MySQL accepts
// re-running the same MODIFY against a column already in that state.
func (dm *DatabaseManager) modifyColumnSafe(table, column, definition string) {
	query := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s", table, column, definition)
	if _, err := dm.db.Exec(query); err != nil {
		fmt.Printf("[MIGRATION WARNING] Could not modify column %s.%s: %v\n", table, column, err)
	}
}

// addForeignKeySafe adds a named foreign key constraint and gracefully
// ignores MySQL's "Duplicate foreign key constraint name" error (1826) if
// it already exists.
func (dm *DatabaseManager) addForeignKeySafe(table, constraintName, column, refTable, refColumn string) {
	query := fmt.Sprintf(
		"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
		table, constraintName, column, refTable, refColumn,
	)
	_, err := dm.db.Exec(query)
	if err != nil && !isDuplicateConstraintError(err) {
		fmt.Printf("[MIGRATION WARNING] Could not add foreign key %s: %v\n", constraintName, err)
	}
}

// Helper function to handle a duplicate foreign key constraint gracefully in MySQL
func isDuplicateConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL error code 1826: Duplicate foreign key constraint name
	msg := err.Error()
	return strings.Contains(msg, "1826") || strings.Contains(msg, "Duplicate foreign key constraint")
}

