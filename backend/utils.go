package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-sql-driver/mysql"
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
// goroutine (TimeUntilNextTick), so the pointer itself needs its own
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

func (o *observableTicker) Chan() <-chan time.Time { return o.ticker.C }
func (o *observableTicker) Stop()                  { o.ticker.Stop() }
func (o *observableTicker) markTick(t time.Time) {
	o.mu.Lock()
	o.last = t
	o.mu.Unlock()
}

// Reset changes the ticker's period to d, effective immediately — the next
// tick fires d after this call, discarding whatever was left of the old
// period. Safe to call while another goroutine is receiving from Chan().
func (o *observableTicker) Reset(d time.Duration) {
	o.mu.Lock()
	o.interval = d
	o.mu.Unlock()
	o.ticker.Reset(d)
}

// Remaining returns the duration until the next tick. If the ticker hasn't
// fired yet it computes remaining time from the start time. Never returns a
// negative duration; returns 0 if the next tick is due now.
func (o *observableTicker) Remaining() time.Duration {
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

	dt := newObservableTicker(time.Duration(daemonTickTime) * time.Second)
	daemonTicker.Store(dt)

	go func() {
		defer dt.Stop()

		// Lives for the goroutine's lifetime so the smoothed rate persists
		// across ticks — a fresh EMA each cycle would reset the smoothing
		// every time and defeat the point of it.
		rateEMA := NewEMA(0.3)

		for {
			select {
			case t := <-dt.Chan():
				// record tick time so Remaining() can be observed
				dt.markTick(t)

				// Uploads + retrievals over the trailing 30s, smoothed so a
				// single noisy second doesn't yank the estimate around.
				instantRate := uploadCounter.Rate() + retrievalCounter.Rate()
				smoothedRate := rateEMA.Update(instantRate)
				SetUploadRateEstimate(smoothedRate)

				// Both active users and upload/retrieval traffic push the
				// interval up (slower ticks) — maintenance work backs off
				// rather than competing with live activity.
				dt.Reset(computeTickInterval(ActiveUserCount(), smoothedRate))

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

					err := saveBackup()
					if err == nil {
						atomic.AddInt64(&FilesInBackUp, 1)
					}
					if _, err := pm.RunByHook("OnTick", DBEntry{}); err != nil {
						Warn("Failed to run OnTick: " + err.Error())
					}
				}()

			case <-ctx.Done():
				// Warn("daemon stopped")
				return
			}
		}
	}()
}

// TimeUntilNextTick returns the duration until the daemon's next scheduled
// run. If the daemon hasn't been started it returns -1.
func TimeUntilNextTick() time.Duration {
	dt := daemonTicker.Load()
	if dt == nil {
		return time.Duration(-1)
	}
	return dt.Remaining()
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
// NewDatabaseManager (db.go) uses to open the connection pool — and splits
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

		writer, err := zipWriter.Create(relPath)
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

// NewEMA returns an EMA with the given smoothing factor.
// alpha is clamped to (0, 1] since values outside that range make the
// formula diverge or degenerate into a no-op.
func NewEMA(alpha float64) *EMA {
	if alpha <= 0 {
		alpha = 0.01
	}
	if alpha > 1 {
		alpha = 1
	}
	return &EMA{alpha: alpha}
}

// Update feeds a new sample in and returns the updated smoothed value.
// The first call just seeds the average with the raw sample — there's
// nothing to blend against yet.
func (e *EMA) Update(sample float64) float64 {
	if !e.hasValue {
		e.value = sample
		e.hasValue = true
		return e.value
	}

	e.value = e.alpha*sample + (1-e.alpha)*e.value
	return e.value
}

// Value returns the current smoothed value without feeding in a new sample.
func (e *EMA) Value() float64 {
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

// NewSlidingWindowCounter creates a counter covering the trailing windowSeconds.
func NewSlidingWindowCounter(windowSeconds int) *SlidingWindowCounter {
	return &SlidingWindowCounter{
		buckets:    make([]int64, windowSeconds),
		bucketTime: make([]int64, windowSeconds),
		windowSecs: int64(windowSeconds),
	}
}

// Record logs one event (an upload, a retrieval — call sites decide which
// counter to bump) at the current time.
func (c *SlidingWindowCounter) Record() {
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

// Count returns the number of events recorded within the trailing window,
// as of now. Buckets whose timestamp has aged out of the window are treated
// as zero without needing to be cleared up front.
func (c *SlidingWindowCounter) Count() int64 {
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

// Rate returns events per second, averaged over the trailing window.
func (c *SlidingWindowCounter) Rate() float64 {
	return float64(c.Count()) / float64(c.windowSecs)
}

// uploadCounter and retrievalCounter track upload/retrieval throughput over
// a trailing 30s window, read from startDaemon's tick loop to feed the
// upload-rate EMA (app_upload_rate).
var (
	uploadCounter    = NewSlidingWindowCounter(30)
	retrievalCounter = NewSlidingWindowCounter(30)
)
