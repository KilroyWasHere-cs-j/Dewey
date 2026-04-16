package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"strings"

	"github.com/gin-gonic/gin"
)

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

	filename := c.Param("filename")
	metaFlag := c.Query("meta") // FIX: meta should be query param, not path param

	// -------------------------
	// 1. Validate filename
	// -------------------------
	if filepath.IsAbs(filename) || filepath.Base(filename) != filename {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid filename",
		})
		return
	}

	filePath := filepath.Join(uploadDir, filename)

	// -------------------------
	// 2. Check file existence
	// -------------------------
	fileLocation, err := searchAndReturn(filePath)
	if err != nil {
		Warn("file not found: " + err.Error())
		c.JSON(http.StatusNotFound, gin.H{
			"error": "file not found",
		})
		return
	}

	// -------------------------
	// 3. Metadata handling
	// -------------------------
	if metaFlag == "true" {
		Debug("metadata requested")

		info, err := os.Stat(fileLocation)
		if err != nil {
			Warn(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to read file metadata",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"name":    info.Name(),
			"size":    info.Size(),
			"modTime": info.ModTime(),
		})
		return
	}

	Debug("serving file")

	// -------------------------
	// 4. Serve file
	// -------------------------
	c.File(fileLocation)
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

	// -------------------------
	// 1. Get uploaded file
	// -------------------------
	fileHeader, err := c.FormFile("file")
	if err != nil {
		Warn(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded or 'file' field missing",
		})
		return
	}

	Debug("checking formats")

	// -------------------------
	// 2. Validate extension
	// -------------------------
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

	allowedExts := map[string]struct{}{
		".pdf":  {},
		".txt":  {},
		".doc":  {},
		".docx": {},
		".xls":  {},
		".xlsx": {},
		".csv":  {},
		".ppt":  {},
		".png":  {},
		".jpg":  {},
		".jpeg": {},
	}

	if _, ok := allowedExts[ext]; !ok {
		Warn("invalid file type: " + ext)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type. Allowed: PDF, TXT, DOC/DOCX, XLS/XLSX, CSV, PPT, PNG, JPG",
		})
		return
	}

	// -------------------------
	// 3. Open file stream
	// -------------------------
	file, err := fileHeader.Open()
	if err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open file",
		})
		return
	}
	defer file.Close()

	// -------------------------
	// 4. Security checks (PE / ELF)
	// -------------------------
	if isPE, err := IsPEFile(file); err == nil && isPE {
		Warn("PE detected")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Executable file detected (PE blocked)",
		})
		return
	}

	// reset stream for next read
	file.Seek(0, io.SeekStart)

	if isELF, err := IsELFFile(file); err == nil && isELF {
		Warn("ELF detected")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Executable file detected (ELF blocked)",
		})
		return
	}

	// reset again before hashing
	file.Seek(0, io.SeekStart)

	// -------------------------
	// 5. Generate safe filename
	// -------------------------
	timestamp := time.Now().Unix()
	safeFilename := fmt.Sprintf("%d_%s", timestamp, filepath.Base(fileHeader.Filename))

	// -------------------------
	// 6. Compute SHA-256 hash
	// -------------------------
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read file",
		})
		return
	}

	hashString := hex.EncodeToString(hasher.Sum(nil))
	Debug("SHA256: " + hashString)

	// -------------------------
	// 7. Save file to disk
	// -------------------------
	dst := filepath.Join(uploadDir, safeFilename)
	if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
		Warn(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file",
		})
		return
	}

	// -------------------------
	// 8. Post-processing
	// -------------------------
	idAndSort(safeFilename)

	// -------------------------
	// 9. Response
	// -------------------------
	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
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
