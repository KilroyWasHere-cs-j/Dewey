package main

/**
curl upload testing command: curl -X POST http://localhost:8080/upload -F "file=@test.txt"
curl retrieve testing command: curl http://localhost:8080/files/test.txt
curl retrieve stored files command: curl http://localhost:8080/files
*/


import (
	"context"
	"os"
	"net/http"
	"time"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// I created this, so future debuggers can have some fun...
var appRules *Config

// startDaemon launches a background worker that periodically runs maintenance tasks.
//
// Behavior:
//   - Runs runTask every n hours
//   - Stops cleanly when context is cancelled
//
// Args:
//   - ctx: context used to signal shutdown (cancellation-safe goroutine)
func startDaemon(ctx context.Context) {
	Debug("starting cache clear daemon")

	ticker := time.NewTicker(daemonTickTime * time.Hour)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// run task safely so panic won't kill goroutine
				func() {
					defer func() {
						if r := recover(); r != nil {
							Warn("daemon panic recovered")
						}
					}()
					dumpCache()
				}()

			case <-ctx.Done():
				Warn("daemon stopped")
				return
			}
		}
	}()
}

// runTask executes periodic maintenance logic such as cache cleanup.
func dumpCache() {
	Debug("Running system cache dump")
	entries, err := os.ReadDir(uploadDir) // Read current directory
  if err != nil {
		Fatal("Failed to dump cache dir " + err.Error())
  }

  for _, entry := range entries {
		if !entry.IsDir() {
			err := os.Remove(uploadDir + "/" + entry.Name())
			if err != nil {
				Fatal("Failed to remove a file from the cache " + err.Error())
			}
		}	
  }
	Debug("Cache dumped")
}

func main() {
	InitLogger("logs", "app")
	defer logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startDaemon(ctx)

	rules := fileSystemInit()
	appRules = rules


	// dbFunction()
	Debug("server starting")

	r := gin.New() // more control than gin.Default()

	// gin.SetMode(gin.Release)
	// -------------------------
	// Core middleware
	// -------------------------
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Limit multipart uploads
	r.MaxMultipartMemory = maxFileSize

	// -------------------------
	// Rate limiting (GLOBAL)
	// -------------------------
	limiter := rate.NewLimiter(1, 5)

	r.Use(func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			return
		}
		c.Next()
	})

	// -------------------------
	// 404 handler
	// -------------------------
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "PAGE_NOT_FOUND",
			"message": "page not found",
		})
	})

	// -------------------------
	// Templates
	// -------------------------
	r.LoadHTMLGlob(filepath.Join(htmlTemplatesDir, "*"))

	// -------------------------
	// Routes
	// -------------------------
	r.GET("/", index)

	r.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "adminportal.html", nil)
	})

	r.GET("/settings", func(c *gin.Context) {
		c.HTML(http.StatusOK, "settings.html", nil)
	})

	r.POST("/upload", uploadFile)
	r.GET("/files/:filename/:meta", getFile)
	r.GET("/files", listFiles)
	r.DELETE("/files/:filename", deleteFile)
	r.GET("/admin/dumpCache", triggerCacheDump)
	// -------------------------
	// Start server
	// -------------------------
	Debug("server running on port " + portNumber)

	if err := r.Run(":" + portNumber); err != nil {
		Fatal(err.Error())	
	}
}
