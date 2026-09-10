package main

import (
	"fmt"
	"os/exec"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

func pullFile(podfilepath string, hostfilepath string) {
	// podfilepath is "<container>:<path>" (podman cp's source syntax) — split
	// off the container name before validating the path itself.
	container, path, ok := strings.Cut(podfilepath, ":")
	if !ok {
		fmt.Printf("error: expected <container>:<path>, got %q\n", podfilepath)
		return
	}

	// path is already absolute inside the container (e.g. "/app/store/x").
	// Clean collapses any "../" in it, then the prefix check confirms the
	// cleaned result didn't walk outside /app/store — same guard as the
	// backend's resolveStorePath (filemanager.go).
	full := filepath.Clean(path)
	baseClean := filepath.Clean("/app/store")

	if full != baseClean && !strings.HasPrefix(full, baseClean+string(os.PathSeparator)) {
		fmt.Printf("error: path %q escapes base directory %q\n", path, "/app/store")
		return
	}

	// hostfilepath isn't confined to a fixed directory (any writable path is
	// allowed), but reject an existing symlink at that exact spot — without
	// this, someone could pre-plant a symlink pointing elsewhere and have
	// podman cp silently write through it to an unintended location.
	if info, err := os.Lstat(hostfilepath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		fmt.Printf("error: hostfilepath %q is a symlink, refusing to write through it\n", hostfilepath)
		return
	}

	cmd := exec.Command("podman", "cp", container+":"+full, hostfilepath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	fmt.Println(stdout.String())
}

func listFilesOnDisk() {
	cmd := exec.Command("podman", "exec", "cross-doc-tool-dev", "ls", "-la", "/app/store")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	fmt.Println(stdout.String())
}

func viewLogList(host string) {
	get(host + "/fileview/viewLogDir", "", nil)
}

func viewLog(logfile string, host string) {
	get(host+"/fileview/viewFile/log/"+logfile, "", nil)
}
