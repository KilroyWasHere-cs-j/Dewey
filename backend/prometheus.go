package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
			Help: "RAM usage of the application",
		},
		func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.TotalAlloc / 1024 / 1024)
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

	cacheSize = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_cache_size",
			Help: "Size of the application cache",
		},
		func() float64 {
			entries, err := os.ReadDir("./cache")
			if err != nil {
				Warn("Failed to read cache directory: " + err.Error())
			}

			count := 0
			for _, entry := range entries {
				// Use !entry.IsDir() to exclude subdirectories from the count
				if !entry.IsDir() {
					count++
				}
			}
			return float64(count)
		},
	)

	filesInStore = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_files_in_store",
			Help: "Number of files in the application store",
		},
		func() float64 {
			count := 0
			err := filepath.WalkDir("./store", func(path string, d fs.DirEntry, err error) error {
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
			return float64(count)
		},
	)
	filesInBackUp = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_files_in_backup",
			Help: "Number of files in the application backup",
		},
		func() float64 {
			count := 0
			err := filepath.WalkDir("./backup", func(path string, d fs.DirEntry, err error) error {
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
			return float64(count)
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

	fileRetries = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_retries",
			Help: "Number of times a file has been retried",
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

func init() {
	prometheus.MustRegister(
		fileOps, fileBytes, fileDuration,
		uptime, systemInfo, cpuCount, ramUsage, currentHeap, gcCycles,
		cacheSize, filesInStore, filesInBackUp, exeCount,
		fileCopys, fileRetries, fileSorts, filtersLoadings, timeTilNextTick,
		// new in issue #104
		fileDeletions,
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
	uploadRejections.WithLabelValues("too_large")
}
