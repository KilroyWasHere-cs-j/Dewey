package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

/*
	Default route, gives users a server alive message and a Unix timestamp
	Adding this in so default isn't a dead route
*/
func index(c *gin.Context) {
	Debug("index")
	c.JSON(http.StatusOK, gin.H{
		"message":   "Server alive",
		"timestamp": time.Now().Unix(),
	})
}

/*
	Retrives a file and it's assicated metadata if meta flag is set
*/
func getFile(c *gin.Context) {
	Debug("getFile")

	filename := c.Param("filename")
	meta := c.Param("meta")

	if meta == "true" {
		// User is requesting metadata to be in file return
		fmt.Println("Getting meta data")
	}

	// Security: prevent directory traversal
	if filepath.IsAbs(filename) || filepath.Base(filename) != filename {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
		return
	}

	filePath := filepath.Join(uploadDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		Warn(err.Error())
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	// Serve the file
	c.File(filePath)
}

/*
	Returns a list of all files stored
*/
func listFiles(c *gin.Context) {
	Debug("listFiles")
	file, err := os.ReadDir(uploadDir)
	if err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error" : "Unable to read form file directory"})
		return
	}

	var filenames []string
	for _, f := range file {
		filenames = append(filenames, f.Name())
	}

	c.JSON(http.StatusOK, gin.H{"found files" : filenames})
}

// uploadFile handles file uploads
func uploadFile(c *gin.Context) {
	Debug("uploadFile")
	// Get the file from form
	file, err := c.FormFile("file")
	if err != nil {
		Warn(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded or 'file' field missing",
		})
		return
	}


	Debug("checking formats")

	ext := filepath.Ext(file.Filename)
	allowedExts := map[string]bool{
		".pdf":  true,
		".txt":  true,
		".doc":  true,
		".docx": true,
		".xls":  true,
		".xlsx": true,
		".csv":  true,
		".ppt":  true,
		".png":  true,
		".jpg":  true,
		".jpeg": true,
	}

	// Ensure the uploaded file isn't part of a list of banded formats
	if !allowedExts[ext] {
		Warn("Invalid file type: " + ext)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type. Valid types: \n PDF \n TXT \n DOC/DOCX \n XLS/XLSX \n CSV",
		})
		return
	}

	// Open file for reading and preform some file type checks
	realFile, err := file.Open()

	if err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open file",
		})
		return
	}

	exe, err := IsPEFile(realFile)
	if err != nil {
		Warn(err.Error())
	}
	if exe == true {
		Warn("exe detected")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error" : "PE detected, shot down in selfprotect",
		})
		return 
	}

	elf, err := IsELFFile(realFile)
	if err != nil {
		Warn(err.Error())
	}
	if elf == true {
		Warn("elf detected")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error" : "ELF detected, shot down in selfprotect",
		})
		return
	}

	defer func(realFile multipart.File) {
		err := realFile.Close()
		if err != nil {
			Fatal(err.Error())
			os.Exit(5)
		}
	}(realFile)

	// Create unique filename to avoid collisions
	timestamp := time.Now().Unix()
	safeFilename := fmt.Sprintf("%d_%s", timestamp, file.Filename)

	// Create hasher
	hasher := sha256.New()

	// Stream file into hash (no full read into memory)
	if _, err := io.Copy(hasher, realFile); err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
		"error": "Failed to read file",
		})
		return
	}

	// Get hash
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)
	fmt.Println("SHA256 hash", hashString)

	// Save the file
	dst := filepath.Join(uploadDir, safeFilename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
		"filename": safeFilename,
		"original": file.Filename,
		"size":     file.Size,
		"path":     "/files/" + safeFilename,
	})
}

/*
	Delete a file from storage based on file name
*/
func deleteFile(c *gin.Context) {
	Debug("deleteFile")
	filename := c.Param("filename")

	file, err := os.ReadDir(uploadDir)
	if err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploads directory"})
		return
	}
	for _, f := range file {
		if !f.IsDir() {
			if f.Name() == filename {
				err := os.Remove(filepath.Join(uploadDir, f.Name()))
				if err != nil {
					Warn(err.Error())
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete requested file"})
					return
				}
			}
		}
	}
}
