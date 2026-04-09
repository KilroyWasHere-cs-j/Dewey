package main

import (
	"encoding/binary"
	// "fmt"
	// "os"
	"io"
)

func IsPEFile(f io.ReadSeeker) (bool, error) {
	mz_found := false
	pe_found := false
	// Read first 64 bytes (DOS header is at least this long)
	header := make([]byte, 64)
	if _, err := f.Read(header); err != nil {
		zeroize(header)
		return false, err
	}

	// Check for "MZ" signature
	if header[0] == 'M' && header[1] == 'Z' {
		mz_found = true
	}

	// e_lfanew is at offset 0x3C (60), gives PE header offset
	peOffset := binary.LittleEndian.Uint32(header[0x3C:0x40])

	// Seek to PE header
	if _, err := f.Seek(int64(peOffset), io.SeekStart); err != nil {
		zeroize(header)
		return false, err
	}

	// Read PE signature (4 bytes)
	peSig := make([]byte, 4)
	if _, err := f.Read(peSig); err != nil {
		zeroize(header)
		return false, err
	}

	// Check for "PE\0\0"
	if peSig[0] == 'P' && peSig[1] == 'E' && peSig[2] == 0 && peSig[3] == 0 {
		pe_found = true
	}
	zeroize(header)
	if mz_found && pe_found {
		return true, nil
	}
	return false, nil
}

/*
 Used as a clean up function to zeroize the bytes in an array
*/
func zeroize(b []byte) {
    for i := range b {
        b[i] = 0
    }
}

