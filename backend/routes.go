package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type UploadPayload struct {
	Name string `json:"name"`
	Data string `json:"data"` // or whatever fields you expect
}

// index is the default health-check route.
//
// Behavior:
//   - Confirms the server is running
//   - Returns a Unix timestamp for basic uptime reference
//
// Returns (HTTP JSON):
//   - 200 OK: server status + current timestamp
func index(c *gin.Context) {
	Debug("index")

	c.JSON(http.StatusOK, gin.H{
		"message":   "server alive",
		"timestamp": time.Now().Unix(),
	})
}

// getFile retrieves a file from storage and optionally returns metadata.
//
// URL Params:
//   - filename: name of the file to retrieve
//   - meta: "true" or "false" indicating whether metadata should be included
//
// Behavior:
//   - Validates filename to prevent path traversal
//   - Locates file in upload directory
//   - If meta=true, includes file metadata in response
//   - Otherwise returns file content only
//
// Returns (HTTP JSON or file stream):
//   - 200 OK: file content (and optional metadata)
//   - 400 Bad Request: invalid filename
//   - 404 Not Found: file does not exist
func getFile(c *gin.Context) {
	Debug("getFile")
	dbm := c.MustGet("db").(*DatabaseManager)

	filename := c.Param("filename")
	metaFlag := c.Param("meta")

	c.File(searchAndReturn(dbm, filename, metaFlag))
}

// listFiles returns all files stored in the upload directory.
//
// Args:
//   - c (*gin.Context): Gin request context
//
// Behavior:
//   - Reads the upload directory
//   - Filters out subdirectories (only returns files)
//   - Returns a JSON list of filenames
//
// Returns (HTTP JSON):
//   - 200 OK: list of filenames
//   - 500 Internal Server Error: unable to read directory
func listFiles(c *gin.Context) {
	Debug("listFiles")

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		Warn("failed to read upload dir: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to read upload directory",
		})
		return
	}

	filenames := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filenames = append(filenames, entry.Name())
	}

	Debug("files found: " + fmt.Sprintf("%d", len(filenames)))

	c.JSON(http.StatusOK, gin.H{
		"files": filenames,
		"count": len(filenames),
	})
}

// uploadFile handles file uploads via multipart/form-data.
//
// Args:
//   - c (*gin.Context): Gin request context containing the uploaded file under form field "file"
//
// Behavior:
//   - Validates file extension against allowlist
//   - Performs basic binary checks (PE/ELF detection)
//   - Computes SHA-256 hash of uploaded content
//   - Saves file to uploadDir with timestamped filename
//   - Queues file via idAndSort
//
// Returns (HTTP JSON):
//   - 200 OK: upload success + file metadata
//   - 400 Bad Request: missing file or invalid extension
//   - 500 Internal Server Error: processing or storage failure

func uploadFile(c *gin.Context) {
	Debug("uploadFile")
	pm := c.MustGet("plugins").(*PluginManager)
	dbm := c.MustGet("db").(*DatabaseManager)

	contentType := c.GetHeader("Content-Type")

	// -------------------------
	// JSON ONLY
	// -------------------------
	if strings.Contains(contentType, "application/json") {
		var payload UploadPayload

		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "JSON received",
			"data":    payload,
		})
		return
	}

	// -------------------------
	// MULTIPART (file + fields)
	// -------------------------
	if !strings.Contains(contentType, "multipart/form-data") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported Content-Type",
		})
		return
	}

	// Get form fields

	metadata := MetaData{
		ClaimNumber:  c.PostForm("claim_number"),
		ClaimantName: c.PostForm("claimant_name"),
		DateOfInjury: c.PostForm("date_of_injury"),
		Employer:     c.PostForm("employer"),
		Adjuster:     c.PostForm("adjuster"),
		Support:      c.PostForm("support"),
		ClaimType:    c.PostForm("claim_type"),
		Jurisdiction: c.PostForm("jurisdiction"),
		PolicyNumber: c.PostForm("policy_number"),
		ACTsID:       c.PostForm("acts_id"),
	}

	fmt.Print("metadata: " + fmt.Sprintf("%+v\n", metadata))

	data := c.PostForm("data")

	Debug("data: " + data)

	// Get file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file is required",
		})
		return
	}

	Debug("checking formats")

	// -------------------------
	// Validate extension
	// -------------------------
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

	allowedExts := map[string]struct{}{
		".pdf": {}, ".txt": {}, ".doc": {}, ".docx": {},
		".xls": {}, ".xlsx": {}, ".csv": {}, ".ppt": {},
		".png": {}, ".jpg": {}, ".jpeg": {},
	}

	if _, ok := allowedExts[ext]; !ok {
		Warn("invalid file type: " + ext)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type",
		})
		return
	}
	Debug("Vaild file type")

	// -------------------------
	// Open file
	// -------------------------
	file, err := fileHeader.Open()
	if err != nil {
		Warn("Failed to open file: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open file",
		})
		return
	}
	defer file.Close()

	// -------------------------
	// Security checks
	// -------------------------
	if isPE, _ := IsPEFile(file); isPE {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Executable file detected (PE blocked)",
		})
		return
	}

	file.Seek(0, io.SeekStart)

	if isELF, _ := IsELFFile(file); isELF {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Executable file detected (ELF blocked)",
		})
		return
	}

	file.Seek(0, io.SeekStart)

	// -------------------------
	// Generate filename
	// -------------------------
	timestamp := time.Now().Unix()
	safeFilename := fmt.Sprintf("%d_%s", timestamp, filepath.Base(fileHeader.Filename))

	// -------------------------
	// Hash file
	// -------------------------
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		Warn("Failed to read file: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read file",
		})
		return
	}

	hashString := hex.EncodeToString(hasher.Sum(nil))
	Debug("SHA256: " + hashString)

	// -------------------------
	// Save file
	// -------------------------
	dst := filepath.Join(uploadDir, safeFilename)
	if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
		Warn("Failed to save file: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file",
		})
		return
	}

	// -------------------------
	// Post-processing
	// -------------------------
	idAndSort(pm, dbm, safeFilename, hashString, safeFilename, metadata)

	// -------------------------
	// Response
	// -------------------------
	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
		"data":     data,
		"filename": safeFilename,
		"original": fileHeader.Filename,
		"size":     fileHeader.Size,
		"sha256":   hashString,
	})
}

// deleteFile removes a file from upload storage by filename.
//
// Args:
//   - c (*gin.Context): Gin request context containing the "filename" URL param
//
// Behavior:
//   - Builds a safe filesystem path from uploadDir and filename
//   - Attempts to delete the file from disk
//   - Returns appropriate HTTP response based on outcome
//
// Returns (HTTP JSON):
//   - 200 OK: file successfully deleted
//   - 404 Not Found: file does not exist
//   - 500 Internal Server Error: filesystem deletion failure
func deleteFile(c *gin.Context) {
	filename := filepath.Base(c.Param("filename")) // prevent path traversal
	path := filepath.Join(uploadDir, filename)

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			Warn("file not found: " + filename)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "File not found",
			})
		} else {
			Warn("failed to delete file: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to delete file",
			})
		}
		return
	}

	Debug("file deleted: " + filename)

	c.JSON(http.StatusOK, gin.H{
		"message":  "File deleted",
		"filename": filename,
	})
}

func triggerCacheDump(c *gin.Context) {
	dumpCache()
	c.JSON(http.StatusOK, gin.H{
		"message": "CacheDump triggered",
	})
}
