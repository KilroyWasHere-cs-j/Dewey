package main

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var appName = "crossApp"
var startTime = time.Now()

// Atomic counters — safe for concurrent access from HTTP handlers
var FileSorts int64
var FileRetrievals int64
var FileDeletions int64
var FiltersLoadings int64
var FileCopys int64
var BarcodeSuccesses int64
var BarcodeFailures int64
var PluginRuns int64
var PluginErrors int64
var DBErrors int64
var FilesInStore int64
var FilesInBackUp int64
var UploadsSinceLastTick int64
var CacheCleanCycles int64

// activeUsers tracks how many in-flight requests each client IP currently
// has open — an IP counts as connected only while a request is actually
// being processed, so this rises and falls with real traffic rather than
// just the known_machines allowlist size.
var (
	activeUsers   = make(map[string]int)
	activeUsersMu sync.Mutex
)

// trackUser marks ip as having one more in-flight request and returns a
// function that must be called (typically via defer) when that request
// finishes.
func trackUser(ip string) func() {
	activeUsersMu.Lock()
	activeUsers[ip]++
	activeUsersMu.Unlock()

	return func() {
		activeUsersMu.Lock()
		activeUsers[ip]--
		if activeUsers[ip] <= 0 {
			delete(activeUsers, ip) // don't let the map grow forever with zero-counts
		}
		activeUsersMu.Unlock()
	}
}

// ActiveUserCount returns how many distinct client IPs currently have an
// in-flight request.
func ActiveUserCount() int {
	activeUsersMu.Lock()
	defer activeUsersMu.Unlock()
	return len(activeUsers)
}

// uploadRateEst holds the latest EMA-smoothed upload rate computed by
// cacheKiller.updateRateEstimate. A plain mutex guards it rather than an
// atomic — float64 has no native atomic add/store in this codebase's Go
// version, and this is only touched once per daemon tick plus once per
// Prometheus scrape, so contention isn't a concern.
var (
	uploadRateEst   float64
	uploadRateEstMu sync.Mutex
)

// SetUploadRateEstimate records the latest upload-rate EMA value for the
// app_upload_rate gauge to read.
func SetUploadRateEstimate(v float64) {
	uploadRateEstMu.Lock()
	uploadRateEst = v
	uploadRateEstMu.Unlock()
}

// UploadRateEstimate returns the most recently recorded upload-rate EMA.
func UploadRateEstimate() float64 {
	uploadRateEstMu.Lock()
	defer uploadRateEstMu.Unlock()
	return uploadRateEst
}

var (
	fileOps = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "myapp",
			Name:        "file_io_ops_total",
			Help:        "Counts of file IO operations.",
			ConstLabels: prometheus.Labels{"app": appName},
		},
		[]string{"op", "result", "file_group"},
	)

	fileBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "myapp",
			Name:        "file_io_bytes_total",
			Help:        "Total bytes read/written.",
			ConstLabels: prometheus.Labels{"app": appName},
		},
		[]string{"op"},
	)

	fileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "myapp",
			Name:        "file_io_duration_seconds",
			Help:        "Duration of file IO operations.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"app": appName},
		},
		[]string{"op"},
	)

	uptime = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_uptime_seconds",
			Help: "Uptime of the application in seconds",
		},
		func() float64 {
			return time.Since(startTime).Seconds()
		},
	)

	systemInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "app_system_info",
			Help: "System information",
		},
		[]string{"os", "arch", "version"},
	)

	cpuCount = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_cpu_count",
			Help: "CPU count of the application",
		},
		func() float64 {
			return float64(runtime.NumCPU())
		},
	)

	ramUsage = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_ram_usage",
			Help: "Total memory obtained from the OS by the application (bytes, converted to MB)",
		},
		func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			// Sys is total memory obtained from the OS — unlike TotalAlloc
			// (cumulative allocations since start, never decreases), this
			// reflects actual current RAM footprint.
			return float64(m.Sys / 1024 / 1024)
		},
	)

	currentHeap = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_heap_usage",
			Help: "Heap usage of the application",
		},
		func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.HeapAlloc / 1024 / 1024)
		},
	)

	gcCycles = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_gc_cycles",
			Help: "Number of GC cycles performed by the application",
		},
		func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.NumGC)
		},
	)

	// Counted live off disk rather than tracked incrementally: an
	// atomic-counter approach requires every code path that adds/removes a
	// cache file to remember to update it, and deleteFile's cache-copy
	// removal doesn't (soak-testing #311 surfaced the drift this causes
	// under sustained upload/retrieval traffic). Scanning uploadDir on
	// every scrape can never drift, since it has no state to drift from —
	// cache is expected to stay small (the daemon sweeps it every tick),
	// so this is cheap.
	cacheSize = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_cache_size",
			Help: "Size of the application cache",
		},
		func() float64 {
			return float64(countFilesRecursive(uploadDir))
		},
	)

	connectedUsers = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_connected_users",
			Help: "Number of distinct client IPs with an in-flight request",
		},
		func() float64 {
			return float64(ActiveUserCount())
		},
	)

	uploadRate = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_upload_rate",
			Help: "EMA-smoothed upload rate in uploads/sec",
		},
		UploadRateEstimate,
	)

	filesInStore = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_files_in_store",
			Help: "Number of files in the application store",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&FilesInStore))
		},
	)
	filesInBackUp = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_files_in_backup",
			Help: "Number of files in the application backup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&FilesInBackUp))
		},
	)

	exeCount = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_exe_count",
			Help: "Number of executable files in the application store",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&PECount) + atomic.LoadInt64(&ELFCount))
		},
	)

	filtersLoadings = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_filters_loadings",
			Help: "Number of times the filters configuration has been loaded",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&FiltersLoadings))
		},
	)

	fileCopys = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_copys",
			Help: "Number of times a file has been copied",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&FileCopys))
		},
	)

	fileRetrievals = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_retrievals",
			Help: "Number of times a file has been retrieved",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&FileRetrievals))
		},
	)

	fileSorts = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_sorts",
			Help: "Number of times a file has been sorted",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&FileSorts))
		},
	)

	timeTilNextTick = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_time_til_next_tick",
			Help: "Time until the next daemon tick",
		},
		func() float64 {
			return float64(TimeUntilNextTick().Seconds())
		},
	)

	// Counts daemon tick cycles that ran a cache clear (issue #311's soak
	// tool logs this alongside RAM/heap/goroutines to see cache-clean
	// cadence over a long run, not just current cache size).
	cacheCleanCycles = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_cache_clean_cycles_total",
			Help: "Number of daemon tick cycles that ran a cache clear since startup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&CacheCleanCycles))
		},
	)

	// FileDeletions was declared but never registered — wired up here
	fileDeletions = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_deletions",
			Help: "Number of files deleted since startup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&FileDeletions))
		},
	)

	// Upload rejections broken out by rejection reason
	uploadRejections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_upload_rejections_total",
			Help: "Number of upload rejections, labelled by reason",
		},
		[]string{"reason"},
	)

	// Accepted uploads broken out by file extension
	uploadsByType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_uploads_by_type_total",
			Help: "Number of accepted uploads, labelled by file extension",
		},
		[]string{"ext"},
	)

	// Distribution of uploaded file sizes in bytes
	uploadSizeBytes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "app_upload_size_bytes",
			Help:    "Distribution of uploaded file sizes",
			Buckets: []float64{1024, 10240, 102400, 524288, 1048576, 5242880, 10485760, 52428800},
		},
	)

	barcodeSuccesses = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_barcode_successes",
			Help: "Number of successful barcode scans since startup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&BarcodeSuccesses))
		},
	)

	barcodeFailures = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_barcode_failures",
			Help: "Number of failed barcode scans since startup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&BarcodeFailures))
		},
	)

	pluginRuns = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_plugin_runs",
			Help: "Number of plugin filter executions since startup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&PluginRuns))
		},
	)

	pluginErrors = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_plugin_errors",
			Help: "Number of plugin filter errors since startup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&PluginErrors))
		},
	)

	dbErrors = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_db_errors",
			Help: "Number of database operation errors since startup",
		},
		func() float64 {
			return float64(atomic.LoadInt64(&DBErrors))
		},
	)
)

// countFilesRecursive walks dir and counts non-directory entries. Used only
// to seed the file-count counters once at startup — everything after that
// is maintained incrementally at the actual write/delete call sites, so this
// never runs on the scrape path.
func countFilesRecursive(dir string) int64 {
	var count int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		Warn(err.Error())
	}
	return count
}

func init() {
	atomic.StoreInt64(&FilesInStore, countFilesRecursive(fileSystemBaseDir))
	atomic.StoreInt64(&FilesInBackUp, countFilesRecursive(backupDir))

	prometheus.MustRegister(
		fileOps, fileBytes, fileDuration,
		uptime, systemInfo, cpuCount, ramUsage, currentHeap, gcCycles,
		cacheSize, filesInStore, filesInBackUp, exeCount, connectedUsers, uploadRate,
		fileCopys, fileRetrievals, fileSorts, filtersLoadings, timeTilNextTick,
		// new in issue #104
		fileDeletions,
		// new in issue #311
		cacheCleanCycles,
		uploadRejections, uploadsByType, uploadSizeBytes,
		barcodeSuccesses, barcodeFailures,
		pluginRuns, pluginErrors,
		dbErrors,
	)

	// Zero-valued label instances so metrics appear even before any traffic
	fileBytes.WithLabelValues("read")
	fileBytes.WithLabelValues("write")

	fileOps.WithLabelValues("read", "ok", "default")
	fileOps.WithLabelValues("write", "ok", "default")

	fileDuration.WithLabelValues("read")
	fileDuration.WithLabelValues("write")

	// Pre-seed all known rejection reasons so they appear in the scrape at zero
	uploadRejections.WithLabelValues("invalid_ext")
	uploadRejections.WithLabelValues("pe_blocked")
	uploadRejections.WithLabelValues("elf_blocked")
	uploadRejections.WithLabelValues("content_mismatch")
	uploadRejections.WithLabelValues("pdf_js_blocked")
}
