package main

/*
	A collection of functions used to validate the uploaded files
	Author: Gabriel Tower
	Last update: 4/10/26
*/

import (
	"encoding/binary"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// Atomic counters for PE and ELF detections — read by Prometheus
var PECount int64
var ELFCount int64

// pdfcpu otherwise tries to create/read a user config dir (~/.config) on
// first use, e.g. via api.ReadContext's underlying config lookup — the
// backend container runs --read-only (issue #213), so that mkdir fails
// and every PDF upload errors out instead of getting scanned. We don't
// need custom fonts/config for structural validation, so disabling it
// entirely is the documented fix (pkg/api/api.go's own comment on
// DisableConfigDir points here for exactly this case).
func init() {
	api.DisableConfigDir()
}

// oleMagic is the Compound File Binary Format signature used by legacy
// Microsoft Office formats (.doc, .xls, .ppt) prior to the OOXML/zip switch.
// http.DetectContentType has no signature for this format, so it's checked
// directly.
var oleMagic = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// IsPEFile checks whether the provided file is a Windows Portable Executable (PE).
//
// It validates:
//   - DOS header signature ("MZ")
//   - PE header signature ("PE\0\0")
//
// Args:
//   - f: seekable file reader (io.ReadSeeker)
//
// Returns:
//   - bool: true if file is a valid PE binary
//   - error: I/O or parsing error
func IsPEFile(f io.ReadSeeker) (bool, error) {
	// Reset file pointer
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	// Read DOS header
	header := make([]byte, 64)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, err
	}

	// Check "MZ" signature
	if header[0] != 'M' || header[1] != 'Z' {
		return false, nil
	}

	// PE header offset (e_lfanew at 0x3C)
	peOffset := binary.LittleEndian.Uint32(header[0x3C:0x40])

	// Basic sanity check (prevents crazy offsets)
	if peOffset < 64 {
		return false, nil
	}

	// Seek to PE header
	if _, err := f.Seek(int64(peOffset), io.SeekStart); err != nil {
		return false, err
	}

	// Read PE signature
	var peSig [4]byte
	if _, err := io.ReadFull(f, peSig[:]); err != nil {
		return false, err
	}

	// Validate "PE\0\0"
	if peSig != [4]byte{'P', 'E', 0, 0} {
		return false, nil
	}
	atomic.AddInt64(&PECount, 1)
	return true, nil
}

// IsELFFile checks whether the provided file is a Linux ELF binary.
//
// It validates the ELF magic number:
//
//	0x7F 'E' 'L' 'F'
//
// Args:
//   - f: seekable file reader (io.ReadSeeker)
//
// Returns:
//   - bool: true if file is an ELF binary
//   - error: I/O or read error
func IsELFFile(f io.ReadSeeker) (bool, error) {
	// Reset reader
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	// ELF magic number is 4 bytes, no need for 8
	var header [4]byte

	if _, err := io.ReadFull(f, header[:]); err != nil {
		return false, err
	}

	magic := [4]byte{0x7F, 'E', 'L', 'F'}
	if header == magic {
		atomic.AddInt64(&ELFCount, 1)
		return true, nil
	}
	return false, nil
}

// ContainsPDFJavaScript parses f's structure via pdfcpu and checks the
// document catalog for /OpenAction, /AA (additional-actions), or a
// populated /JavaScript name tree. Presence of any of these is grounds
// for rejection regardless of the action type — issue #245 settled on
// reject-any-auto-action/JS rather than trying to classify intent.
//
// Note: this only checks catalog-level triggers. Per-page or
// per-annotation /AA (e.g. a form widget's action) would need walking
// the page tree's /Annots separately — not covered here yet.
//
// Args:
//   - f: seekable file reader (io.ReadSeeker), already confirmed to be a
//     PDF via MatchesDeclaredType
//
// Returns:
//   - bool: true if any JS/auto-action trigger was found
//   - error: parse error (malformed PDF, etc.)
func ContainsPDFJavaScript(f io.ReadSeeker) (bool, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	ctx, err := api.ReadContext(f, nil)
	if err != nil {
		return false, err
	}

	catalog, err := ctx.Catalog()
	if err != nil {
		return false, err
	}

	if catalog.HasEntry("OpenAction") || catalog.HasEntry("AA") {
		return true, nil
	}

	if err := ctx.LocateNameTree("JavaScript", false); err != nil {
		return false, err
	}
	if ctx.Names["JavaScript"] != nil {
		return true, nil
	}

	return false, nil
}

// MatchesDeclaredType sniffs the actual content of f and checks it against
// what the given extension claims to be. Extension + PE/ELF checks alone
// still let a script or HTML payload through under an allowed extension
// (e.g. "notes.txt") — if that file is ever served back, a browser that
// content-sniffs it as HTML would execute it. This closes that gap.
//
// Args:
//   - ext: lowercase extension including the leading dot (e.g. ".pdf")
//   - f: seekable file reader (io.ReadSeeker)
//
// Returns:
//   - bool: true if the sniffed content is consistent with ext
//   - error: I/O error while reading
func MatchesDeclaredType(ext string, f io.ReadSeeker) (bool, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	// Legacy binary Office formats (.doc, .xls, .ppt) share the OLE header —
	// handled separately since http.DetectContentType can't identify them.
	switch ext {
	case ".doc", ".xls", ".ppt":
		var header [8]byte
		n, err := io.ReadFull(f, header[:])
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return false, err
		}
		return n == len(header) && header == oleMagic, nil
	}

	// http.DetectContentType only looks at (up to) the first 512 bytes.
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return false, err
	}
	sniffed := http.DetectContentType(buf[:n])

	switch ext {
	case ".pdf":
		return sniffed == "application/pdf", nil
	case ".png":
		return sniffed == "image/png", nil
	case ".jpg", ".jpeg":
		return sniffed == "image/jpeg", nil
	case ".docx", ".xlsx":
		// OOXML formats are zip archives; the sniffer doesn't unpack them
		// far enough to see the inner content-type manifest.
		return sniffed == "application/zip", nil
	case ".txt", ".csv":
		// Anything that sniffs as something other than plain text here
		// (e.g. HTML) means the content doesn't match a text declaration —
		// most likely a payload relying on browser content-sniffing to
		// render as HTML.
		return strings.HasPrefix(sniffed, "text/plain"), nil
	}

	return false, nil
}
