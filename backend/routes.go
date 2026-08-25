package main

import (
	"errors"
	"fmt"
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

// respondError writes the HTTP response for err returned by a validation or
// storage helper (filemanager.go/filevalidator.go) — those helpers report
// the intended status/message via *apiError rather than writing to c
// directly, so this is the one place that turns that into an actual
// response. Falls back to 500 for a plain error, matching the generic
// failure response those call sites already used before this existed.
func respondError(c *gin.Context, err error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		c.JSON(apiErr.status, gin.H{"error": apiErr.message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		retrievalCounter.Record()

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
	pm := c.MustGet("plugins").(*PluginManger)
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

	data := c.PostForm("data")

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

	if err := checkFileSize(fileHeader); err != nil {
		Warn(fmt.Sprintf("file too large: %d bytes", fileHeader.Size))
		respondError(c, err)
		return
	}
	Debug(fmt.Sprintf("File size is valid. File size: %d bytes", fileHeader.Size))

	Debug("checking formats")

	// -------------------------
	// Validate extension
	// -------------------------
	ext, err := validateFileExtensionType(fileHeader)
	if err != nil {
		Warn("Incorrect file type")
		respondError(c, err)
		return
	}

	Debug("Vaild file type")

	// Track accepted extension so the frontend can show upload distribution
	uploadsByType.WithLabelValues(ext).Inc()

	// -------------------------
	// Open file
	// -------------------------
	err, file := OpenFile(fileHeader)
	if err != nil {
		respondError(c, err)
		return
	}
	defer file.Close()

	// -------------------------
	// Security checks
	// -------------------------
	if err := checkFileForExe(file); err != nil {
		respondError(c, err)
		return
	}

	if err := checkFileContent(file, ext); err != nil {
		respondError(c, err)
		return
	}

	if err := CheckPDFJavaScript(ext, file); err != nil {
		respondError(c, err)
		return
	}

	// -------------------------
	// Generate filename
	// -------------------------
	safeFilename := CreateTimestamp(fileHeader.Filename)

	// -------------------------
	// Hash file
	// -------------------------
	var hashString string
	err, hashString = CreateFileHash(file)
	if err != nil {
		Warn("Failed to hash file: " + err.Error())
		respondError(c, err)
		return
	}

	Debug("SHA256: " + hashString)

	// Record file size distribution for the histogram
	uploadSizeBytes.Observe(float64(fileHeader.Size))

	// -------------------------
	// Save file
	// -------------------------
	// Reuse the already-open multipart file instead of c.SaveUploadedFile,
	// which re-opens and re-copies the same bytes from the multipart source
	// a second time — the hashing pass above already read this file once
	// (issue #166).
	err = SaveFile(safeFilename, file)
	if err != nil {
		Warn("Failed to save file: " + err.Error())
		respondError(c, err)
		return
	}
	uploadCounter.Record()

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
	pm := c.MustGet("plugins").(*PluginManger)
	dbm := c.MustGet("db").(*DatabaseManager)

	filename := filepath.Base(c.Param("filename")) // prevent path traversal
	err := DeleteFile(filename, pm, dbm)

	if err != nil {
		Warn(err.Error())
		respondError(c, err)
		return
	}

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

// reloadPlugins reloads the plugins from the filesystem.
//
// Returns (HTTP JSON):
//   - 200 OK: plugins reloaded
//   - 500 Internal Server Error: reload failure
func reloadPlugins(c *gin.Context) {
	Debug("reloadPlugins called")
	pm := c.MustGet("plugins").(*PluginManger)
	if err := pm.ReloadPlugins(); err != nil {
		Warn("reloadPlugins failed: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to reload plugins"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plugins reloaded"})
}
