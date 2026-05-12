package main

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"golang.org/x/time/rate"
)

// I created this, so future debuggers can have some fun...
var appRules *Config

func main() {
	InitLogger("logs", "app")
	defer logger.Close()

	testPlugin()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startDaemon(ctx)

	dbPing := InitDB()
	if dbPing != nil {
		Warn("Bad db")
	}
	rules := fileSystemInit()
	appRules = rules

	// go startPrometheus()

	barcodeText, err := scanBarCode("./barcodes/one.png")
	if err != nil {
		Warn("Unable to process barcodes " + err.Error())
	}
	Debug(barcodeText)

	// dbFunction()
	Debug("server starting")

	r := gin.New()

	// -------------------------
	// Core middleware
	// -------------------------
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	//r.Use(PrometheusMiddleware())

	p := ginprometheus.NewWithConfig(ginprometheus.Config{
		Subsystem: "gin",
	})
	p.Use(r)
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
	//r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// -------------------------
	// Start server
	// -------------------------
	r.Run(":8080")
	Debug("server running on port " + portNumber)

	if err := r.Run(":" + portNumber); err != nil {
		Fatal(err.Error())
	}
}
