package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"

	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"golang.org/x/time/rate"
)

func main() {

	// --- Logger
	InitLogger("logs", "app")
	defer logger.Close()

	// --- Plugins
	Debug("Loading plugins...")
	pm := NewPluginManager()
	defer pm.Close()
	err := pm.LoadPlugins()
	if err != nil {
		Warn("Failed to load plugins: " + err.Error())
	}
	pm.ListPlugins()
	pm.RunPlugins(Init)

	// --- Daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startDaemon(ctx, pm)

	// --- DB
	dbm, err := NewDatabaseManager()

	if err != nil {
		// I want a hard fail if the database can't initialize
		Fatal("Failed to initialize database: " + err.Error())
	}
	Info("Creating files table if it doesn't exist...")
	dbm.Migrate()

	// --- Filesystem
	fileSystemInit()

	// --- Server
	Debug("server starting")
	r := gin.New()
	r.MaxMultipartMemory = maxFileSize

	// Core middleware
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Prometheus
	p := ginprometheus.NewWithConfig(ginprometheus.Config{Subsystem: "gin"})
	p.Use(r)

	// Rate limiter
	limiter := rate.NewLimiter(1, 5)
	r.Use(func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	})

	// 404
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "page not found"})
	})

	// All routes share plugin and DB context via middleware
	api := r.Group("/")
	api.Use(func(c *gin.Context) {
		c.Set("plugins", pm)
		c.Set("db", dbm)
		c.Next()
	})
	{
		// Health check
		api.GET("/", index)

		// HTML views
		api.GET("/settings", func(c *gin.Context) { c.HTML(http.StatusOK, "settings.html", nil) })

		// File management
		api.POST("/upload", uploadFile)
		files := api.Group("/files")
		{
			files.GET("", listFiles)
			files.GET("/:filename/:meta", getFile)
			files.DELETE("/:filename", deleteFile)
		}

		// Admin — portal view, cache operations, and runtime config
		admin := api.Group("/admin")
		{
			admin.GET("", func(c *gin.Context) { c.HTML(http.StatusOK, "adminportal.html", nil) })
			admin.GET("/dumpCache", triggerCacheDump)

			// Runtime settings — each param updates the corresponding backend constant
			set := admin.Group("/set")
			{
				set.GET("/daemonTickInterval/:tickInterval", setDaemonTickInterval)
				set.GET("/maxUpSize/:size", setMaxUploadSize)
				set.GET("/maxDBOpenConn/:openCons", setMaxDBOpenConn)
				set.GET("/maxDBIdleConn/:idleCons", setMaxDBIdleConn)
				set.GET("/dbTimeout/:dbTimeout", setDBTimeout)
			}
		}
	}

	// Run BITs in the background so they fire at startup after all init is complete,
	// without blocking the server from starting.
	go func() {
		Debug("---------------------------------------------------------- Running BITs ----------------------------------------------------------")
		cmd := exec.Command("/bin/bash", "testing_tooling/test_suite.sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		Debug("Running test suite: " + cmd.String())
		if err := cmd.Run(); err != nil {
			Warn("BITs failed: " + err.Error())
		}
	}()

	Debug("server running on port " + portNumber)
	if err := r.Run(":" + portNumber); err != nil {
		Fatal(err.Error())
	}
}
