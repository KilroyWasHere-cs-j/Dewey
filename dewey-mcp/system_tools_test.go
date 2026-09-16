package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateFile confirms CreateFile writes its contents to the given path.
func TestCreateFile(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := CreateFile("hello dewey", "test.txt"); err != nil {
		t.Fatalf("CreateFile() unexpected error: %v", err)
	}

	got, err := os.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("reading test.txt: %v", err)
	}
	if string(got) != "hello dewey" {
		t.Fatalf("test.txt content = %q, want %q", got, "hello dewey")
	}
}

// TestCreateFile_Overwrites confirms a second call truncates rather than
// appending, since os.Create always opens with O_TRUNC.
func TestCreateFile_Overwrites(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := CreateFile("first", "test.txt"); err != nil {
		t.Fatalf("CreateFile() first call unexpected error: %v", err)
	}
	if err := CreateFile("second", "test.txt"); err != nil {
		t.Fatalf("CreateFile() second call unexpected error: %v", err)
	}

	got, err := os.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("reading test.txt: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("test.txt content = %q, want %q", got, "second")
	}
}

// TestCreateFile_PathTraversal confirms CreateFile refuses to write outside
// the working directory.
func TestCreateFile_PathTraversal(t *testing.T) {
	parent := t.TempDir()
	workDir := filepath.Join(parent, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatalf("creating work dir: %v", err)
	}
	t.Chdir(workDir)

	if err := CreateFile("payload", "../escaped.txt"); err == nil {
		t.Fatalf("CreateFile with escaping path = nil, want error")
	}
	if _, err := os.Stat(filepath.Join(parent, "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("escaping file was created outside the working directory (stat err = %v)", err)
	}
}

// TestReadFile confirms ReadFile returns a file's full contents when given a
// path inside the working directory.
func TestReadFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("note.txt", []byte("read me"), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	got, err := ReadFile("note.txt")
	if err != nil {
		t.Fatalf("ReadFile() unexpected error: %v", err)
	}
	if got != "read me" {
		t.Fatalf("ReadFile() = %q, want %q", got, "read me")
	}
}

// TestReadFile_MissingFile confirms ReadFile surfaces the underlying error
// rather than returning an empty string silently.
func TestReadFile_MissingFile(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := ReadFile("missing.txt")
	if err == nil {
		t.Fatalf("ReadFile(%q) = %q, want error", "missing.txt", got)
	}
}

// TestReadFile_PathTraversal confirms ReadFile refuses to read a file
// outside the working directory, even when it exists and is reachable via
// "../" from cwd — the fix for the path-traversal vulnerability where any
// MCP client could read arbitrary files on the host.
func TestReadFile_PathTraversal(t *testing.T) {
	parent := t.TempDir()
	secret := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	workDir := filepath.Join(parent, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatalf("creating work dir: %v", err)
	}
	t.Chdir(workDir)

	got, err := ReadFile("../secret.txt")
	if err == nil {
		t.Fatalf("ReadFile(%q) = %q, want error escaping working directory", "../secret.txt", got)
	}
	if !strings.Contains(err.Error(), "escapes working directory") {
		t.Fatalf("ReadFile(%q) error = %q, want it to mention escaping the working directory", "../secret.txt", err.Error())
	}
}

// TestMoveFile confirms a successful move relocates the file's content from
// src to dst and leaves src gone.
func TestMoveFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("src.txt", []byte("payload"), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	if err := MoveFile("src.txt", "dst.txt"); err != nil {
		t.Fatalf("MoveFile() unexpected error: %v", err)
	}

	if _, err := os.Stat("src.txt"); !os.IsNotExist(err) {
		t.Fatalf("src still exists after move (stat err = %v)", err)
	}
	got, err := os.ReadFile("dst.txt")
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("dst content = %q, want %q", got, "payload")
	}
}

// TestMoveFile_DstExists confirms MoveFile skips the move rather than
// overwriting an existing destination, leaving both files as they were.
func TestMoveFile_DstExists(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("src.txt", []byte("src content"), 0o644); err != nil {
		t.Fatalf("writing src fixture: %v", err)
	}
	if err := os.WriteFile("dst.txt", []byte("dst content"), 0o644); err != nil {
		t.Fatalf("writing dst fixture: %v", err)
	}

	if err := MoveFile("src.txt", "dst.txt"); err != nil {
		t.Fatalf("MoveFile() unexpected error: %v", err)
	}

	if _, err := os.Stat("src.txt"); err != nil {
		t.Fatalf("src should still exist, stat error: %v", err)
	}
	got, err := os.ReadFile("dst.txt")
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != "dst content" {
		t.Fatalf("dst content = %q, want unchanged %q", got, "dst content")
	}
}

// TestMoveFile_MissingSrc confirms MoveFile errors when src doesn't exist.
func TestMoveFile_MissingSrc(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := MoveFile("missing.txt", "dst.txt"); err == nil {
		t.Fatalf("MoveFile(%q, %q) = nil, want error", "missing.txt", "dst.txt")
	}
}

// TestMoveFile_PathTraversal confirms MoveFile refuses a src or dst that
// escapes the working directory, in either position.
func TestMoveFile_PathTraversal(t *testing.T) {
	parent := t.TempDir()
	workDir := filepath.Join(parent, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatalf("creating work dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "src.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("writing src fixture: %v", err)
	}
	t.Chdir(workDir)

	if err := MoveFile("src.txt", "../escaped.txt"); err == nil {
		t.Fatalf("MoveFile with escaping dst = nil, want error")
	}
	if err := MoveFile("../outside.txt", "dst.txt"); err == nil {
		t.Fatalf("MoveFile with escaping src = nil, want error")
	}

	if _, err := os.Stat(filepath.Join(parent, "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("escaping dst was created outside the working directory (stat err = %v)", err)
	}
}

// TestDeleteFile confirms DeleteFile removes an existing file.
func TestDeleteFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("gone.txt", []byte("bye"), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	if err := DeleteFile("gone.txt"); err != nil {
		t.Fatalf("DeleteFile() unexpected error: %v", err)
	}

	if _, err := os.Stat("gone.txt"); !os.IsNotExist(err) {
		t.Fatalf("file still exists after delete (stat err = %v)", err)
	}
}

// TestDeleteFile_MissingFile confirms DeleteFile errors when the file
// doesn't exist rather than treating it as a no-op.
func TestDeleteFile_MissingFile(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := DeleteFile("missing.txt"); err == nil {
		t.Fatalf("DeleteFile(%q) = nil, want error", "missing.txt")
	}
}

// TestDeleteFile_PathTraversal confirms DeleteFile refuses to remove a file
// outside the working directory — without this, any MCP client could delete
// arbitrary files on the host via a "../" path.
func TestDeleteFile_PathTraversal(t *testing.T) {
	parent := t.TempDir()
	victim := filepath.Join(parent, "victim.txt")
	if err := os.WriteFile(victim, []byte("do not delete me"), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	workDir := filepath.Join(parent, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatalf("creating work dir: %v", err)
	}
	t.Chdir(workDir)

	if err := DeleteFile("../victim.txt"); err == nil {
		t.Fatalf("DeleteFile(%q) = nil, want error escaping working directory", "../victim.txt")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("victim file should still exist, stat error: %v", err)
	}
}
