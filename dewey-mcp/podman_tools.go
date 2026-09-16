package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// validateContainerID rejects a containerID that starts with "-". exec.Command
// already uses a real argv (not a shell string), so classic ";"/"|" shell
// injection isn't possible — but a flag-shaped value like "-i" or "--rm"
// would still reach podman as a bare positional argument and could be
// interpreted as a flag instead of a container name/ID (issue #411).
func validateContainerID(id string) error {
	if strings.HasPrefix(id, "-") {
		return fmt.Errorf("invalid container id: %q looks like a flag", id)
	}
	return nil
}

func GetPodmanHealth() (string, error) {
	cmd := exec.Command("podman", "pod", "ps", "--filter", "name=dewey-pod")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func GetPodmanContainers() (string, error) {
	cmd := exec.Command("podman", "ps")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func GetPodmanContainerLogs(containerID string) (string, error) {
	if err := validateContainerID(containerID); err != nil {
		return "", err
	}
	cmd := exec.Command("podman", "logs", containerID)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func RestartPodmanContainer(containerID string) (string, error) {
	if err := validateContainerID(containerID); err != nil {
		return "", err
	}
	cmd := exec.Command("podman", "restart", containerID)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
