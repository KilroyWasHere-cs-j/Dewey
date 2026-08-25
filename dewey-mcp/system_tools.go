package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CreateFile writes contents to the file at path, creating it if it doesn't
// exist or truncating it if it does. path is resolved via resolveSafePath,
// so it can't escape the server's working directory.
func CreateFile(contents string, path string) error {
	safePath, err := resolveSafePath(path)
	if err != nil {
		return err
	}

	file, err := os.Create(safePath)
	if err != nil {
		return err
	}

	_, err = file.WriteString(contents)
	if err != nil {
		return err
	}
	return nil
}

// resolveSafePath joins rel onto the server's working directory and rejects
// anything that resolves outside it (mirrors resolveStorePath in
// backend/filemanager.go). Without this, a caller-supplied absolute path or
// "../" escape in read_file/move_file/delete_file would reach any file on
// the host the server process can access, not just files under its own
// working directory.
func resolveSafePath(rel string) (string, error) {
	base, err := os.Getwd()
	if err != nil {
		return "", err
	}

	full := filepath.Join(base, rel)
	baseClean := filepath.Clean(base)

	if full != baseClean && !strings.HasPrefix(full, baseClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes working directory %q", rel, base)
	}

	return full, nil
}

// ReadFile returns the full contents of the file at path as a string.
func ReadFile(path string) (error, string) {
	safePath, err := resolveSafePath(path)
	if err != nil {
		return err, ""
	}
	data, err := os.ReadFile(safePath)
	if err != nil {
		return err, ""
	}
	return nil, string(data)
}

// MoveFile renames src to dst. If dst already exists, the move is skipped
// (src is left in place) rather than overwriting it.
func MoveFile(src, dst string) error {
	safeSrc, err := resolveSafePath(src)
	if err != nil {
		return err
	}
	safeDst, err := resolveSafePath(dst)
	if err != nil {
		return err
	}
	if _, err := os.Stat(safeSrc); err != nil {
		return err
	}
	if _, err := os.Stat(safeDst); err == nil {
		return nil
	}
	return os.Rename(safeSrc, safeDst)
}

// DeleteFile removes the file at path. Errors if path doesn't exist.
func DeleteFile(path string) error {
	safePath, err := resolveSafePath(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(safePath); err != nil {
		return err
	}
	return os.Remove(safePath)
}
