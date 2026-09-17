package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"syscall"
	"net/netip"

	"github.com/go-sql-driver/mysql"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"golang.org/x/time/rate"

)

// startDaemon launches a background worker that periodically runs maintenance tasks.
//
// Behavior:
//   - Runs runTask every n hours
//   - Stops cleanly when context is cancelled
//
// Args:
//   - ctx: context used to signal shutdown (cancellation-safe goroutine)
//
// observableTicker wraps time.Ticker so callers can inspect time remaining
// until the next tick.
type observableTicker struct {
	ticker   *time.Ticker
	interval time.Duration
	start    time.Time
	mu       sync.RWMutex
	last     time.Time
}

type cacheKiller struct {
	uploadsSinceLastTick int
	lastTime             time.Time
	initialized          bool
	tickInterval         time.Duration
	rateEst              float64
	activeUsers          int
}

// daemonTicker is written once at startup and read from a Prometheus scrape
// goroutine (timeUntilNextTick), so the pointer itself needs its own
// synchronization on top of the mutex already guarding observableTicker's
// internal fields.
var daemonTicker atomic.Pointer[observableTicker]

func newObservableTicker(d time.Duration) *observableTicker {
	return &observableTicker{
		ticker:   time.NewTicker(d),
		interval: d,
		start:    time.Now(),
	}
}

// channel exposes the underlying ticker's channel so callers can select on it.
func (o *observableTicker) channel() <-chan time.Time { return o.ticker.C }

// stop stops the underlying ticker; the observableTicker itself is not
// otherwise reusable afterward.
func (o *observableTicker) stop() { o.ticker.Stop() }
func (o *observableTicker) markTick(t time.Time) {
	o.mu.Lock()
	o.last = t
	o.mu.Unlock()
}

// reset changes the ticker's period to d, effective immediately — the next
// tick fires d after this call, discarding whatever was left of the old
// period. Safe to call while another goroutine is receiving from channel().
func (o *observableTicker) reset(d time.Duration) {
	o.mu.Lock()
	o.interval = d
	o.mu.Unlock()
	o.ticker.Reset(d)
}

// remaining returns the duration until the next tick. If the ticker hasn't
// fired yet it computes remaining time from the start time. Never returns a
// negative duration; returns 0 if the next tick is due now.
func (o *observableTicker) remaining() time.Duration {
	o.mu.RLock()
	last := o.last
	start := o.start
	interval := o.interval
	o.mu.RUnlock()

	if last.IsZero() {
		elapsed := time.Since(start)
		// compute how far until the next interval boundary
		rem := interval - (elapsed % interval)
		if rem < 0 {
			return 0
		}
		return rem
	}

	rem := interval - time.Since(last)
	if rem < 0 {
		return 0
	}
	return rem
}

func startDaemon(ctx context.Context, pm *PluginManger) {
	// Debug("starting cache clear daemon")

	dt := newObservableTicker(time.Duration(daemonTickTime) * time.Minute)
	daemonTicker.Store(dt)

	go func() {
		defer dt.stop()

		// Lives for the goroutine's lifetime so the smoothed rate persists
		// across ticks — a fresh EMA each cycle would reset the smoothing
		// every time and defeat the point of it.
		rateEMA := newEMA(0.3)

		for {
			select {
			case t := <-dt.channel():
				// record tick time so remaining() can be observed
				dt.markTick(t)

				// Uploads + retrievals over the trailing 30s, smoothed so a
				// single noisy second doesn't yank the estimate around.
				instantRate := uploadCounter.rate() + retrievalCounter.rate()
				smoothedRate := rateEMA.update(instantRate)
				SetUploadRateEstimate(smoothedRate)

				// Both active users and upload/retrieval traffic push the
				// interval up (slower ticks) — maintenance work backs off
				// rather than competing with live activity.
				dt.reset(computeTickInterval(ActiveUserCount(), smoothedRate))

				// run task safely so panic won't kill goroutine
				func() {
					defer func() {
						if r := recover(); r != nil {
							Warn(fmt.Sprintf("daemon panic recovered: %v", r))
						}
					}()
					if err := dumpCache(); err != nil {
						Warn("Cache clear incomplete: " + err.Error())
					}
					atomic.AddInt64(&CacheCleanCycles, 1)

					// Reload only if the plugin directory actually changed
					// since the last tick — hashing every plugin file on
					// every tick is cheap for a handful of .lua files, but
					// an unconditional reload would still needlessly rerun
					// static validation and rebuild the Lua sandbox state
					// for every plugin even when nothing changed.
					if changed, err := pm.havePluginsChanged(); err != nil {
						Warn("Unable to check for plugin changes: " + err.Error())
					} else if changed {
						if err := pm.reloadPlugins(); err != nil {
							Warn("Plugin reload failed: " + err.Error())
						}
					}

					if _, err := pm.runByHook("OnTick", DBEntry{}); err != nil {
						Warn("Failed to run OnTick: " + err.Error())
					}
				}()

			case <-ctx.Done():
				// Warn("daemon stopped")
				return
			}
		}
	}()

	// Backup runs on its own fixed, coarser interval rather than riding the
	// load-adaptive dt ticker above — a full mysqldump + store/ zip is too
	// expensive to run on a clock tuned for cheap housekeeping like
	// cache-clearing, especially once dt backs off toward tickMin under
	// light load (issue #393).
	bt := time.NewTicker(time.Duration(backupIntervalMinutes) * time.Minute)

	go func() {
		defer bt.Stop()

		for {
			select {
			case <-bt.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							Warn(fmt.Sprintf("backup daemon panic recovered: %v", r))
						}
					}()
					if err := saveBackup(); err != nil {
						Warn("Backup incomplete: " + err.Error())
					} else {
						atomic.AddInt64(&FilesInBackUp, 1)
					}
				}()

			case <-ctx.Done():
				return
			}
		}
	}()
}

// timeUntilNextTick returns the duration until the daemon's next scheduled
// run. If the daemon hasn't been started it returns -1.
func timeUntilNextTick() time.Duration {
	dt := daemonTicker.Load()
	if dt == nil {
		return time.Duration(-1)
	}
	return dt.remaining()
}

// dumpCache clears all files from the upload cache directory. Best-effort:
// a failure removing one entry doesn't stop it from attempting the rest, so
// one stuck file can't block cleanup forever. app_cache_size is scanned
// live off disk rather than tracked here, so there's no counter to keep in
// sync with what this actually removes. Returns the first error
// encountered, if any, so the caller knows the clear was incomplete.
func dumpCache() error {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		Warn("Failed to read cache dir: " + err.Error())
		return err
	}

	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(uploadDir, entry.Name())); err != nil {
			Warn("Failed to remove cached file " + entry.Name() + ": " + err.Error())
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
	}
	return firstErr
}

// backupMySQLDatabase runs mysqldump against a running MySQL server and
// writes the SQL output to a timestamped file. Password is passed via the
// MYSQL_PWD env var rather than a -p flag, since command-line args are
// briefly visible to other processes on the host (e.g. `ps`).
func backupMySQLDatabase(host, port, user, password, database, outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	outPath := fmt.Sprintf("%s/%s_%s.sql", outDir, database, timestamp)

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating backup file: %w", err)
	}
	defer outFile.Close()

	// --set-gtid-purged=OFF: without it, mysqldump emits a
	// SET @@GLOBAL.GTID_PURGED=... statement that fails to restore onto the
	// same server the dump came from, since its GTID_EXECUTED already
	// overlaps with what the dump tries to purge. Dewey runs standalone
	// (no replication), so the dump only ever needs to restore onto itself.
	cmd := exec.Command("mysqldump", "-u", user, "-h", host, "-P", port, "--set-gtid-purged=OFF", database)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	cmd.Stdout = outFile

	var stderr []byte
	cmd.Stderr = &stderrCollector{buf: &stderr}

	if err := cmd.Run(); err != nil {
		os.Remove(outPath) // don't leave a partial/empty dump behind
		return "", fmt.Errorf("mysqldump failed: %w (stderr: %s)", err, stderr)
	}

	return outPath, nil
}

// restoreMySQLDatabase replays a dump produced by backupMySQLDatabase
// against a running server, restoring it into the given database.
func restoreMySQLDatabase(host, port, user, password, database, dumpPath string) error {
	dumpFile, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("opening dump file: %w", err)
	}
	defer dumpFile.Close()

	cmd := exec.Command("mysql", "-u", user, "-h", host, "-P", port, database)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	cmd.Stdin = dumpFile

	var stderr []byte
	cmd.Stderr = &stderrCollector{buf: &stderr}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql restore failed: %w (stderr: %s)", err, stderr)
	}

	return nil
}

// stderrCollector is a minimal io.Writer that appends to a byte slice,
// used to surface mysqldump/mysql's stderr in error messages above.
type stderrCollector struct{ buf *[]byte }

func (w *stderrCollector) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// dbConnParamsFromDSN reads DB_DSN — the same required env var
// newDatabaseManager (db.go) uses to open the connection pool — and splits
// it into the separate host/port/user/password/database pieces mysqldump
// and mysql need as individual CLI flags.
func dbConnParamsFromDSN() (host, port, user, password, database string, err error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return "", "", "", "", "", fmt.Errorf("DB_DSN environment variable is required")
	}

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("parsing DB_DSN: %w", err)
	}

	host, port, err = net.SplitHostPort(cfg.Addr)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("parsing DB_DSN address %q: %w", cfg.Addr, err)
	}

	return host, port, cfg.User, cfg.Passwd, cfg.DBName, nil
}

// latestBackupZip returns the most recent "<timestamp>_backup.zip" file in
// dir, matching the naming saveBackup uses when it creates one. Backup
// filenames sort lexicographically by their yyyymmdd_hhmmss timestamp, so
// the greatest name is also the newest.
func latestBackupZip(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading backup dir: %w", err)
	}

	var latest string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_backup.zip") {
			continue
		}
		if e.Name() > latest {
			latest = e.Name()
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no backup zip found in %q", dir)
	}

	return filepath.Join(dir, latest), nil
}

// latestBackupDump returns the most recent "<database>_<timestamp>.sql" file
// in dir, matching the naming backupMySQLDatabase uses when it creates one.
// Filenames sort lexicographically by their yyyymmdd_hhmmss timestamp, so
// the greatest name is also the newest.
func latestBackupDump(dir, database string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading dump dir: %w", err)
	}

	prefix := database + "_"
	var latest string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if e.Name() > latest {
			latest = e.Name()
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no dump found in %q for database %q", dir, database)
	}

	return filepath.Join(dir, latest), nil
}

// restoreFilesFromBackup extracts a zip archive produced by saveBackup's
// store/ backup back into destDir, restoring each file to the same relative
// path it was zipped from (issue #297). Each entry is resolved through
// resolveStorePath so a zip entry path (e.g. "../../etc/passwd") can't write
// outside destDir.
func restoreFilesFromBackup(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening backup zip: %w", err)
	}
	defer reader.Close()

	for _, f := range reader.File {
		destPath, err := resolveStorePath(destDir, f.Name)
		if err != nil {
			return fmt.Errorf("restoring %q: %w", f.Name, err)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("creating directory %q: %w", destPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("creating parent directory for %q: %w", destPath, err)
		}

		if err := extractZipFile(f, destPath); err != nil {
			return fmt.Errorf("restoring %q: %w", f.Name, err)
		}
	}

	return nil
}

// extractZipFile copies a single zip entry's contents to destPath,
// preserving the mode it was zipped with.
func extractZipFile(f *zip.File, destPath string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// dbDumpDir is where backupMySQLDatabase writes SQL dumps and where
// latestBackupDump looks for the most recent one to restore — kept as a
// single constant so save and load can't drift apart on the path.
const dbDumpDir = "./backups"

func saveBackup() error {
	Debug("Backing up the database")
	host, port, user, password, database, err := dbConnParamsFromDSN()
	if err != nil {
		return err
	}
	if _, err := backupMySQLDatabase(host, port, user, password, database, dbDumpDir); err != nil {
		return err
	}

	Debug("Backup completed successfully")

	Debug("Creating zip backup")

	sourceDir := "store"
	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		return err
	}

	// Create filename with UTC timestamp
	timestamp := time.Now().UTC().Format("20060102_150405")
	zipPath := filepath.Join(backupDir, timestamp+"_backup.zip")

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		writer, err := zipWriter.CreateHeader(&zip.FileHeader{Name: relPath, Method: zip.Store})
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, file)
		return err
	})
}

func loadBackup() error {
	Debug("Loading backup")
	host, port, user, password, database, err := dbConnParamsFromDSN()
	if err != nil {
		return err
	}
	dumpPath, err := latestBackupDump(dbDumpDir, database)
	if err != nil {
		return err
	}
	if err := restoreMySQLDatabase(host, port, user, password, database, dumpPath); err != nil {
		return err
	}

	Debug("Loading in store")
	zipPath, err := latestBackupZip(backupDir)
	if err != nil {
		return err
	}
	if err := restoreFilesFromBackup(zipPath, "store"); err != nil {
		return err
	}

	return nil
}

// clamp bounds value to the [tickMin, tickMax] range configured for the
// daemon tick interval.
func clamp(value float64) float64 {
	if value >= float64(tickMax) {
		return float64(tickMax)
	}
	if value <= float64(tickMin) {
		return float64(tickMin)
	}
	return value
}

// computeTickInterval maps current system activity to the daemon's next
// tick interval, in seconds. Both inputs push the interval up: more active
// users and/or a higher smoothed upload/retrieval rate both mean maintenance
// work (cache clear + backup zip) should back off rather than compete with
// live traffic. alpha/beta/tBase are the config-driven coefficients
// reserved for this (issue #305), clamped to [tickMin, tickMax].
func computeTickInterval(activeUsers int, smoothedRate float64) time.Duration {
	seconds := tBase + alpha*float64(activeUsers) + beta*smoothedRate
	return time.Duration(clamp(seconds)) * time.Second
}

type EMA struct {
	alpha    float64 // smoothing factor: 0 < alpha <= 1 (higher = reacts faster, noisier)
	value    float64 // current smoothed value
	hasValue bool    // false until the first sample seeds the average
}

// newEMA returns an EMA with the given smoothing factor.
// alpha is clamped to (0, 1] since values outside that range make the
// formula diverge or degenerate into a no-op.
func newEMA(alpha float64) *EMA {
	if alpha <= 0 {
		alpha = 0.01
	}
	if alpha > 1 {
		alpha = 1
	}
	return &EMA{alpha: alpha}
}

// update feeds a new sample in and returns the updated smoothed value.
// The first call just seeds the average with the raw sample — there's
// nothing to blend against yet.
func (e *EMA) update(sample float64) float64 {
	if !e.hasValue {
		e.value = sample
		e.hasValue = true
		return e.value
	}

	e.value = e.alpha*sample + (1-e.alpha)*e.value
	return e.value
}

// current returns the current smoothed value without feeding in a new
// sample. Named current rather than value to avoid colliding with EMA's own
// value field — Go doesn't allow a method and field to share a name.
func (e *EMA) current() float64 {
	return e.value
}

// SlidingWindowCounter counts events over a trailing window of windowSeconds,
// using one bucket per second in a ring buffer. Old buckets are lazily
// cleared as time passes rather than actively decayed, so idle periods cost
// nothing until something asks for the count.
type SlidingWindowCounter struct {
	mu         sync.Mutex
	buckets    []int64 // buckets[i] holds the count for one specific second
	bucketTime []int64 // unix-second timestamp each bucket was last written for
	windowSecs int64
}

// newSlidingWindowCounter creates a counter covering the trailing windowSeconds.
func newSlidingWindowCounter(windowSeconds int) *SlidingWindowCounter {
	return &SlidingWindowCounter{
		buckets:    make([]int64, windowSeconds),
		bucketTime: make([]int64, windowSeconds),
		windowSecs: int64(windowSeconds),
	}
}

// record logs one event (an upload, a retrieval — call sites decide which
// counter to bump) at the current time.
func (c *SlidingWindowCounter) record() {
	now := time.Now().Unix()
	idx := now % c.windowSecs

	c.mu.Lock()
	defer c.mu.Unlock()

	// This bucket's slot was last written for a different (older) second —
	// it's stale, so overwrite rather than accumulate onto last time's count.
	if c.bucketTime[idx] != now {
		c.buckets[idx] = 0
		c.bucketTime[idx] = now
	}
	c.buckets[idx]++
}

// count returns the number of events recorded within the trailing window,
// as of now. Buckets whose timestamp has aged out of the window are treated
// as zero without needing to be cleared up front.
func (c *SlidingWindowCounter) count() int64 {
	now := time.Now().Unix()

	c.mu.Lock()
	defer c.mu.Unlock()

	var total int64
	for i, t := range c.bucketTime {
		if now-t < c.windowSecs {
			total += c.buckets[i]
		}
	}
	return total
}

// rate returns events per second, averaged over the trailing window.
func (c *SlidingWindowCounter) rate() float64 {
	return float64(c.count()) / float64(c.windowSecs)
}

// uploadCounter and retrievalCounter track upload/retrieval throughput over
// a trailing 30s window, read from startDaemon's tick loop to feed the
// upload-rate EMA (app_upload_rate).
var (
	uploadCounter    = newSlidingWindowCounter(30)
	retrievalCounter = newSlidingWindowCounter(30)
)

func SpinDownTrigger() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Println("Running... press Ctrl+C to stop")

	select {
	case <-ctx.Done():
		fmt.Println("\nCtrl+C detected, shutting down")
	case <-time.After(30 * time.Second):
		fmt.Println("timed out")
	}
}

// SpinDown runs final cleanup once request draining is already complete —
// callers must not invoke this while requests may still be in flight, since
// closing dbm out from under an active request would break it.
func SpinDown(dbm *DatabaseManager) {
	Debug("Spinning down the server...")

	Debug("Saving backup...")
	if err := saveBackup(); err != nil {
		Warn("Backup incomplete: " + err.Error())
	} else {
		atomic.AddInt64(&FilesInBackUp, 1)
		Debug("Backup saved successfully")
	}

	Debug("Closing database connection pool...")
	if err := dbm.Close(); err != nil {
		Warn("Database close incomplete: " + err.Error())
	} else {
		Debug("Database connection pool closed")
	}
}

func SpinUp() error {
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
		return err
	}
	if err := dbm.migrate(); err != nil {
		Fatal("Failed to run migrations: " + err.Error())
		return err
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
		files.POST("/files/move", moveFile)
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

	
	srv := &http.Server{Addr: ":" + portNumber, Handler: r}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Fatal(err.Error())
		}
	}()

	// Run BITs in the background so they fire at startup after all init is complete,
	// without blocking the server from starting. Explicitly pass the port this
	// process is actually listening on — test_suite.py defaults to :8080 when
	// called with no argument, which is wrong for any instance not bound to
	// that exact port (e.g. a second backend running alongside the usual one).
	go func() {
		Section("BITs")
		cmd := exec.Command("python3", "testing_tooling/test_suite.py", "http://localhost:"+portNumber)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			Warn("BITs failed: " + err.Error())
		} else {
			Ok("BITs passed")
		}
	}()

	// OS signals feed into the SAME trigger as any programmatic call above.
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-sigCtx.Done()
		triggerShutdown()
	}()

	<-shutdownCtx.Done() // blocks here until *anything* calls triggerShutdown()
	Debug("shutdown triggered, draining...")

	shutdownTimeoutCtx, cancelShutdownTimeout := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdownTimeout()
	if err := srv.Shutdown(shutdownTimeoutCtx); err != nil {
		Fatal("forced shutdown: " + err.Error())
		return err
	}

	// Requests have finished draining, so it's now safe to back up and
	// close the database — nothing is still relying on the connection pool.
	SpinDown(dbm)
	return nil
}

var (
	shutdownCtx    context.Context
	triggerOnce    sync.Once
	cancelShutdown context.CancelFunc
)

func init() {
	shutdownCtx, cancelShutdown = context.WithCancel(context.Background())
}

// triggerShutdown starts the shutdown sequence. Safe to call multiple
// times or from multiple goroutines — only the first call has any effect.
func triggerShutdown() {
	triggerOnce.Do(cancelShutdown)
}

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
