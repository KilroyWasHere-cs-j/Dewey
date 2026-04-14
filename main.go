package main

/**
curl upload testing command: curl -X POST http://localhost:8080/upload -F "file=@test.txt"
curl retrieve testing command: curl http://localhost:8080/files/test.txt
curl retrieve stored files command: curl http://localhost:8080/files
*/


/*
	Error code index
	0 = clean exit no error
	3 = directory error
	5 = server error
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

// startDaemon launches a background worker that periodically runs maintenance tasks.
//
// Behavior:
//   - Runs runTask every 3 hours
//   - Stops cleanly when context is cancelled
//
// Args:
//   - ctx: context used to signal shutdown (cancellation-safe goroutine)
func startDaemon(ctx context.Context) {
	Debug("starting cache clear daemon")

	ticker := time.NewTicker(3 * time.Hour)

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
					runTask()
				}()

			case <-ctx.Done():
				Warn("daemon stopped")
				return
			}
		}
	}()
}

// runTask executes periodic maintenance logic such as cache cleanup.
func runTask() {
	Debug("running system cache clear")

	// TODO: implement cache cleanup logic here
}

func main() {
	InitLogger("logs", "app")
	defer logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startDaemon(ctx)

	loadFilters()
	fileSystemInit()

	Debug("server starting")

	r := gin.New() // more control than gin.Default()

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

	// -------------------------
	// Start server
	// -------------------------
	Debug("server running on port " + portNumber)

	if err := r.Run(":" + portNumber); err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}
}
