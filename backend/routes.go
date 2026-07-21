package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// versionInfo reports the running binary's release version and the git
// branch it was built from, so it's possible to confirm what's actually
// deployed without shelling into the container. See version.go for how
// GitBranch gets set.
//
// Returns (HTTP JSON):
//   - 200 OK: version + git branch
func versionInfo(c *gin.Context) {
	Debug("versionInfo")

	c.JSON(http.StatusOK, gin.H{
		"version":    appVersion,
		"git_branch": GitBranch,
	})
}

// getFile retrieves a file or its metadata depending on the meta flag.
//
// URL Params:
//   - filename: name of the file to retrieve
//   - meta: "true" returns a JSON metadata record; "false" streams the file
//
// Returns:
//   - 200 OK + file stream (meta=false)
//   - 200 OK + JSON MetaData (meta=true)
//   - 400 Bad Request: unknown meta flag value
//   - 404 Not Found: file or metadata record does not exist
func getFile(c *gin.Context) {
	Debug("getFile")
	dbm := c.MustGet("db").(*DatabaseManager)

	filename := c.Param("filename")
	metaFlag := c.Param("meta")

	switch metaFlag {
	case "false":
		// Locate and stream the file; path traversal is stripped inside locateFile
		path, err := locateFile(dbm, filename)
		if err != nil {
			Warn("getFile failed: " + err.Error())
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}
		// Force a download instead of inline rendering, and stop browsers from
		// re-sniffing the content type — a stored file whose bytes look like
		// HTML/script must not be executed just because it's served from here.
		c.Header("X-Content-Type-Options", "nosniff")
		c.FileAttachment(path, filepath.Base(filename))

	case "true":
		// Return the metadata record linked to this file as JSON
		meta, err := dbm.pullMetaByFilename(filename)
		if err != nil {
			Warn("getFile metadata failed: " + err.Error())
			c.JSON(http.StatusNotFound, gin.H{"error": "Metadata not found"})
			return
		}
		c.JSON(http.StatusOK, meta)

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unknown meta flag: %s", metaFlag)})
	}
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

	// MaxMultipartMemory only controls the in-memory/disk threshold while
	// parsing — it doesn't cap the request body itself. Without this, a
	// client can stream an unbounded body regardless of Content-Length,
	// exhausting disk as gin buffers it. This enforces a hard ceiling.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileSize)

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
		// http.MaxBytesReader surfaces as a read error here once the body
		// cap above is exceeded, rather than as a clean multipart error.
		if strings.Contains(err.Error(), "too large") {
			uploadRejections.WithLabelValues("too_large").Inc()
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "File exceeds maximum allowed size",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file is required",
		})
		return
	}

	// Belt-and-suspenders check against the declared part size, independent
	// of whatever MaxBytesReader caught at the body-read level.
	if fileHeader.Size > maxFileSize {
		Warn(fmt.Sprintf("file too large: %d bytes", fileHeader.Size))
		uploadRejections.WithLabelValues("too_large").Inc()
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "File exceeds maximum allowed size",
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
		uploadRejections.WithLabelValues("invalid_ext").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type",
		})
		return
	}
	Debug("Vaild file type")

	// Track accepted extension so the frontend can show upload distribution
	uploadsByType.WithLabelValues(ext).Inc()

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
		uploadRejections.WithLabelValues("pe_blocked").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Executable file detected (PE blocked)",
		})
		return
	}

	file.Seek(0, io.SeekStart)

	if isELF, _ := IsELFFile(file); isELF {
		uploadRejections.WithLabelValues("elf_blocked").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Executable file detected (ELF blocked)",
		})
		return
	}

	file.Seek(0, io.SeekStart)

	// Sniff actual content and reject anything that doesn't match what the
	// extension claims — e.g. a "report.txt" that's really an HTML/script
	// payload passes the extension allowlist otherwise.
	matches, err := MatchesDeclaredType(ext, file)
	if err != nil {
		Warn("Failed to sniff file content: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to validate file content",
		})
		return
	}
	if !matches {
		Warn("file content does not match declared extension: " + ext)
		uploadRejections.WithLabelValues("content_mismatch").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File content does not match its extension",
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

	// Record file size distribution for the histogram
	uploadSizeBytes.Observe(float64(fileHeader.Size))

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
	atomic.AddInt64(&FilesInCache, 1)

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

	// -------------------------
	// Post-processing
	// -------------------------
	queueIdAndSort(pm, dbm, safeFilename, hashString, safeFilename, metadata)

}

// deleteFile removes a file from upload storage by filename.
//
// Args:
//   - c (*gin.Context): Gin request context containing the "filename" URL param
//
// Behavior:
//   - Looks up the file's permanent path via the DB record (this is the copy
//     idAndSort placed under fileSystemBaseDir — deleting only the uploadDir/cache
//     copy left the store copy, and its DB record, behind forever)
//   - Removes the store copy from disk and marks the DB record as deleted
//   - Also clears any leftover cache copy, best-effort
//   - Returns appropriate HTTP response based on outcome
//
// Returns (HTTP JSON):
//   - 200 OK: file successfully deleted
//   - 404 Not Found: no active file record for this filename
//   - 500 Internal Server Error: filesystem or DB failure
func deleteFile(c *gin.Context) {
	filename := filepath.Base(c.Param("filename")) // prevent path traversal
	dbm := c.MustGet("db").(*DatabaseManager)

	storeRelPath, err := dbm.pullRecordByFilename(filename)
	if err != nil {
		Warn("file record not found: " + err.Error())
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found",
		})
		return
	}

	storePath := filepath.Join(fileSystemBaseDir, storeRelPath)
	if err := os.Remove(storePath); err != nil && !os.IsNotExist(err) {
		Warn("failed to delete file from store: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete file",
		})
		return
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
		return
	}

	Debug("file deleted: " + filename)
	atomic.AddInt64(&FileDeletions, 1)
	atomic.AddInt64(&FilesInStore, -1)

	c.JSON(http.StatusOK, gin.H{
		"message":  "File deleted",
		"filename": filename,
	})
}

func triggerCacheDump(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "CacheDump triggered",
	})
	go dumpCache()
}

// listMachines returns every machine registered in the known_machines allowlist.
//
// Returns (HTTP JSON):
//   - 200 OK: list of known machines
//   - 500 Internal Server Error: query failure
func listMachines(c *gin.Context) {
	dbm := c.MustGet("db").(*DatabaseManager)

	machines, err := dbm.getKnownMachines()
	if err != nil {
		Warn("listMachines failed: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve known machines"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"machines": machines})
}

// addMachine registers a new machine in the known_machines allowlist.
//
// Body (JSON): {"ip": "...", "label": "..."}
//
// Returns (HTTP JSON):
//   - 200 OK: machine added
//   - 400 Bad Request: missing ip or label
//   - 409 Conflict: ip already registered
//   - 500 Internal Server Error: insert failure
func addMachine(c *gin.Context) {
	dbm := c.MustGet("db").(*DatabaseManager)

	var body struct {
		IP    string `json:"ip"`
		Label string `json:"label"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.IP == "" || body.Label == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip and label are required"})
		return
	}

	if err := dbm.addKnownMachine(body.IP, body.Label); err != nil {
		if errors.Is(err, ErrMachineExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Machine already registered"})
			return
		}
		Warn("addMachine failed: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to add machine"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Machine added", "ip": body.IP, "label": body.Label})
}

// deleteMachine removes a machine from the known_machines allowlist by IP.
//
// Returns (HTTP JSON):
//   - 200 OK: machine removed
//   - 500 Internal Server Error: delete failure
func deleteMachine(c *gin.Context) {
	dbm := c.MustGet("db").(*DatabaseManager)
	ip := c.Param("ip")

	if err := dbm.removeKnownMachine(ip); err != nil {
		Warn("deleteMachine failed: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to remove machine"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Machine removed", "ip": ip})
}
