package main

import (
	"os"
	"io"
	"time"
	"net/http"
	"context"
	"archive/zip"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
  "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "active_connections",
        Help: "Number of active connections",
    })
)

func init() {
    prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, activeConnections)
}

// Middleware to instrument handlers
func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        activeConnections.Inc()
        defer activeConnections.Dec()

        // Wrap ResponseWriter to capture status code
        rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        next.ServeHTTP(rw, r)

        duration := time.Since(start).Seconds()
        status := http.StatusText(rw.statusCode)

        httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
        httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func startPrometheus() {
	mux := http.NewServeMux()

	// Prometheus scrape endpoint
	mux.Handle("/metrics", promhttp.Handler())
	// Wrap everything with metrics middleware
	http.ListenAndServe(prometheusServer, metricsMiddleware(mux))
}

//
// startDaemon launches a background worker that periodically runs maintenance tasks.
//
// Behavior:
//   - Runs runTask every n hours
//   - Stops cleanly when context is cancelled
//
// Args:
//   - ctx: context used to signal shutdown (cancellation-safe goroutine)
func startDaemon(ctx context.Context) {
	// Debug("starting cache clear daemon")

	ticker := time.NewTicker(daemonTickTime * time.Minute)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// run task safely so panic won't kill goroutine
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Warn("daemon panic recovered")
						}
					}()
					dumpCache()
					save()
				}()

			case <-ctx.Done():
				// Warn("daemon stopped")
				return
			}
		}
	}()
}

// runTask executes periodic maintenance logic such as cache cleanup.
func dumpCache() {
	// Debug("Running system cache dump")
	entries, err := os.ReadDir(uploadDir) // Read current directory
  if err != nil {
		// Fatal("Failed to dump cache dir " + err.Error())
  }

  for _, entry := range entries {
		if !entry.IsDir() {
			err := os.Remove(uploadDir + "/" + entry.Name())
			if err != nil {
				// Fatal("Failed to remove a file from the cache " + err.Error())
			}
		}	
  }
	// Debug("Cache dumped")
}

func save() error {
	Debug("Creating zip backup")

	sourceDir := "store"
	// Ensure backup directory exists
	backupDir := "backup"
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
		Debug("Zip written")
		return err
	})
}
