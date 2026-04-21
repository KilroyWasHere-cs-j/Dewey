package main

import (
	"os"
	"io"
	"time"
	"context"
	"archive/zip"
	"path/filepath"
)

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
	zipPath := "back_up.zip"

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

		// Skip directories (we only add files, but keep structure via headers)
		if d.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Create path inside zip (relative to sourceDir)
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
