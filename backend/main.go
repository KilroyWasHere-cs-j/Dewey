package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"golang.org/x/time/rate"
)

// logConnections gates every route behind the known_machines allowlist and
// records each authorized request — this is both the access control and the
// "who and when" audit trail for the closed network this server runs on.
func logConnections(dbm *DatabaseManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		label, err := dbm.checkKnownMachine(ip)
		if err != nil {
			Warn(fmt.Sprintf("unregistered machine %s hit %s %s", ip, c.Request.Method, c.FullPath()))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unregistered machine"})
			return
		}

		Debug(fmt.Sprintf("%s (%s) -> %s %s", label, ip, c.Request.Method, c.FullPath()))
		if err := dbm.logMachineIP(ip); err != nil {
			Warn("failed to update last_seen_at for " + ip + ": " + err.Error())
		}

		c.Next()
	}
}

func main() {

	InitLogger("logs", "app")
	defer logger.Close()
	Banner()

	Section("Plugins")
	pm := NewPluginManger()
	defer pm.Close()
	// Hooks must be registered before LoadPlugins runs — LoadPlugins only
	// checks plugin files against already-registered hook names.
	pm.RegisterHook("OnInit")
	pm.RegisterHook("OnFilter")
	pm.RegisterHook("OnTick")
	pm.RegisterHook("OnUpload")
	pm.RegisterHook("OnDelete")
	if err := pm.LoadPlugins(); err != nil {
		Warn("Failed to load plugins: " + err.Error())
	}
	pm.ListPlugins()
	if _, err := pm.RunByHook("OnInit", DBEntry{}); err != nil {
		Warn("Failed to run OnInit: " + err.Error())
	}

	Section("Daemon")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startDaemon(ctx, pm)
	Ok("daemon started")

	Section("Database")
	dbm, err := NewDatabaseManager()
	if err != nil {
		Fatal("Failed to initialize database: " + err.Error())
	}
	if err := dbm.Migrate(); err != nil {
		Fatal("Failed to run migrations: " + err.Error())
	}
	Ok("database ready")

	Section("Filesystem")
	fileSystemInit()
	Ok("filesystem ready")

	Section("Server")
	r := gin.New()
	r.MaxMultipartMemory = maxFileSize
	// No reverse proxy in front of this pod, so don't trust forwarded-for
	// headers — otherwise a VM could spoof its way past the IP allowlist.
	r.SetTrustedProxies(nil)

	// Core middleware
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Prometheus
	p := ginprometheus.NewWithConfig(ginprometheus.Config{Subsystem: "gin"})
	p.Use(r)

	// Rate limiter
	limiter := rate.NewLimiter(rateLimitPerSecond, rateLimitBurst)
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

	// Routes with plugin context
	api := r.Group("/")
	api.Use(logConnections(dbm))
	api.Use(func(c *gin.Context) {
		c.Set("plugins", pm)
		c.Set("db", dbm)
		c.Next()
	})
	{
		api.GET("/", index)
		api.GET("/version", versionInfo)
		api.GET("/admin", func(c *gin.Context) { c.HTML(http.StatusOK, "adminportal.html", nil) })
		api.GET("/settings", func(c *gin.Context) { c.HTML(http.StatusOK, "settings.html", nil) })
		api.POST("/upload", uploadFile)
		api.GET("/files/:filename/:meta", getFile)
		api.GET("/files", listFiles)
		api.DELETE("/files/:filename", deleteFile)
		api.GET("/admin/dumpCache", triggerCacheDump)
		api.GET("/machines", listMachines)
		api.POST("/machines", addMachine)
		api.DELETE("/machines/:ip", deleteMachine)
	}

	// Run BITs in the background so they fire at startup after all init is complete,
	// without blocking the server from starting.
	go func() {
		Section("BITs")
		cmd := exec.Command("/bin/bash", "testing_tooling/test_suite.sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			Warn("BITs failed: " + err.Error())
		} else {
			Ok("BITs passed")
		}
	}()

	Ok("listening on :" + portNumber)
	if err := r.Run(":" + portNumber); err != nil {
		Fatal(err.Error())
	}
}
