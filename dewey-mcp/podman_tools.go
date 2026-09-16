package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

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
	cmd := exec.Command("podman", "restart", containerID)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
