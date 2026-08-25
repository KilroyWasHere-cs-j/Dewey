package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"golang.org/x/time/rate"
)

// resolveClientIP parses the raw RemoteAddr Go's net/http sets on every
// request, preserving any IPv6 zone identifier.
//
// Gin's own c.ClientIP() can't be used here — it calls net.ParseIP, which
// silently returns nil (so an empty ClientIP()) for any zone-qualified
// address such as "fe80::...%eth0". That's exactly what rootless Podman's
// pasta network helper presents for host-to-forwarded-port connections
// (issue #315): SplitHostPort succeeds, but ParseIP then drops the
// connection's identity entirely, misreading a present-but-unusual address
// as a missing one. netip.ParseAddr, unlike net.ParseIP, understands zone
// identifiers and parses these cleanly.
func resolveClientIP(remoteAddr string) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return netip.Addr{}, err
	}
	return netip.ParseAddr(host)
}

// logConnections gates every route behind the known_machines allowlist and
// records each authorized request — this is both the access control and the
// "who and when" audit trail for the closed network this server runs on.
func logConnections(dbm *DatabaseManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip, err := resolveClientIP(c.Request.RemoteAddr)
		if err != nil {
			Warn(fmt.Sprintf("could not resolve client IP from remote addr %q (%s), hit %s %s", c.Request.RemoteAddr, err, c.Request.Method, c.FullPath()))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unregistered machine"})
			return
		}
		ipStr := ip.String()

		// Link-local IPv6 (fe80::/10) is only reachable from the local link.
		// Under rootless Podman's pasta network helper, this is specifically
		// how host-to-forwarded-port connections present themselves (issue
		// #315) rather than a routable loopback address. Treated as
		// host-equivalent here rather than requiring each host's
		// NIC-derived address to be registered individually — that address
		// is tied to the host's specific interface and isn't stable across
		// machines or interface changes, so pre-seeding it into
		// known_machines the way 127.0.0.1/::1 are wouldn't generalize.
		if ip.IsLinkLocalUnicast() {
			Debug(fmt.Sprintf("localhost (link-local %s) -> %s %s", ipStr, c.Request.Method, c.FullPath()))
			stop := trackUser(ipStr)
			defer stop()
			c.Next()
			return
		}

		label, err := dbm.checkKnownMachine(ipStr)
		if err != nil {
			Warn(fmt.Sprintf("unregistered machine %s (raw remote addr %q) hit %s %s", ipStr, c.Request.RemoteAddr, c.Request.Method, c.FullPath()))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unregistered machine"})
			return
		}

		Debug(fmt.Sprintf("%s (%s) -> %s %s", label, ipStr, c.Request.Method, c.FullPath()))
		if err := dbm.logMachineIP(ipStr); err != nil {
			Warn("failed to update last_seen_at for " + ipStr + ": " + err.Error())
		}

		stop := trackUser(ipStr)
		defer stop()

		c.Next()
	}
}

func main() {

	InitLogger("logs", "app")
	defer logger.Close()
	Banner()

	Section("Config")
	load()
	Ok("config loaded from " + configFile)

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

	Section("Daemons")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startDaemon(ctx, pm)

	Ok("daemons summoned")

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

	// Prometheus — request-metric collection runs globally, but the /metrics
	// endpoint itself is registered below inside the api group so it sits
	// behind the same known_machines IP allowlist as every other route
	// (issue #203). p.Use(r) would register it directly on the bare engine,
	// bypassing that group entirely.
	p := ginprometheus.NewWithConfig(ginprometheus.Config{Subsystem: "gin"})
	r.Use(p.HandlerFunc())

	// Rate limiter
	limiter := rate.NewLimiter(rate.Limit(rateLimitPerSecond), rateLimitBurst)
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
	base := r.Group("/")
	base.Use(logConnections(dbm))
	{
		base.GET("/", index)
		base.GET("/version", versionInfo)
		base.GET(p.MetricsPath, gin.WrapH(promhttp.Handler()))
	}

	admin := r.Group("/admin")
	admin.Use(logConnections(dbm))
	{
		admin.GET("/", func(c *gin.Context) { c.HTML(http.StatusOK, "adminportal.html", nil) })
		admin.GET("/settings", func(c *gin.Context) { c.HTML(http.StatusOK, "settings.html", nil) })
	}

	core := r.Group("/core")
	core.Use(logConnections(dbm))
	core.Use(func(c *gin.Context) {
		c.Set("plugins", pm)
		c.Set("db", dbm)
		c.Next()
	})
	{
		core.POST("/upload", uploadFile)
		core.GET("/files/:filename/:meta", getFile)
		core.GET("/files", listFiles)
		core.DELETE("/files/:filename", deleteFile)
		core.POST("/files/move/:currentfilepathandname/:newfilepathandname", moveFile)
		core.GET("/admin/dumpCache", triggerCacheDump)
		core.GET("/admin/reloadPlugins", reloadPlugins)
		core.GET("/machines", listMachines)
		core.POST("/machines", addMachine)
		core.DELETE("/machines/:ip", deleteMachine)
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
