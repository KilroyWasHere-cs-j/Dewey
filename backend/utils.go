package main

import (
	"io"
	"os"
	"sync"
	"time"

	"archive/zip"
	"context"
	"path/filepath"

	"github.com/gin-gonic/gin"
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

var daemonTicker *observableTicker

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

func startDaemon(ctx context.Context, pm *PluginManager) {
	// Debug("starting cache clear daemon")

	daemonTicker = newObservableTicker(time.Duration(daemonTickTime) * time.Hour)

	go func() {
		defer daemonTicker.Stop()

		for {
			select {
			case t := <-daemonTicker.Chan():
				// record tick time so Remaining() can be observed
				daemonTicker.markTick(t)
				// run task safely so panic won't kill goroutine
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Warn("daemon panic recovered")
						}
					}()
					dumpCache()
					save()
					pm.RunPlugins("tick")
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
	if daemonTicker == nil {
		return time.Duration(-1)
	}
	return daemonTicker.Remaining()
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
		return err
	})
}

func PluginMiddleware() gin.HandlerFunc {
	pm := NewPluginManager()
	pm.LoadPlugins()
	pm.RunPlugins("init")

	return func(c *gin.Context) {
		Debug("Loading plugins...")
		c.Set("plugins", pm)
		c.Next()
	}
}
