package main

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
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
					if err := dumpCache(); err != nil {
						Warn("Cache clear incomplete: " + err.Error())
					}

					err := save()
					if err == nil {
						atomic.AddInt64(&FilesInBackUp, 1)
					}
					pm.RunPlugins(Tick)
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

// dumpCache clears all files from the upload cache directory. Best-effort:
// a failure removing one entry doesn't stop it from attempting the rest, so
// one stuck file can't block cleanup forever. FilesInCache is decremented
// per file actually removed rather than reset in bulk, so it can't be
// clobbered by a file uploaded concurrently with this pass (which wouldn't
// have been in the directory snapshot below anyway). Returns the first
// error encountered, if any, so the caller knows the clear was incomplete.
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
		atomic.AddInt64(&FilesInCache, -1)
	}
	return firstErr
}

func save() error {
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
