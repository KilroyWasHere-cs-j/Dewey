package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// wantAPIErrorStatus fails the test unless err is an *apiError carrying
// wantStatus — the validation/storage helpers in filemanager.go/
// filevalidator.go report their intended HTTP response this way instead of
// writing to a *gin.Context directly.
func wantAPIErrorStatus(t *testing.T, err error, wantStatus int) {
	t.Helper()
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *apiError, got %T (%v)", err, err)
	}
	if apiErr.status != wantStatus {
		t.Fatalf("apiError.status = %d, want %d", apiErr.status, wantStatus)
	}
}

// newUploadedFile builds a real *multipart.FileHeader backed by an actual
// multipart form body (not just a struct literal), for tests that need to
// Open() and read the file's content rather than just its Filename/Size.
func newUploadedFile(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("writing form file content: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	reader := multipart.NewReader(&buf, w.Boundary())
	form, err := reader.ReadForm(int64(len(content)) + 1024)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	t.Cleanup(func() { form.RemoveAll() })

	return form.File["file"][0]
}

// TestResolveStorePath exercises the boundary check idAndSort relies on
// (issue #202) to stop a filter plugin's attacker-controlled entry.Path from
// writing outside fileSystemBaseDir.
func TestResolveStorePath(t *testing.T) {
	base := filepath.FromSlash("/app/store")

	tests := []struct {
		name    string
		rel     string
		want    string
		wantErr bool
	}{
		{
			name: "plain relative path stays under base",
			rel:  "2024/claim.pdf",
			want: filepath.FromSlash("/app/store/2024/claim.pdf"),
		},
		{
			name: "empty path resolves to base itself",
			rel:  "",
			want: base,
		},
		{
			name:    "traversal escaping base entirely",
			rel:     "../../etc/cron.d/x",
			wantErr: true,
		},
		{
			name:    "traversal landing on a sibling directory sharing base's name as a prefix",
			rel:     "../store-evil/x",
			wantErr: true,
		},
		{
			name: "traversal that stays inside base after cleaning",
			rel:  "2024/../2025/claim.pdf",
			want: filepath.FromSlash("/app/store/2025/claim.pdf"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveStorePath(base, tt.rel)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveStorePath(%q, %q) = %q, want error", base, tt.rel, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveStorePath(%q, %q) unexpected error: %v", base, tt.rel, err)
			}
			if got != tt.want {
				t.Fatalf("resolveStorePath(%q, %q) = %q, want %q", base, tt.rel, got, tt.want)
			}
		})
	}
}

// TestCheckFileSize exercises the "belt-and-suspenders" declared-size check
// (issue #233's extraction of uploadFile's size check into its own
// function) against the real maxFileSize loaded from config.json by
// TestMain's load() call.
func TestCheckFileSize(t *testing.T) {
	tests := []struct {
		name   string
		size   int64
		wantOK bool
	}{
		{name: "under limit passes", size: maxFileSize - 1, wantOK: true},
		{name: "exactly at limit passes", size: maxFileSize, wantOK: true},
		{name: "over limit is rejected", size: maxFileSize + 1, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fh := &multipart.FileHeader{Filename: "big.pdf", Size: tt.size}

			err := checkFileSize(fh)

			if got := err == nil; got != tt.wantOK {
				t.Fatalf("checkFileSize() ok = %v, want %v", got, tt.wantOK)
			}
			if !tt.wantOK {
				wantAPIErrorStatus(t, err, http.StatusRequestEntityTooLarge)
			}
		})
	}
}

// TestCreateTimestamp checks the "<unix-seconds>_<filename>_<random-suffix>"
// format uploadFile relies on to generate a collision-resistant stored
// filename (issue #367 — a bare unix-second timestamp let concurrent
// uploads generate identical filenames and silently overwrite each other).
func TestCreateTimestamp(t *testing.T) {
	got, err := createTimestamp("report.pdf")
	if err != nil {
		t.Fatalf("createTimestamp(%q) unexpected error: %v", "report.pdf", err)
	}

	want := regexp.MustCompile(`^\d+_report\.pdf_[0-9a-f]{20}$`)
	if !want.MatchString(got) {
		t.Fatalf("createTimestamp(%q) = %q, want format <unix>_report.pdf_<20 hex chars>", "report.pdf", got)
	}
}

// TestCreateTimestampUnique confirms two calls for the same filename don't
// collide — the random suffix, not the timestamp, is what makes the
// generated filename collision-resistant when two uploads land in the same
// wall-clock second (issue #367).
func TestCreateTimestampUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		got, err := createTimestamp("report.pdf")
		if err != nil {
			t.Fatalf("createTimestamp(%q) unexpected error: %v", "report.pdf", err)
		}
		if seen[got] {
			t.Fatalf("createTimestamp(%q) produced a duplicate: %q", "report.pdf", got)
		}
		seen[got] = true
	}
}

// TestCreateFileHash confirms the extracted hashing step still computes the
// same SHA-256 digest the original inline code did.
func TestCreateFileHash(t *testing.T) {
	content := []byte("hello dewey")
	wantSum := sha256.Sum256(content)
	wantHex := hex.EncodeToString(wantSum[:])

	fh := newUploadedFile(t, "note.txt", content)
	file, err := fh.Open()
	if err != nil {
		t.Fatalf("opening fixture file: %v", err)
	}
	defer file.Close()

	got, err := createFileHash(file)
	if err != nil {
		t.Fatalf("createFileHash() unexpected error: %v", err)
	}
	if got != wantHex {
		t.Fatalf("createFileHash() = %q, want %q", got, wantHex)
	}
}

// TestSaveFile confirms the extracted save step writes the file's full
// content to uploadDir under the given name, rewinding first as the
// original inline code did (issue #166 — file has usually already been
// read once for hashing by this point).
func TestSaveFile(t *testing.T) {
	origUploadDir := uploadDir
	uploadDir = t.TempDir()
	t.Cleanup(func() { uploadDir = origUploadDir })

	content := []byte("save me")
	fh := newUploadedFile(t, "note.txt", content)
	file, err := fh.Open()
	if err != nil {
		t.Fatalf("opening fixture file: %v", err)
	}
	defer file.Close()

	if err := saveFile("saved.txt", file); err != nil {
		t.Fatalf("saveFile() unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(uploadDir, "saved.txt"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("saved content = %q, want %q", got, content)
	}
}

// TestSaveFileRejectsExistingDestination confirms saveFile refuses to
// overwrite an existing file under uploadDir instead of silently
// truncating it (issue #367 — saveFile is the request's actual first
// write, before the async copyFile move into the permanent store).
func TestSaveFileRejectsExistingDestination(t *testing.T) {
	origUploadDir := uploadDir
	uploadDir = t.TempDir()
	t.Cleanup(func() { uploadDir = origUploadDir })

	dst := filepath.Join(uploadDir, "saved.txt")
	if err := os.WriteFile(dst, []byte("original content"), 0600); err != nil {
		t.Fatalf("writing pre-existing fixture: %v", err)
	}

	fh := newUploadedFile(t, "note.txt", []byte("new content"))
	file, err := fh.Open()
	if err != nil {
		t.Fatalf("opening fixture file: %v", err)
	}
	defer file.Close()

	if err := saveFile("saved.txt", file); err == nil {
		t.Fatalf("saveFile() with existing destination = nil error, want an error")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst after saveFile: %v", err)
	}
	if string(got) != "original content" {
		t.Fatalf("dst content = %q after rejected saveFile, want original content preserved", got)
	}
}

// TestCopyFileRejectsExistingDestination confirms copyFile refuses to
// overwrite an existing destination instead of silently truncating it
// (issue #367 — the other half of the collision: even with a unique
// filename, an unconditional os.Create(dst) would still clobber a file that
// legitimately already sits at that path with no error).
func TestCopyFileRejectsExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("new content"), 0600); err != nil {
		t.Fatalf("writing src fixture: %v", err)
	}
	if err := os.WriteFile(dst, []byte("original content"), 0600); err != nil {
		t.Fatalf("writing dst fixture: %v", err)
	}

	if err := copyFile(src, dst); err == nil {
		t.Fatalf("copyFile() with existing destination = nil error, want an error")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst after copyFile: %v", err)
	}
	if string(got) != "original content" {
		t.Fatalf("dst content = %q after rejected copyFile, want original content preserved", got)
	}
}

// TestCopyFileSucceedsForNewDestination confirms the exists check doesn't
// block the ordinary case of copying to a path that doesn't exist yet.
func TestCopyFileSucceedsForNewDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "sub", "dst.txt")

	content := []byte("hello")
	if err := os.WriteFile(src, content, 0600); err != nil {
		t.Fatalf("writing src fixture: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() unexpected error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("dst content = %q, want %q", got, content)
	}
}

// TestValidateMetaData exercises the issue #365 guard — date_of_injury is a
// NOT NULL column, and previously a blank value reached the DB insert
// silently since it happens in a post-processing goroutine after the upload
// response is already sent.
func TestValidateMetaData(t *testing.T) {
	tests := []struct {
		name    string
		meta    MetaData
		wantErr bool
	}{
		{
			name:    "valid date_of_injury passes",
			meta:    MetaData{DateOfInjury: "2025-01-01"},
			wantErr: false,
		},
		{
			name:    "empty date_of_injury is rejected",
			meta:    MetaData{DateOfInjury: ""},
			wantErr: true,
		},
		{
			name:    "whitespace-only date_of_injury is rejected",
			meta:    MetaData{DateOfInjury: "   "},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetaData(tt.meta)
			if got := err != nil; got != tt.wantErr {
				t.Fatalf("validateMetaData(%+v) error = %v, wantErr %v", tt.meta, err, tt.wantErr)
			}
		})
	}
}

// TestOpenFile confirms the extracted open step returns a readable handle
// to the uploaded file's actual content.
func TestOpenFile(t *testing.T) {
	content := []byte("openable")
	fh := newUploadedFile(t, "note.txt", content)

	file, err := openFile(fh)
	if err != nil {
		t.Fatalf("openFile() unexpected error: %v", err)
	}
	defer file.Close()

	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("reading opened file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("openFile content = %q, want %q", got, content)
	}
}

// TestViewFile exercises the helper both getViewFile branches (log, config)
// call — a baseDir+name join and read, with name stripped to its base
// filename first so a ".."-bearing name can't escape baseDir (issue #410).
func TestViewFile(t *testing.T) {
	dir := t.TempDir()
	content := "line one\nline two\n"
	if err := os.WriteFile(filepath.Join(dir, "app.log"), []byte(content), 0600); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	t.Run("existing file returns its content", func(t *testing.T) {
		got, err := viewFile(dir, "app.log")
		if err != nil {
			t.Fatalf("viewFile() unexpected error: %v", err)
		}
		if got != content {
			t.Fatalf("viewFile() = %q, want %q", got, content)
		}
	})

	t.Run("missing file returns an error", func(t *testing.T) {
		if _, err := viewFile(dir, "does-not-exist.log"); err == nil {
			t.Fatalf("viewFile() with missing file = nil error, want an error")
		}
	})

	t.Run("name escaping baseDir via .. is rejected", func(t *testing.T) {
		// Confirms the path-traversal fix (issue #410): viewFile strips
		// name to its base filename before joining, so a ".."-bearing name
		// can no longer resolve to a real file outside dir.
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside"), 0600); err != nil {
			t.Fatalf("writing outside fixture: %v", err)
		}

		rel, err := filepath.Rel(dir, filepath.Join(outside, "secret.txt"))
		if err != nil {
			t.Fatalf("computing relative path: %v", err)
		}

		if _, err := viewFile(dir, rel); err == nil {
			t.Fatalf("viewFile(%q) = nil error, want an error since the traversal should be stripped to a bare filename that doesn't exist in dir", rel)
		}
	})
}
