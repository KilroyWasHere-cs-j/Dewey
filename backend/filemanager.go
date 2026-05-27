package main

import (
	"io"
	"os"
	"path/filepath"
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
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			Warn("failed to create directory " + dir + ": " + err.Error())
			return
		}
	}

	Debug("filesystem initialization complete")
}

func idAndSort(pm *PluginManager, path string, hash string, filename string) {
	FileSorts++ // Move this to the end of the function after all checks are done

	entry := DBEntry{
		Filename: filename,
		Act:      "ACTS_00N", // Placeholder, should be determined by filter rules
		Hash:     hash,
		Path:     path,
		Meta:     "11111111111111111111111111111111", // Placeholder, should be determined by filter rules
	}

	runFilter := pm.RunPlugins("filter")
	entry, err := runFilter(entry)
	if err != nil {
		Warn("Failed to run filter " + err.Error())
	}
	new_path := filepath.Join(fileSystemBaseDir, entry.Path, path)
	err = CopyFile(
		filepath.Join(uploadDir, path),
		new_path,
	)
	entry.Hash = hash // Overide the hash value
	createNewFileRecord(entry.Filename, entry.Act, entry.Hash, new_path, entry.Meta)
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

	FileCopys++

	// flush to disk
	return dstFile.Sync()
}

// searchAndReturn validates that a file exists and returns its absolute path.
//
// Behavior:
//   - Ensures the path exists
//   - Ensures it is not a directory
//   - Converts to absolute path
//
// Security note:
//   - Does NOT currently enforce uploadDir containment (important)
//
// Returns:
//   - string: absolute file path
//   - error: if file does not exist or path is invalid
func searchAndReturn(filename string, pullMeta string) string {

	/// Search cache and return if found

	if pullMeta == "false" {
		Debug("Checking cache")
		files, err := os.ReadDir(uploadDir) // List current directory
		if err != nil {
			Warn("During file reterival os.ReadDir() encoutered " + err.Error())
		}

		for _, file := range files {
			if file.Name() == filename {
				Debug("Found file in cache")

				FileRetrievals++
				return filepath.Join(uploadDir, file.Name())
			}
		}

		Debug("No file found in cache, searching db")

		path, err := pullRecordByFilename(filename)
		if err != nil {
			Warn("pullRecordByFilename failed " + err.Error())
		}
		Debug("Pulled this path from db " + path)

		FileRetrievals++
		return path

		// : http.ResponseWriter, r *http.Request
	} else if pullMeta == "true" {
		Debug("Would sql search and pull meta")
		return ""
	} else {
		Debug("Unknown meta flag")
		return "Unknown meta flag"
	}
}

func changeMeta() {
	// Pull metadata from SQL
	// Update records according to the uploaded meta
}

func setFileStatus() {
	// searchAndReturn()

	// Search up file retrive it's path
	// Flip the deleted flag
}
