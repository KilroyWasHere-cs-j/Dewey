package main


/*
	A collection of functions used to validate the uploaded files
	Author: Gabriel Tower
	Last update: 4/10/26
*/

import (
	"encoding/binary"
	"io"
)

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

	return true, nil
}

// IsELFFile checks whether the provided file is a Linux ELF binary.
//
// It validates the ELF magic number:
//
//   0x7F 'E' 'L' 'F'
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

	return header == [4]byte{0x7F, 'E', 'L', 'F'}, nil
}

// zeroize overwrites a byte slice with zeros.
//
// Note:
//   - Intended for sensitive in-memory data cleanup (best-effort)
//   - Not guaranteed against compiler optimizations in all cases
func zeroize(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
