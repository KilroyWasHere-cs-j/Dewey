package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// var filters *Config
// PDF files should be treated as seperate files for each page with there own records

// Holder struct for a file database entry
type DBEntry struct {
	Filename string
	Act      string
	Hash     string
	Path     string
	Meta     string
	Barcode  sql.NullString
}

// apiError pairs the HTTP status and client-facing message a validation or
// storage failure should produce with its underlying cause, so helpers like
// checkFileSize/saveFile/deleteStoredFile/etc. can report exactly how a route
// handler should respond without needing a *gin.Context themselves.
// respondError (routes.go) is the one place that turns this into an actual
// HTTP response.
type apiError struct {
	status  int
	message string
	cause   error
}

func newAPIError(status int, message string, cause error) *apiError {
	return &apiError{status: status, message: message, cause: cause}
}

// Error returns the underlying cause's message when there is one, so
// existing "Warn(... + err.Error())" call sites keep logging the real
// failure instead of the sanitized client-facing message.
func (e *apiError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.message
}

func (e *apiError) Unwrap() error { return e.cause }

// barcodeCandidateExt matches upload extensions scanBarCode can plausibly
// decode. scanBarCode only calls image.Decode, which has PNG/JPEG decoders
// registered (backend/barcode.go) — no PDF decoder is wired in, so .pdf
// stays out of this set until that's actually implemented, and the other
// extensions listed are limited to what the upload allowlist (routes.go)
// even accepts, since anything else can never occur.
var barcodeCandidateExt = regexp.MustCompile(`(?i)\.(png|jpe?g)$`)

// fileSystemInit ensures required filesystem directories exist before server start.
//
// Behavior:
//   - Creates upload directory
//   - Creates base storage directory
//   - Creates plugin scratch directory (files.read/files.write, issue #284)
//   - Initiate the loading of filter rules
//   - Fails fast if any directory cannot be created
func fileSystemInit() {
	dirs := []string{
		uploadDir,
		fileSystemBaseDir,
		backupDir,
		pluginScratchDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			Warn("failed to create directory " + dir + ": " + err.Error())
			return
		}
	}

	Debug("filesystem initialization complete")
}

// isValidDate reports whether s matches the given layout.
// Go's reference time is "Mon Jan 2 15:04:05 MST 2006" — the layout
// string is written using that exact reference date/time, not tokens
// like "YYYY-MM-DD".
func isValidDate(s, layout string) bool {
	_, err := time.Parse(layout, s)
	return err == nil
}

func idAndSort(pm *PluginManger, dbm *DatabaseManager, path string, hash string, filename string, metaData MetaData) error {
	entry := DBEntry{
		Filename: filename,
		Act:      metaData.ACTsID,
		Hash:     hash,
		Path:     path,
		Meta:     "0000000000000000000000000000000",
		Barcode:  sql.NullString{},
	}

	isDateOfInjuryValid := isValidDate(metaData.DateOfInjury, dateLayout)
	if !isDateOfInjuryValid {
		Warn("Invalid date_of_injury: " + metaData.DateOfInjury)
		return errors.New("Invalid date_of_injury")
	}

	if barcodeCandidateExt.MatchString(filename) {
		if barcodeText, err := scanBarCode(filepath.Join(uploadDir, entry.Path)); err != nil {
			Warn("Unable to process barcodes: " + err.Error())
			atomic.AddInt64(&BarcodeFailures, 1)
			entry.Barcode = sql.NullString{}
		} else {
			Debug("Decoded barcode text to: " + barcodeText)
			atomic.AddInt64(&BarcodeSuccesses, 1)
			entry.Barcode = sql.NullString{String: barcodeText, Valid: true}
		}
	} else {
		entry.Barcode = sql.NullString{}
	}

	// OnUpload fires first — a notification hook for plugins that just want
	// to observe/log/tag the incoming file — then OnFilter decides Path.
	// Both run here (async, after the 200 OK is already sent) rather than
	// synchronously in uploadFile, so neither can veto the upload itself.
	atomic.AddInt64(&PluginRuns, 1)
	entry, err := pm.runByHook("OnUpload", entry)
	if err != nil {
		Warn("Failed to run OnUpload: " + err.Error())
		atomic.AddInt64(&PluginErrors, 1)
	}

	atomic.AddInt64(&PluginRuns, 1)
	entry, err = pm.runByHook("OnFilter", entry)
	if err != nil {
		Warn("Failed to run filter " + err.Error())
		atomic.AddInt64(&PluginErrors, 1)
	}

	// entry.Path is fully attacker-controllable by this point — OnFilter just
	// ran and any Lua plugin can set it to arbitrary text (issue #202). Reject
	// anything that resolves outside fileSystemBaseDir before it's ever used
	// for a filesystem write, rather than trusting filepath.Join alone.
	new_path, err := resolveStorePath(fileSystemBaseDir, entry.Path)
	if err != nil {
		Warn("Rejected plugin-supplied path escaping store directory: " + err.Error())
		atomic.AddInt64(&PluginErrors, 1)
		return err
	}
	err = copyFile(
		filepath.Join(uploadDir, path),
		new_path,
	)
	if err != nil {
		Warn("Failed to copy file to store: " + err.Error())
		return err
	}

	// createNewFileRecord's returned id links the metadata row to this exact
	// file via meta.file_id (issue #228), instead of the client-suppliable
	// acts_id string previously used to join files and meta. Without a valid
	// file id there's nothing correct to link a meta row to, so skip it —
	// createNewFileRecord has already logged and counted the failure.
	fileID, err := dbm.createNewFileRecord(entry)
	if err != nil {
		return err
	}
	err = dbm.createNewMetaDataRecord(metaData, fileID)
	if err != nil {
		return err
	}

	atomic.AddInt64(&FilesInStore, 1)
	atomic.AddInt64(&FileSorts, 1)
	return nil
}

// resolveStorePath joins rel onto base and confirms the result still lives
// under base, rejecting anything that escapes it (issue #202). filepath.Join
// already runs filepath.Clean, which collapses "../" segments — but Clean
// alone can't tell "this walked past base" from "this legitimately reaches
// outside", so the prefix check below is still required.
//
// The check appends a trailing separator to base before comparing so that a
// sibling directory sharing base's name as a prefix (e.g. base
// "/app/store" and rel "../store-evil" cleaning to "/app/store-evil")
// isn't mistaken for a path inside it.
func resolveStorePath(base, rel string) (string, error) {
	full := filepath.Join(base, rel)
	baseClean := filepath.Clean(base)

	if full != baseClean && !strings.HasPrefix(full, baseClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes base directory %q", rel, base)
	}

	return full, nil
}

// copyFile copies src to dst, creating dst's parent directory if needed, and
// fsyncs the destination before returning so the copy is durable on disk.
func copyFile(src, dst string) error {
	// open source
	srcFile, err := os.Open(src)
	if err != nil {
		Warn("Failed to open source file: " + err.Error())
		return err
	}
	defer srcFile.Close()

	// ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		Warn("Failed to create directory: " + err.Error())
		return err
	}

	exists, err := exists(dst)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("destination file already exists")
	}

	// create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		Warn("Failed to create directory: " + err.Error())
		return err
	}
	defer dstFile.Close()

	// copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		Warn("Failed to copy: " + err.Error())
		return err
	}

	atomic.AddInt64(&FileCopys, 1)

	// flush to disk
	return dstFile.Sync()
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// locateFile checks the cache first, then falls back to the DB record.
// Strips directory components from filename to prevent path traversal.
// Returns the absolute path to the file on disk.
func locateFile(dbm *DatabaseManager, filename string) (string, error) {
	// Prevent path traversal — only the base name is ever used
	filename = filepath.Base(filename)
	Debug("Checking cache")
	cachePath := filepath.Join(uploadDir, filename)
	if _, err := os.Stat(cachePath); err == nil {
		Debug("Found file in cache")
		atomic.AddInt64(&FileRetrievals, 1)
		return cachePath, nil
	}

	Debug("No file found in cache, searching db")

	path, err := dbm.pullRecordByFilename(filename)
	if err != nil {
		Warn("pullRecordByFilename failed " + err.Error())
		return "", err
	}
	Debug("Pulled this path from db " + path)

	fullPath := filepath.Join(fileSystemBaseDir, path)
	if _, err := os.Stat(fullPath); err != nil {
		Warn("File not found at stored path: " + fullPath)
		return "", err
	}

	atomic.AddInt64(&FileRetrievals, 1)
	return fullPath, nil
}

func checkFileSize(fileHeader *multipart.FileHeader) error {
	// Belt-and-suspenders check against the declared part size, independent
	// of whatever MaxBytesReader caught at the body-read level.
	if fileHeader.Size > maxFileSize {
		Warn(fmt.Sprintf("file too large: %d bytes", fileHeader.Size))
		uploadRejections.WithLabelValues("too_large").Inc()
		return newAPIError(http.StatusRequestEntityTooLarge, "File exceeds maximum allowed size", nil)
	}
	return nil
}

func randomSuffix(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil // n=4 -> 8 hex chars
}

// createTimestamp prefixes filename with the current Unix timestamp so
// concurrent uploads of the same name can't collide in the store.
//
// filename is stripped to its base name first (issue #403) — it comes
// straight from the client-supplied multipart filename, and without this a
// name like "../../../../etc/cron.d/pwn.txt" gets embedded as-is, letting
// filepath.Join's traversal collapse later write the file outside
// uploadDir entirely.
func createTimestamp(filename string) (string, error) {
	filename = filepath.Base(filename)

	timestamp := time.Now().Unix()
	suff, err := randomSuffix(10)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d_%s_%s", timestamp, filename, suff), nil
}

// createFileHash reads file to EOF computing its SHA-256, so callers must
// seek back to the start themselves before reading it again (see saveFile).
func createFileHash(file multipart.File) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		Warn("Failed to read file: " + err.Error())
		return "", newAPIError(http.StatusInternalServerError, "Failed to read file", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// saveFile rewinds file (it may already have been read once, e.g. by
// createFileHash) and writes it into uploadDir under filename.
func saveFile(filename string, file multipart.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		Warn("Failed to rewind file before save: " + err.Error())
		return newAPIError(http.StatusInternalServerError, "Failed to save file", err)
	}

	// Defense in depth alongside createTimestamp's stripping (issue #403):
	// reject outright if filename resolves outside uploadDir, the same
	// resolveStorePath containment check already used for the store-tier
	// copy, rather than trusting the caller sanitized it correctly.
	dst, err := resolveStorePath(uploadDir, filename)
	if err != nil {
		Warn("Rejected upload destination outside uploadDir: " + err.Error())
		uploadRejections.WithLabelValues("path_traversal").Inc()
		return newAPIError(http.StatusBadRequest, "Invalid filename", err)
	}

	exists, err := exists(dst)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("destination file already exists")
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		Warn("Failed to create destination file: " + err.Error())
		return newAPIError(http.StatusInternalServerError, "Failed to save file", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		Warn("Failed to save file: " + err.Error())
		return newAPIError(http.StatusInternalServerError, "Failed to save file", err)
	}
	atomic.AddInt64(&UploadsSinceLastTick, 1)
	return nil
}

// deleteStoredFile soft-deletes filename's DB record and removes its cache
// copy, after giving OnDelete plugins a chance to veto the deletion
// synchronously (unlike OnFilter/OnUpload, no response has been sent yet).
// The on-disk store copy is deliberately left in place (issue #324) so
// undeleteFileRecord can restore an actual file, not just a DB pointer with
// nothing behind it — listFiles (routes.go) filters is_deleted=1 filenames
// out of its disk-walk results so they don't reappear in the file list.
func deleteStoredFile(filename string, pm *PluginManger, dbm *DatabaseManager) error {
	storeRelPath, err := dbm.pullRecordByFilename(filename)
	if err != nil {
		Warn("file record not found: " + err.Error())
		return newAPIError(http.StatusNotFound, "File not found", err)
	}

	// OnDelete runs synchronously, before anything is actually removed —
	// unlike OnFilter/OnUpload, no response has been sent yet at this
	// point, so a plugin calling error(...) genuinely vetoes the deletion
	// instead of racing an already-sent 200 OK.
	entry := DBEntry{Filename: filename, Path: storeRelPath}
	if _, err := pm.runByHook("OnDelete", entry); err != nil {
		Warn("deletion vetoed by plugin: " + err.Error())
		return newAPIError(http.StatusForbidden, "Deletion blocked by plugin: "+err.Error(), err)
	}

	// Cache copy may already be gone (dumpCache runs independently) — not an error either way.
	cachePath := filepath.Join(uploadDir, filename)
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		Warn("failed to delete cached file copy: " + err.Error())
	}

	if err := dbm.deleteFileRecord(filename); err != nil {
		Warn("failed to mark file record deleted: " + err.Error())
		return newAPIError(http.StatusInternalServerError, "Failed to delete file", err)
	}

	Debug("file deleted: " + filename)
	atomic.AddInt64(&FileDeletions, 1)
	atomic.AddInt64(&FilesInStore, -1)
	return nil
}

// openFile opens the multipart file behind fileHeader, wrapping any failure
// in an apiError so the caller can respond without inspecting the cause.
func openFile(fileHeader *multipart.FileHeader) (multipart.File, error) {
	file, err := fileHeader.Open()
	if err != nil {
		Warn("Failed to open file: " + err.Error())
		return nil, newAPIError(http.StatusInternalServerError, "Failed to open file", err)
	}
	return file, nil
}

// moveStoredFile copies the file on disk to newPath and updates its DB path
// record to match, so the two stay in sync. Refuses to move a record that's
// currently soft-deleted (issue #324) — a deleted file staying deleted is
// assumed intentional, so a move shouldn't silently resurrect it.
func moveStoredFile(currentPath string, newPath string, dbm *DatabaseManager) error {
	filename := filepath.Base(currentPath)

	status, err := dbm.pullFileStatus(filename)
	if err != nil {
		Warn("Failed to look up file status before move: " + err.Error())
		return err
	}
	if status.IsDeleted {
		return fmt.Errorf("cannot move %q: record is marked deleted", filename)
	}

	old_path, err := resolveStorePath(fileSystemBaseDir, currentPath)
	if err != nil {
		Warn("Rejected supplied path escaping store directory: " + err.Error())
		atomic.AddInt64(&PluginErrors, 1)
		return err
	}

	new_path, err := resolveStorePath(fileSystemBaseDir, newPath)
	if err != nil {
		Warn("Rejected supplied path escaping store directory: " + err.Error())
		atomic.AddInt64(&PluginErrors, 1)
		return err
	}

	if err := copyFile(old_path, new_path); err != nil {
		Warn("Failed to copy file to store: " + err.Error())
		return err
	}

	// Update the DB pointer before removing the old copy — if updateFilePath
	// fails, this leaves a harmless duplicate on disk rather than a DB
	// record pointing at a file that no longer exists anywhere.
	if err := dbm.updateFilePath(filename, newPath); err != nil {
		Warn("Failed to update file path: " + err.Error())
		return err
	}

	// Best-effort: the move already succeeded from the DB/caller's
	// perspective at this point, so a cleanup failure here is logged, not
	// returned as an error.
	if err := os.Remove(old_path); err != nil {
		Warn("Failed to remove old file after move: " + err.Error())
	}

	return nil
}

// rerunFilters re-invokes the OnFilter hook against an already-stored
// file's existing record, then relocates it to whatever Path the filter
// pipeline decides this time — useful for reclassifying a file after
// plugin filter logic changes, without re-uploading it (issue #324).
// Deliberately skips OnUpload: that hook is a one-time upload notification,
// not part of classification, so replaying it here wouldn't make sense.
func rerunFilters(filename string, pm *PluginManger, dbm *DatabaseManager) error {
	entry, err := dbm.pullFileRecord(filename)
	if err != nil {
		Warn("Failed to look up file record before rerunning filters: " + err.Error())
		return err
	}
	oldPath := entry.Path

	atomic.AddInt64(&PluginRuns, 1)
	entry, err = pm.runByHook("OnFilter", entry)
	if err != nil {
		Warn("Failed to rerun filter: " + err.Error())
		atomic.AddInt64(&PluginErrors, 1)
		return err
	}

	// Filter pipeline decided the file already lives where it should.
	if entry.Path == oldPath {
		return nil
	}

	return moveStoredFile(oldPath, entry.Path, dbm)
}

func validateMetaData(m MetaData) error {
	if strings.TrimSpace(m.DateOfInjury) == "" {
		return errors.New("date_of_injury is required")
	}
	// same for any other NOT NULL column fed from PostForm
	return nil
}

func viewFile(baseDir string, logname string) (string, error) {
	filepath := filepath.Join(baseDir, logname)
	file, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	return string(file), nil
}

func viewLogDir() ([]string, error) {
	files, err := os.ReadDir("/app/logs")
	if err != nil {
		return nil, err
	}

	var fileList []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileList = append(fileList, file.Name())
	}

	return fileList, nil
}

