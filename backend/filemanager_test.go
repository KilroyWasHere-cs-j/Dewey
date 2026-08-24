package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestGinContext returns a *gin.Context backed by a ResponseRecorder, so
// handler-style functions under test can be called directly and their
// JSON response inspected via the returned recorder.
func newTestGinContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
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
			c, w := newTestGinContext(t)
			fh := &multipart.FileHeader{Filename: "big.pdf", Size: tt.size}

			got := checkFileSize(fh, c)

			if got != tt.wantOK {
				t.Fatalf("checkFileSize() = %v, want %v", got, tt.wantOK)
			}
			if !tt.wantOK && w.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("response code = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
			}
		})
	}
}

// TestCreateTimestamp checks the "<unix-seconds>_<filename>" format
// uploadFile relies on to generate a collision-resistant stored filename.
func TestCreateTimestamp(t *testing.T) {
	got := CreateTimestamp("report.pdf")

	if !strings.HasSuffix(got, "_report.pdf") {
		t.Fatalf("CreateTimestamp(%q) = %q, want suffix %q", "report.pdf", got, "_report.pdf")
	}

	prefix := strings.TrimSuffix(got, "_report.pdf")
	if _, err := strconv.ParseInt(prefix, 10, 64); err != nil {
		t.Fatalf("CreateTimestamp(%q) = %q, timestamp prefix not numeric: %v", "report.pdf", got, err)
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

	c, _ := newTestGinContext(t)
	err, got := CreateFileHash(file, c)
	if err != nil {
		t.Fatalf("CreateFileHash() unexpected error: %v", err)
	}
	if got != wantHex {
		t.Fatalf("CreateFileHash() = %q, want %q", got, wantHex)
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

	c, _ := newTestGinContext(t)
	if err := SaveFile("saved.txt", file, c); err != nil {
		t.Fatalf("SaveFile() unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(uploadDir, "saved.txt"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("saved content = %q, want %q", got, content)
	}
}

// TestOpenFile confirms the extracted open step returns a readable handle
// to the uploaded file's actual content.
func TestOpenFile(t *testing.T) {
	content := []byte("openable")
	fh := newUploadedFile(t, "note.txt", content)

	c, _ := newTestGinContext(t)
	err, file := OpenFile(fh, c)
	if err != nil {
		t.Fatalf("OpenFile() unexpected error: %v", err)
	}
	defer file.Close()

	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("reading opened file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("OpenFile content = %q, want %q", got, content)
	}
}
