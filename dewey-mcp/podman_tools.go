package main

import (
	"fmt"
	"os/exec"
	"bytes"
)

func GetPodmanHealth() (error, string) {
	cmd := exec.Command("podman", "pod", "ps", "--filter", "name=dewey-pod")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String()), ""
	}
	return nil, stdout.String()
}

func GetPodmanContainers() (error, string) {
	cmd := exec.Command("podman", "ps")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String()), ""
	}
	return nil, stdout.String()
}

func GetPodmanContainerLogs(containerID string) (error, string) {
	cmd := exec.Command("podman", "logs", containerID)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String()), ""
	}
	return nil, stdout.String()
}

func RestartPodmanContainer(containerID string) (error, string) {
	cmd := exec.Command("podman", "restart", containerID)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String()), ""
	}
	return nil, stdout.String()	
}
