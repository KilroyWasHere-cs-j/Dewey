package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// dumpCompletedOnRe strips mysqldump's "-- Dump completed on <timestamp>"
// footer, the only line that differs between two dumps of otherwise
// identical data (confirmed by diffing two live back-to-back dumps).
var dumpCompletedOnRe = regexp.MustCompile(`(?m)^-- Dump completed on .*\n?`)

func normalizeDump(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading dump %q: %v", path, err)
	}
	return dumpCompletedOnRe.ReplaceAllString(string(content), "")
}

// walkFiles returns every regular file under dir as relative-path -> content,
// used to compare a directory tree's contents before and after a restore.
func walkFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	got := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got[rel] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %q: %v", dir, err)
	}
	return got
}

// TestManualLiveBackupRestoreE2E exercises the real backup/restore path end
// to end: real mysqldump/mysql against a live DB_DSN, plus a real zip
// round-trip through restoreFilesFromBackup. It uses disposable synthetic
// files rather than the real store/backup volumes, so it validates the live
// mechanics without touching production data.
//
// Both halves check state before AND after the restore: the DB is dumped
// again post-restore and diffed against the pre-restore dump (content must
// match, modulo mysqldump's own timestamp footer), and the store files are
// walked and compared file-by-file before vs. after.
//
// Skipped unless DEWEY_LIVE_E2E=1 — needs a reachable MySQL server and
// shells out to real mysqldump/mysql, so it's not part of the normal
// `go test ./...` run. This file is a one-off manual validation aid, not a
// permanent addition to the suite.
func TestManualLiveBackupRestoreE2E(t *testing.T) {
	if os.Getenv("DEWEY_LIVE_E2E") == "" {
		t.Skip("set DEWEY_LIVE_E2E=1 to run against a real live MySQL instance")
	}

	host, port, user, password, database, err := dbConnParamsFromDSN()
	if err != nil {
		t.Fatalf("dbConnParamsFromDSN: %v", err)
	}

	// --- DB: snapshot before, backup, restore, snapshot after, compare ---
	beforeDumpDir := t.TempDir()
	beforeDumpPath, err := backupMySQLDatabase(host, port, user, password, database, beforeDumpDir)
	if err != nil {
		t.Fatalf("real mysqldump (before) failed: %v", err)
	}
	beforeInfo, err := os.Stat(beforeDumpPath)
	if err != nil || beforeInfo.Size() == 0 {
		t.Fatalf("before-dump file missing or empty: err=%v", err)
	}
	t.Logf("before-restore dump written: %s (%d bytes)", beforeDumpPath, beforeInfo.Size())
	beforeContent := normalizeDump(t, beforeDumpPath)

	if err := restoreMySQLDatabase(host, port, user, password, database, beforeDumpPath); err != nil {
		t.Fatalf("real mysql restore failed: %v", err)
	}
	t.Log("real DB restore succeeded")

	afterDumpDir := t.TempDir()
	afterDumpPath, err := backupMySQLDatabase(host, port, user, password, database, afterDumpDir)
	if err != nil {
		t.Fatalf("real mysqldump (after) failed: %v", err)
	}
	afterContent := normalizeDump(t, afterDumpPath)

	if beforeContent != afterContent {
		t.Fatalf("DB content differs before vs. after restore (lengths %d vs %d) — restore did not reproduce the original data", len(beforeContent), len(afterContent))
	}
	t.Log("DB content verified identical before and after restore")

	// --- store files: snapshot source, backup, restore, compare trees ---
	sourceDir := t.TempDir()
	writeFile := func(rel, content string) {
		full := filepath.Join(sourceDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("claim1.txt", "hello claim 1")
	writeFile("2026/claim2.txt", "hello claim 2")
	writeFile("2026/nested/claim3.txt", "hello claim 3, nested deeper")

	beforeFiles := walkFiles(t, sourceDir)

	zipPath := filepath.Join(t.TempDir(), "20260101_000000_backup.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	walkErr := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(content)
		return err
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	destDir := t.TempDir()
	if err := restoreFilesFromBackup(zipPath, destDir); err != nil {
		t.Fatalf("restoreFilesFromBackup failed: %v", err)
	}

	afterFiles := walkFiles(t, destDir)

	if len(beforeFiles) != len(afterFiles) {
		t.Fatalf("file count differs before vs. after restore: before=%d after=%d", len(beforeFiles), len(afterFiles))
	}
	for rel, wantContent := range beforeFiles {
		gotContent, ok := afterFiles[rel]
		if !ok {
			t.Fatalf("%q present before restore but missing after", rel)
		}
		if gotContent != wantContent {
			t.Fatalf("%q content differs before vs. after restore: before=%q after=%q", rel, wantContent, gotContent)
		}
	}
	t.Log("store files verified identical before and after restore")
}

// TestManualLiveSaveLoadBackupChainedE2E exercises saveBackup and loadBackup
// themselves — the actual functions the daemon calls — rather than the
// lower-level backupMySQLDatabase/restoreMySQLDatabase/restoreFilesFromBackup
// primitives TestManualLiveBackupRestoreE2E targets directly. This is the
// specific case the dbDumpDir fix closed: loadBackup used to read a
// hardcoded filename saveBackup never actually wrote, so the two could never
// be chained together before.
//
// Because saveBackup/loadBackup use fixed relative paths ("store", dbDumpDir,
// backupDir) rather than taking them as arguments, this test chdirs into a
// scratch directory shaped like a real deployment. That chdir is
// process-wide, so this test must be run in isolation via
// -test.run TestManualLiveSaveLoadBackupChainedE2E, same as this file's
// other manual test.
//
// Skipped unless DEWEY_LIVE_E2E=1, for the same reasons as above.
func TestManualLiveSaveLoadBackupChainedE2E(t *testing.T) {
	if os.Getenv("DEWEY_LIVE_E2E") == "" {
		t.Skip("set DEWEY_LIVE_E2E=1 to run against a real live MySQL instance")
	}

	host, port, user, password, database, err := dbConnParamsFromDSN()
	if err != nil {
		t.Fatalf("dbConnParamsFromDSN: %v", err)
	}

	workDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	storeDir := filepath.Join(workDir, "store")
	writeFile := func(rel, content string) {
		full := filepath.Join(storeDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("claim1.txt", "hello claim 1")
	writeFile("2026/claim2.txt", "hello claim 2")
	beforeFiles := walkFiles(t, storeDir)

	// Independent DB snapshot, taken outside of saveBackup so the
	// before/after comparison doesn't depend on saveBackup's own dump.
	independentBeforeDir := t.TempDir()
	independentBeforePath, err := backupMySQLDatabase(host, port, user, password, database, independentBeforeDir)
	if err != nil {
		t.Fatalf("independent before-dump failed: %v", err)
	}
	beforeDBContent := normalizeDump(t, independentBeforePath)

	if err := saveBackup(); err != nil {
		t.Fatalf("saveBackup failed: %v", err)
	}
	t.Log("saveBackup succeeded")

	// Wipe store/ so loadBackup restoring it back proves the restore
	// actually repopulated it, rather than leaving pre-existing files alone.
	if err := os.RemoveAll(storeDir); err != nil {
		t.Fatal(err)
	}

	if err := loadBackup(); err != nil {
		t.Fatalf("loadBackup failed: %v", err)
	}
	t.Log("loadBackup succeeded")

	afterFiles := walkFiles(t, storeDir)
	if len(beforeFiles) != len(afterFiles) {
		t.Fatalf("file count differs before vs. after saveBackup+loadBackup: before=%d after=%d", len(beforeFiles), len(afterFiles))
	}
	for rel, wantContent := range beforeFiles {
		gotContent, ok := afterFiles[rel]
		if !ok {
			t.Fatalf("%q present before saveBackup but missing after loadBackup", rel)
		}
		if gotContent != wantContent {
			t.Fatalf("%q content differs before vs. after: before=%q after=%q", rel, wantContent, gotContent)
		}
	}
	t.Log("store/ verified identical before saveBackup and after loadBackup")

	independentAfterDir := t.TempDir()
	independentAfterPath, err := backupMySQLDatabase(host, port, user, password, database, independentAfterDir)
	if err != nil {
		t.Fatalf("independent after-dump failed: %v", err)
	}
	afterDBContent := normalizeDump(t, independentAfterPath)

	if beforeDBContent != afterDBContent {
		t.Fatalf("DB content differs before saveBackup vs. after loadBackup (lengths %d vs %d)", len(beforeDBContent), len(afterDBContent))
	}
	t.Log("DB content verified identical before saveBackup and after loadBackup")
}
