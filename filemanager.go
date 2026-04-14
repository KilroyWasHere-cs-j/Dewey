package main

import (
	"os"
	"fmt"
	"encoding/json"
	"path/filepath"
)

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

func loadFilters() (*Config) {
	Debug("Attempting to load filters")

	file, err := os.Open(rulesDir + "/master.json")
	if err != nil {
		Fatal(err.Error())
	}
	defer file.Close()

	var cfg Config

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&cfg); err != nil {
		Fatal(err.Error())
	}
	Debug("Loading filters successful")
	return &cfg
}

func fileSystemInit() {
	// Create caching directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		Fatal(err.Error())
		os.Exit(3)
	}

	// Create store directory if it doesn't exist
	if err := os.MkdirAll(fileSystemBaseDir , 0755); err != nil {
		Fatal(err.Error())
		os.Exit(3)
	}
}

func idAndSort(path string){
	//	barcodeText := scanBarCode(path)

	// Determine where the file needs to go
	// Create and store db entry
	// Store file bytes and dn entry route
}

func searchAndReturn(filePath string) (string, error) {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file does not exist: %s", filePath)
		}
		return "", fmt.Errorf("error checking file: %w", err)
	}

	// Optional: ensure it's not a directory
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}

	// Resolve absolute path
	filePathABS, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	Debug("Resolved absolute path: " + filePathABS)

	return filePathABS, nil
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
