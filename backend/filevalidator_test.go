package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"testing"
)

// buildPEBytes constructs a minimal DOS+PE header: a 64-byte DOS header with
// the "MZ" signature and e_lfanew set to peOffset, followed by peSig at that
// offset. Mirrors exactly what IsPEFile parses, nothing more.
func buildPEBytes(peOffset uint32, peSig [4]byte) []byte {
	b := make([]byte, int(peOffset)+4)
	b[0], b[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(b[0x3C:0x40], peOffset)
	copy(b[peOffset:], peSig[:])
	return b
}

// errReader is an io.ReadSeeker whose Read always fails with a non-EOF
// error, for exercising the genuine I/O-error paths that io.ReadFull's
// EOF-tolerant callers don't hit.
type errReader struct{}

func (errReader) Read(p []byte) (int, error)                   { return 0, errors.New("boom") }
func (errReader) Seek(offset int64, whence int) (int64, error) { return 0, nil }

func TestIsPEFile(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    bool
		wantErr bool
	}{
		{
			name: "valid PE",
			data: buildPEBytes(64, [4]byte{'P', 'E', 0, 0}),
			want: true,
		},
		{
			name: "not MZ",
			data: make([]byte, 64), // all zero, no MZ signature
			want: false,
		},
		{
			name: "MZ but e_lfanew points before the DOS header",
			data: func() []byte {
				b := make([]byte, 64) // full DOS header, so the initial read succeeds
				b[0], b[1] = 'M', 'Z'
				binary.LittleEndian.PutUint32(b[0x3C:0x40], 10) // peOffset=10 < 64
				return b
			}(),
			want: false,
		},
		{
			name: "MZ, valid offset, wrong PE signature",
			data: buildPEBytes(64, [4]byte{'X', 'X', 0, 0}),
			want: false,
		},
		{
			name:    "too short for even a DOS header",
			data:    []byte{'M', 'Z'},
			wantErr: true,
		},
		{
			name:    "MZ with valid offset but truncated before the PE signature",
			data:    buildPEBytes(64, [4]byte{'P', 'E', 0, 0})[:66], // only 2 bytes past peOffset
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsPEFile(bytes.NewReader(tt.data))
			if (err != nil) != tt.wantErr {
				t.Fatalf("IsPEFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("IsPEFile() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("real I/O error", func(t *testing.T) {
		if _, err := IsPEFile(errReader{}); err == nil {
			t.Fatal("expected an error from a reader that always fails")
		}
	})

	t.Run("increments PECount only on a true detection", func(t *testing.T) {
		before := atomic.LoadInt64(&PECount)
		if _, err := IsPEFile(bytes.NewReader(buildPEBytes(64, [4]byte{'P', 'E', 0, 0}))); err != nil {
			t.Fatalf("IsPEFile: %v", err)
		}
		if after := atomic.LoadInt64(&PECount); after != before+1 {
			t.Fatalf("expected PECount to increment by 1, went from %d to %d", before, after)
		}
	})
}

func TestIsELFFile(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    bool
		wantErr bool
	}{
		{
			name: "valid ELF magic",
			data: []byte{0x7F, 'E', 'L', 'F', 1, 2, 3}, // trailing bytes are irrelevant
			want: true,
		},
		{
			name: "wrong magic",
			data: []byte{0x00, 0x00, 0x00, 0x00},
			want: false,
		},
		{
			name:    "too short",
			data:    []byte{0x7F, 'E'},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsELFFile(bytes.NewReader(tt.data))
			if (err != nil) != tt.wantErr {
				t.Fatalf("IsELFFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("IsELFFile() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("real I/O error", func(t *testing.T) {
		if _, err := IsELFFile(errReader{}); err == nil {
			t.Fatal("expected an error from a reader that always fails")
		}
	})

	t.Run("increments ELFCount only on a true detection", func(t *testing.T) {
		before := atomic.LoadInt64(&ELFCount)
		if _, err := IsELFFile(bytes.NewReader([]byte{0x7F, 'E', 'L', 'F'})); err != nil {
			t.Fatalf("IsELFFile: %v", err)
		}
		if after := atomic.LoadInt64(&ELFCount); after != before+1 {
			t.Fatalf("expected ELFCount to increment by 1, went from %d to %d", before, after)
		}
	})
}

func TestMatchesDeclaredType(t *testing.T) {
	oleHeader := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

	tests := []struct {
		name    string
		ext     string
		data    []byte
		want    bool
		wantErr bool
	}{
		{name: "pdf matches", ext: ".pdf", data: []byte("%PDF-1.4\n...rest of a pdf..."), want: true},
		{name: "pdf mismatch", ext: ".pdf", data: []byte("just some plain text"), want: false},

		{name: "png matches", ext: ".png", data: []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0}, want: true},
		{name: "png mismatch", ext: ".png", data: []byte("not a png"), want: false},

		{name: "jpg matches", ext: ".jpg", data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0}, want: true},
		{name: "jpeg extension matches same magic", ext: ".jpeg", data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0}, want: true},

		{name: "docx matches (zip signature)", ext: ".docx", data: []byte("PK\x03\x04rest of the zip"), want: true},
		{name: "xlsx matches (zip signature)", ext: ".xlsx", data: []byte("PK\x03\x04rest of the zip"), want: true},
		{name: "docx mismatch", ext: ".docx", data: []byte("not a zip"), want: false},

		{name: "doc matches OLE header", ext: ".doc", data: oleHeader, want: true},
		{name: "xls matches OLE header", ext: ".xls", data: oleHeader, want: true},
		{name: "ppt matches OLE header", ext: ".ppt", data: oleHeader, want: true},
		{name: "doc mismatch", ext: ".doc", data: []byte{0, 1, 2, 3, 4, 5, 6, 7}, want: false},
		{name: "doc shorter than the OLE header", ext: ".doc", data: []byte{0xD0, 0xCF}, want: false},

		{name: "txt matches plain text", ext: ".txt", data: []byte("just some ordinary plain text"), want: true},
		{name: "csv matches plain text", ext: ".csv", data: []byte("a,b,c\n1,2,3"), want: true},
		{name: "txt mismatch (html payload)", ext: ".txt", data: []byte("<html><body>hi</body></html>"), want: false},

		{name: "unrecognized extension always false", ext: ".exe", data: []byte("MZ anything"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchesDeclaredType(tt.ext, bytes.NewReader(tt.data))
			if (err != nil) != tt.wantErr {
				t.Fatalf("MatchesDeclaredType() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("MatchesDeclaredType(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}

	t.Run("real I/O error", func(t *testing.T) {
		if _, err := MatchesDeclaredType(".pdf", errReader{}); err == nil {
			t.Fatal("expected an error from a reader that always fails")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		// http.DetectContentType falls back to text/plain for empty input,
		// so an empty file matches a text extension but not a binary one.
		if got, err := MatchesDeclaredType(".txt", bytes.NewReader(nil)); err != nil || !got {
			t.Fatalf("MatchesDeclaredType(.txt, empty) = %v, %v, want true, nil", got, err)
		}
		if got, err := MatchesDeclaredType(".pdf", bytes.NewReader(nil)); err != nil || got {
			t.Fatalf("MatchesDeclaredType(.pdf, empty) = %v, %v, want false, nil", got, err)
		}
	})
}

// io.ReadSeeker interface is satisfied by *bytes.Reader; this is a compile-time
// reminder that the fixtures above must stay in sync with that interface.
var _ io.ReadSeeker = (*bytes.Reader)(nil)

// TestContainsPDFJavaScript needs actual structurally-valid PDFs (a full
// xref table and object graph for pdfcpu to parse), unlike MatchesDeclaredType
// above which only sniffs a header prefix — so this reads real fixture files
// from testing_tooling instead of embedding byte literals inline.
func TestContainsPDFJavaScript(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "PDF with an OpenAction JavaScript trigger",
			path: "testing_tooling/js_test.pdf",
			want: true,
		},
		{
			name: "clean PDF, no actions",
			path: "testing_tooling/test.pdf",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.path)
			if err != nil {
				t.Fatalf("open %s: %v", tt.path, err)
			}
			defer f.Close()

			got, err := ContainsPDFJavaScript(f)
			if err != nil {
				t.Fatalf("ContainsPDFJavaScript(%s) error = %v", tt.path, err)
			}
			if got != tt.want {
				t.Fatalf("ContainsPDFJavaScript(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
