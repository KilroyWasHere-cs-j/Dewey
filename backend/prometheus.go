package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var appName = "crossApp"
var startTime = time.Now()
var FileSorts = 0
var FileRetrievals = 0
var FileDeletions = 0
var FiltersLoadings = 0
var FileCopys = 0

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
				panic(err)
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
			return float64(count - 1) // This removes out the db file
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
			return float64(PECount + ELFCount)
		},
	)

	filtersLoadings = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_filters_loadings",
			Help: "Number of times the filters configuration has been loaded",
		},
		func() float64 {
			return float64(FiltersLoadings)
		},
	)

	fileCopys = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_copys",
			Help: "Number of times a file has been copied",
		},
		func() float64 {
			return float64(FileCopys)
		},
	)

	fileRetries = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_retries",
			Help: "Number of times a file has been retried",
		},
		func() float64 {
			return float64(FileRetrievals)
		},
	)

	fileSorts = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "app_file_sorts",
			Help: "Number of times a file has been sorted",
		},
		func() float64 {
			return float64(FileSorts)
		},
	)
)

func init() {
	prometheus.MustRegister(fileOps, fileBytes, fileDuration, uptime, systemInfo, cpuCount,
		ramUsage, currentHeap, gcCycles, cacheSize, filesInStore, filesInBackUp, exeCount,
		fileCopys, fileRetries, fileSorts,
	)

	// create zero-valued label instances so metrics appear even before traffic
	fileBytes.WithLabelValues("read")
	fileBytes.WithLabelValues("write")

	fileOps.WithLabelValues("read", "ok", "default")
	fileOps.WithLabelValues("write", "ok", "default")

	fileDuration.WithLabelValues("read")
	fileDuration.WithLabelValues("write")
}
