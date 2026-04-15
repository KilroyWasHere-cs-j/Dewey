package main

import (
	"os"
	"io"
	"fmt"
	"encoding/json"
	"path/filepath"
	"regexp"
)

// var filters *Config
// PDF files should be treated as seperate files for each page with there own records

type Config struct {
	Version string `json:"version"`
	Level   string `json:"level"`
	Rules   []Rule `json:"rules"`
}

type Rule struct {
	NameMatch       string `json:"name_match"`
	Action          string `json:"action"`
	Meta            []Meta `json:"meta"`
	TargetDirectory string `json:"target_directory"`
}

type Meta struct {
	Tag1 string `json:"tag1"`
}

// loadFilters loads the master filter configuration from disk.
//
// Behavior:
//   - Reads rulesDir/master.json
//   - Strictly decodes JSON (no unknown fields allowed)
//   - Panics (Fatal) if config cannot be loaded
//
// Returns:
//   - *Config: parsed filter configuration
func loadFilters() *Config {
	Debug("attempting to load filters")

	path := filepath.Join(rulesDir, "master.json")

	file, err := os.Open(path)
	if err != nil {
		Fatal("failed to open config: " + err.Error())
	}
	defer file.Close()

	var cfg Config

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&cfg); err != nil {
		Fatal("invalid config JSON: " + err.Error())
	}

	Debug("filter configuration loaded successfully")

	return &cfg
}

// fileSystemInit ensures required filesystem directories exist before server start.
//
// Behavior:
//   - Creates upload directory
//   - Creates base storage directory
//	 - Initiate the loading of filter rules
//   - Fails fast if any directory cannot be created
func fileSystemInit() *Config {
	dirs := []string{
		uploadDir,
		fileSystemBaseDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			Fatal("failed to create directory " + dir + ": " + err.Error())
		}
	}
	
	config := loadFilters()
	if config == nil {
		Fatal("Unable to load configs")
	}

	Debug("filesystem initialization complete")
	return config
}

func idAndSort(path string){
	//  barcodeText := scanBarCode(path)

	//-------------------------------
	// Perform name match rule
	//-------------------------------
	for _, pattern := range appRules.Rules {
		Debug("Attempting match with regex: " + pattern.NameMatch)
		re := regexp.MustCompile(pattern.NameMatch)
		if re.MatchString(path) {
			Debug("Match on pattern: " + pattern.NameMatch)
			Debug("Action: " + pattern.Action)
			Debug("Target dir: " + pattern.TargetDirectory)

			err := os.MkdirAll(fileSystemBaseDir + pattern.TargetDirectory, 0755)
			if err != nil {
				Warn("Failed to create directory for uploaded file " + err.Error())
			}
			
			Debug("Placing file at " + fileSystemBaseDir + "/" + pattern.TargetDirectory + "/" + path)
			
			// err = os.Rename(uploadDir + "/" + path, fileSystemBaseDir + "/" + pattern.TargetDirectory + "/" + path)
			// if err != nil {
			// 	Warn(err.Error())
			// }

			err = CopyFile(
				filepath.Join(uploadDir, path),
				filepath.Join(fileSystemBaseDir, pattern.TargetDirectory, path),
			)

			if err != nil {
				Warn(err.Error())
				return
			}

			return
		} else {
			Debug("No match")
		}
	}

	



	// Determine where the file needs to go
	// Create and store db entry
	// Store file bytes and dn entry route
}

func CopyFile(src, dst string) error {
	// open source
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

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
func searchAndReturn(filePath string) (string, error) {
	// Normalize to absolute early (helps prevent traversal ambiguity)
	absInput, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve input path: %w", err)
	}

	info, err := os.Stat(absInput)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file does not exist: %s", absInput)
		}
		return "", fmt.Errorf("error checking file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}

	Debug("resolved absolute path: " + absInput)

	return absInput, nil
}

func changeMeta(){
	// Pull metadata from SQL
	// Update records according to the uploaded meta
}

func setFileStatus(){
	// searchAndReturn()

	// Search up file retrive it's path
	// Flip the deleted flag
}
