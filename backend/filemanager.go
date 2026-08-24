package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
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

// postProcessingSem bounds concurrent idAndSort work to
// maxConcurrentPostProcessing (issue #217). Sized in load() (consts.go)
// once the config file has actually been read, not here — see the
// comment there for why a static initializer referencing
// maxConcurrentPostProcessing directly would be wrong now that it's a
// runtime-loaded var instead of a compile-time const.
var postProcessingSem chan struct{}

// queueIdAndSort runs idAndSort in a background goroutine without blocking
// the caller (the upload response is already sent before this is called),
// but bounds how many run at once via postProcessingSem — excess uploads
// queue behind the semaphore instead of every one running its Lua filter
// plugins and disk copy concurrently.
func queueIdAndSort(pm *PluginManger, dbm *DatabaseManager, path string, hash string, filename string, metaData MetaData) {
	go func() {
		postProcessingSem <- struct{}{}
		defer func() { <-postProcessingSem }()
		defer func() {
			if r := recover(); r != nil {
				Warn("Recovered from panic in post-processing goroutine: " + fmt.Sprintf("%v", r))
			}
		}()
		idAndSort(pm, dbm, path, hash, filename, metaData)
	}()
}

func idAndSort(pm *PluginManger, dbm *DatabaseManager, path string, hash string, filename string, metaData MetaData) {
	entry := DBEntry{
		Filename: filename,
		Act:      metaData.ACTsID,
		Hash:     hash,
		Path:     path,
		Meta:     "0000000000000000000000000000000", // Placeholder, should be determined by filter rules
		Barcode:  sql.NullString{},                  // Placeholder, should be determined by barcode scanning
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
	entry, err := pm.RunByHook("OnUpload", entry)
	if err != nil {
		Warn("Failed to run OnUpload: " + err.Error())
		atomic.AddInt64(&PluginErrors, 1)
	}

	atomic.AddInt64(&PluginRuns, 1)
	entry, err = pm.RunByHook("OnFilter", entry)
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
		return
	}
	err = CopyFile(
		filepath.Join(uploadDir, path),
		new_path,
	)
	if err != nil {
		Warn("Failed to copy file to store: " + err.Error())
		return
	}

	// createNewFileRecord's returned id links the metadata row to this exact
	// file via meta.file_id (issue #228), instead of the client-suppliable
	// acts_id string previously used to join files and meta. Without a valid
	// file id there's nothing correct to link a meta row to, so skip it —
	// createNewFileRecord has already logged and counted the failure.
	fileID, err := dbm.createNewFileRecord(entry)
	if err != nil {
		return
	}
	dbm.CreateNewMetaDataRecord(metaData, fileID)

	atomic.AddInt64(&FilesInStore, 1)
	atomic.AddInt64(&FileSorts, 1)
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

func CopyFile(src, dst string) error {
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

func checkFileSize(fileHeader *multipart.FileHeader, c *gin.Context) bool {
	// Belt-and-suspenders check against the declared part size, independent
	// of whatever MaxBytesReader caught at the body-read level.
	if fileHeader.Size > maxFileSize {
		Warn(fmt.Sprintf("file too large: %d bytes", fileHeader.Size))
		uploadRejections.WithLabelValues("too_large").Inc()
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "File exceeds maximum allowed size",
		})
		return false
	}
	return true
}

func CreateTimestamp(filename string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%d_%s", timestamp, filename)
}

func CreateFileHash(file multipart.File, c *gin.Context) (error, string) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		Warn("Failed to read file: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read file",
		})
		return err, ""
	}

	return nil, hex.EncodeToString(hasher.Sum(nil))
}

func SaveFile(filename string, file multipart.File, c *gin.Context) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		Warn("Failed to rewind file before save: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file",
		})
		return err
	}

	dst := filepath.Join(uploadDir, filename)
	dstFile, err := os.Create(dst)
	if err != nil {
		Warn("Failed to create destination file: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file",
		})
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		Warn("Failed to save file: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file",
		})
		return err
	}
	atomic.AddInt64(&UploadsSinceLastTick, 1)
	return nil
}

func DeleteFile(filename string, c *gin.Context) error {
	pm := c.MustGet("plugins").(*PluginManger)
	dbm := c.MustGet("db").(*DatabaseManager)

	storeRelPath, err := dbm.pullRecordByFilename(filename)
	if err != nil {
		Warn("file record not found: " + err.Error())
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found",
		})
		return err
	}

	// OnDelete runs synchronously, before anything is actually removed —
	// unlike OnFilter/OnUpload, no response has been sent yet at this
	// point, so a plugin calling error(...) genuinely vetoes the deletion
	// instead of racing an already-sent 200 OK.
	entry := DBEntry{Filename: filename, Path: storeRelPath}
	if _, err := pm.RunByHook("OnDelete", entry); err != nil {
		Warn("deletion vetoed by plugin: " + err.Error())
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Deletion blocked by plugin: " + err.Error(),
		})
		return err
	}

	storePath := filepath.Join(fileSystemBaseDir, storeRelPath)
	if err := os.Remove(storePath); err != nil && !os.IsNotExist(err) {
		Warn("failed to delete file from store: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete file",
		})
		return err
	}

	// Cache copy may already be gone (dumpCache runs independently) — not an error either way.
	cachePath := filepath.Join(uploadDir, filename)
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		Warn("failed to delete cached file copy: " + err.Error())
	}

	if err := dbm.deleteFileRecord(filename); err != nil {
		Warn("failed to mark file record deleted: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete file",
		})
		return err
	}

	Debug("file deleted: " + filename)
	atomic.AddInt64(&FileDeletions, 1)
	atomic.AddInt64(&FilesInStore, -1)
	return nil
}

func OpenFile(fileHeader *multipart.FileHeader, c *gin.Context) (error, multipart.File) {
	file, err := fileHeader.Open()
	if err != nil {
		Warn("Failed to open file: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open file",
		})
		return err, nil
	}
	return nil, file
}
