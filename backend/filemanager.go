package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func idAndSort(pm *PluginManager, dbm *DatabaseManager, path string, hash string, filename string, metaData MetaData) {
	entry := DBEntry{
		Filename: filename,
		Act:      metaData.ACTsID,
		Hash:     hash,
		Path:     path,
		Meta:     "0000000000000000000000000000000", // Placeholder, should be determined by filter rules
		Barcode:  "barcode",                         // Placeholder, should be determined by barcode scanning
	}

	if barcodeText, err := scanBarCode(filepath.Join(uploadDir, entry.Path)); err != nil {
		Warn("Unable to process barcodes: " + err.Error())
		atomic.AddInt64(&BarcodeFailures, 1)
		entry.Barcode = "Nil"
	} else {
		Debug("Decoded barcode text to: " + barcodeText)
		atomic.AddInt64(&BarcodeSuccesses, 1)
		entry.Barcode = barcodeText
	}

	runFilter := pm.RunPlugins(Filter)
	atomic.AddInt64(&PluginRuns, 1)
	entry, err := runFilter(entry)
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

// searchAndReturn validates that a file exists and returns its absolute path.
//
// Returns:
//   - string: absolute file path
//   - error: if file does not exist or path is invalid
func searchAndReturn(dbm *DatabaseManager, filename string, pullMeta string) (string, error) {

	// Strip any directory components to prevent path traversal
	filename = filepath.Base(filename)

	if pullMeta == "false" {
		Debug("Checking cache")
		files, err := os.ReadDir(uploadDir)
		if err != nil {
			Warn("During file retrieval os.ReadDir() encountered " + err.Error())
		}

		for _, file := range files {
			if file.Name() == filename {
				Debug("Found file in cache")

				atomic.AddInt64(&FileRetrievals, 1)
				return filepath.Join(uploadDir, file.Name()), nil
			}
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

		FileRetrievals++
		return fullPath, nil

	} else if pullMeta == "true" {
		Debug("Would sql search and pull meta")
		return "", fmt.Errorf("metadata retrieval not implemented")
	} else {
		Debug("Unknown meta flag")
		return "", fmt.Errorf("unknown meta flag: %s", pullMeta)
	}
}
