package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
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
	Barcode  string
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
//   - Initiate the loading of filter rules
//   - Fails fast if any directory cannot be created
func fileSystemInit() {
	dirs := []string{
		uploadDir,
		fileSystemBaseDir,
		backupDir,
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
// maxConcurrentPostProcessing (issue #217).
var postProcessingSem = make(chan struct{}, maxConcurrentPostProcessing)

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
		Barcode:  "barcode",                         // Placeholder, should be determined by barcode scanning
	}
	if barcodeCandidateExt.MatchString(filename) {
		if barcodeText, err := scanBarCode(filepath.Join(uploadDir, entry.Path)); err != nil {
			Warn("Unable to process barcodes: " + err.Error())
			atomic.AddInt64(&BarcodeFailures, 1)
			entry.Barcode = "Nil"
		} else {
			Debug("Decoded barcode text to: " + barcodeText)
			atomic.AddInt64(&BarcodeSuccesses, 1)
			entry.Barcode = barcodeText
		}
	} else {
		entry.Barcode = "Nil"
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

	new_path := filepath.Join(fileSystemBaseDir, entry.Path)
	err = CopyFile(
		filepath.Join(uploadDir, path),
		new_path,
	)
	if err != nil {
		Warn("Failed to copy file to store: " + err.Error())
		return
	}

	dbm.createNewFileRecord(entry)
	dbm.CreateNewMetaDataRecord(metaData)

	atomic.AddInt64(&FilesInStore, 1)
	atomic.AddInt64(&FileSorts, 1)
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
