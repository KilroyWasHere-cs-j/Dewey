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
//
// trackActivity controls whether the request also counts toward
// ActiveUserCount() (issue #309). /metrics itself sits behind this same
// middleware, so without this flag every scrape of /metrics counts its own
// in-flight connection — guaranteeing ActiveUserCount() never reads below 1
// and masking whether any other request is actually concurrent with it.
func logConnections(dbm *DatabaseManager, trackActivity bool) gin.HandlerFunc {
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
			if trackActivity {
				stop := trackUser(ipStr)
				defer stop()
			}
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

		if trackActivity {
			stop := trackUser(ipStr)
			defer stop()
		}

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
	pm := newPluginManger()
	defer pm.close()
	// Hooks must be registered before loadPlugins runs — loadPlugins only
	// checks plugin files against already-registered hook names.
	pm.registerHook("OnInit")
	pm.registerHook("OnFilter")
	pm.registerHook("OnTick")
	pm.registerHook("OnUpload")
	pm.registerHook("OnDelete")
	if err := pm.loadPlugins(); err != nil {
		Warn("Failed to load plugins: " + err.Error())
	}
	pm.listPlugins()
	if _, err := pm.runByHook("OnInit", DBEntry{}); err != nil {
		Warn("Failed to run OnInit: " + err.Error())
	}

	Section("Daemons")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startDaemon(ctx, pm)

	Ok("daemons summoned")

	Section("Database")
	dbm, err := newDatabaseManager()
	if err != nil {
		Fatal("Failed to initialize database: " + err.Error())
	}
	if err := dbm.migrate(); err != nil {
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
	base.Use(logConnections(dbm, true))
	{
		base.GET("/", index)
		base.GET("/version", versionInfo)
	}

	// Separate group so /metrics stays behind the same known_machines
	// allowlist as everything else, without its own scrape counting toward
	// ActiveUserCount() (issue #309) the way it would under the group above.
	metrics := r.Group("/")
	metrics.Use(logConnections(dbm, false))
	{
		metrics.GET(p.MetricsPath, gin.WrapH(promhttp.Handler()))
	}

	fileview := r.Group("/fileview")
	fileview.Use(logConnections(dbm, false))
	{
		fileview.GET("/viewLogDir", getViewLogFile)
		fileview.GET("/viewFile/:fileType/:file", getViewFile)
	}

	admin := r.Group("/admin")
	admin.Use(logConnections(dbm, true))
	{
		admin.GET("/", func(c *gin.Context) { c.HTML(http.StatusOK, "adminportal.html", nil) })
		admin.GET("/settings", func(c *gin.Context) { c.HTML(http.StatusOK, "settings.html", nil) })
	}

	core := r.Group("/core")
	core.Use(logConnections(dbm, true))
	core.Use(func(c *gin.Context) {
		c.Set("plugins", pm)
		c.Set("db", dbm)
		c.Next()
	})

	// Password exchange endpoints (issue #409) — each sits directly on
	// core, not behind requireSession, since checking the password is
	// their entire job. A client calls one of these once per login and
	// uses the returned token for every subsequent request instead of
	// resending the actual password.
	core.POST("/files/session", exchangeForSession("files"))
	core.POST("/machines/session", exchangeForSession("machines"))

	// Separate passwords per capability (issue #332) — deleting stored
	// documents and altering who can reach the server at all are different
	// enough risks that one shared secret for both didn't make sense.
	files := core.Group("")
	files.Use(requireSession("files"))
	{
		files.POST("/upload", uploadFile)
		files.GET("/files/*filename", getFile)
		files.GET("/files", listFiles)
		files.DELETE("/files/*filename", deleteFile)
		files.POST("/files/undelete/*filename", undeleteFile)
		files.POST("/files/move/:currentfilepathandname/:newfilepathandname", moveFile)
		files.POST("/files/refilter/*filename", refilterFile)
		files.GET("/admin/dumpCache", triggerCacheDump)
		files.GET("/admin/reloadPlugins", reloadPlugins)
	}

	machines := core.Group("")
	machines.Use(requireSession("machines"))
	{
		machines.GET("/machines", listMachines)
		machines.POST("/machines", addMachine)
		machines.DELETE("/machines/:ip", deleteMachine)
	}

	// Run BITs in the background so they fire at startup after all init is complete,
	// without blocking the server from starting. Explicitly pass the port this
	// process is actually listening on — test_suite.sh defaults to :8080 when
	// called with no argument, which is wrong for any instance not bound to
	// that exact port (e.g. a second backend running alongside the usual one).
	go func() {
		Section("BITs")
		cmd := exec.Command("/bin/bash", "testing_tooling/test_suite.sh", "http://localhost:"+portNumber)
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
