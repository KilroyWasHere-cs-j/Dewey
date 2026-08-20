package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeBinary drops an executable shell script named name into dir,
// standing in for mysqldump/mysql so backupMySQLDatabase/restoreMySQLDatabase
// can be exercised without a real MySQL server. body is the script content
// after the shebang line.
func writeFakeBinary(t *testing.T, dir, name, body string) {
	t.Helper()

	script := "#!/bin/sh\n" + body
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake %s: %v", name, err)
	}
}

// useFakeBinDir prepends dir to PATH for the duration of the test, so
// exec.Command("mysqldump"/"mysql", ...) resolves to a fake script instead
// of the real binary.
func useFakeBinDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBackupMySQLDatabaseWritesDumpOnSuccess(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBinary(t, binDir, "mysqldump", "printf 'DUMP CONTENTS'\nexit 0\n")
	useFakeBinDir(t, binDir)

	outDir := t.TempDir()
	path, err := backupMySQLDatabase("127.0.0.1", "3306", "root", "secret", "deweyRecords", outDir)
	if err != nil {
		t.Fatalf("backupMySQLDatabase returned unexpected error: %v", err)
	}

	if !strings.HasPrefix(filepath.Base(path), "deweyRecords_") {
		t.Fatalf("backup filename %q does not start with database name", path)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading backup file: %v", err)
	}
	if string(got) != "DUMP CONTENTS" {
		t.Fatalf("backup file contents = %q, want %q", got, "DUMP CONTENTS")
	}
}

func TestBackupMySQLDatabaseCleansUpOnCommandFailure(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBinary(t, binDir, "mysqldump", "printf 'access denied' >&2\nexit 1\n")
	useFakeBinDir(t, binDir)

	outDir := t.TempDir()
	_, err := backupMySQLDatabase("127.0.0.1", "3306", "root", "wrong-password", "deweyRecords", outDir)
	if err == nil {
		t.Fatal("backupMySQLDatabase returned nil error, want failure surfaced")
	}
	if !strings.Contains(err.Error(), "mysqldump failed") || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("error = %q, want it to mention the mysqldump failure and stderr", err)
	}

	entries, readErr := os.ReadDir(outDir)
	if readErr != nil {
		t.Fatalf("reading outDir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("outDir has %d leftover file(s) after a failed dump, want the partial file removed", len(entries))
	}
}

func TestBackupMySQLDatabaseErrorsWhenOutDirCannotBeCreated(t *testing.T) {
	// outDir points at a path where a regular file already sits, so
	// os.MkdirAll can't create a directory there.
	parent := t.TempDir()
	blocked := filepath.Join(parent, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("setting up blocking file: %v", err)
	}

	_, err := backupMySQLDatabase("127.0.0.1", "3306", "root", "secret", "deweyRecords", blocked)
	if err == nil {
		t.Fatal("backupMySQLDatabase returned nil error, want a mkdir failure")
	}
	if !strings.Contains(err.Error(), "creating backup dir") {
		t.Fatalf("error = %q, want it to mention creating backup dir", err)
	}
}

func TestRestoreMySQLDatabaseErrorsWhenDumpFileMissing(t *testing.T) {
	err := restoreMySQLDatabase("127.0.0.1", "3306", "root", "secret", "deweyRecords", filepath.Join(t.TempDir(), "missing.sql"))
	if err == nil {
		t.Fatal("restoreMySQLDatabase returned nil error, want a file-open failure")
	}
	if !strings.Contains(err.Error(), "opening dump file") {
		t.Fatalf("error = %q, want it to mention opening dump file", err)
	}
}

func TestRestoreMySQLDatabasePipesDumpFileToCommandOnSuccess(t *testing.T) {
	binDir := t.TempDir()
	receivedPath := filepath.Join(t.TempDir(), "received.sql")
	// cat stdin to a known location so the test can verify the dump file's
	// contents actually reached the command, not just that it exited 0.
	writeFakeBinary(t, binDir, "mysql", "cat > \""+receivedPath+"\"\nexit 0\n")
	useFakeBinDir(t, binDir)

	dumpPath := filepath.Join(t.TempDir(), "dump.sql")
	if err := os.WriteFile(dumpPath, []byte("RESTORE CONTENTS"), 0o644); err != nil {
		t.Fatalf("setting up dump file: %v", err)
	}

	if err := restoreMySQLDatabase("127.0.0.1", "3306", "root", "secret", "deweyRecords", dumpPath); err != nil {
		t.Fatalf("restoreMySQLDatabase returned unexpected error: %v", err)
	}

	got, err := os.ReadFile(receivedPath)
	if err != nil {
		t.Fatalf("reading what the fake mysql binary received: %v", err)
	}
	if string(got) != "RESTORE CONTENTS" {
		t.Fatalf("command received %q on stdin, want %q", got, "RESTORE CONTENTS")
	}
}

func TestRestoreMySQLDatabaseSurfacesCommandFailure(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBinary(t, binDir, "mysql", "printf 'syntax error' >&2\nexit 1\n")
	useFakeBinDir(t, binDir)

	dumpPath := filepath.Join(t.TempDir(), "dump.sql")
	if err := os.WriteFile(dumpPath, []byte("bad sql"), 0o644); err != nil {
		t.Fatalf("setting up dump file: %v", err)
	}

	err := restoreMySQLDatabase("127.0.0.1", "3306", "root", "secret", "deweyRecords", dumpPath)
	if err == nil {
		t.Fatal("restoreMySQLDatabase returned nil error, want failure surfaced")
	}
	if !strings.Contains(err.Error(), "mysql restore failed") || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("error = %q, want it to mention the mysql failure and stderr", err)
	}
}

func TestStderrCollectorAccumulatesWrites(t *testing.T) {
	var buf []byte
	w := &stderrCollector{buf: &buf}

	n, err := w.Write([]byte("hello "))
	if err != nil || n != len("hello ") {
		t.Fatalf("Write(%q) = (%d, %v), want (%d, nil)", "hello ", n, err, len("hello "))
	}
	n, err = w.Write([]byte("world"))
	if err != nil || n != len("world") {
		t.Fatalf("Write(%q) = (%d, %v), want (%d, nil)", "world", n, err, len("world"))
	}

	if string(buf) != "hello world" {
		t.Fatalf("accumulated buf = %q, want %q", buf, "hello world")
	}
}

func TestDBConnParamsFromDSNRequiresDBDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")

	_, _, _, _, _, err := dbConnParamsFromDSN()
	if err == nil {
		t.Fatal("dbConnParamsFromDSN returned nil error, want DB_DSN-required failure")
	}
	if !strings.Contains(err.Error(), "DB_DSN") {
		t.Fatalf("error = %q, want it to mention DB_DSN", err)
	}
}

func TestDBConnParamsFromDSNParsesValidDSN(t *testing.T) {
	t.Setenv("DB_DSN", "root:secret@tcp(127.0.0.1:3306)/deweyRecords")

	host, port, user, password, database, err := dbConnParamsFromDSN()
	if err != nil {
		t.Fatalf("dbConnParamsFromDSN returned unexpected error: %v", err)
	}

	want := struct{ host, port, user, password, database string }{"127.0.0.1", "3306", "root", "secret", "deweyRecords"}
	got := struct{ host, port, user, password, database string }{host, port, user, password, database}
	if got != want {
		t.Fatalf("dbConnParamsFromDSN() = %+v, want %+v", got, want)
	}
}

func TestDBConnParamsFromDSNErrorsOnMalformedDSN(t *testing.T) {
	// Unbalanced parens in the address portion is invalid DSN syntax.
	t.Setenv("DB_DSN", "root:secret@tcp(127.0.0.1:3306/deweyRecords")

	_, _, _, _, _, err := dbConnParamsFromDSN()
	if err == nil {
		t.Fatal("dbConnParamsFromDSN returned nil error, want a parse failure")
	}
	if !strings.Contains(err.Error(), "parsing DB_DSN") {
		t.Fatalf("error = %q, want it to mention parsing DB_DSN", err)
	}
}

func TestSaveBackupFailsFastWithoutDBDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")

	err := saveBackup()
	if err == nil {
		t.Fatal("saveBackup returned nil error, want DB_DSN-required failure")
	}
	if !strings.Contains(err.Error(), "DB_DSN") {
		t.Fatalf("error = %q, want it to mention DB_DSN", err)
	}
}

func TestLoadBackupFailsFastWithoutDBDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")

	err := loadBackup()
	if err == nil {
		t.Fatal("loadBackup returned nil error, want DB_DSN-required failure")
	}
	if !strings.Contains(err.Error(), "DB_DSN") {
		t.Fatalf("error = %q, want it to mention DB_DSN", err)
	}
}
