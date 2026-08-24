package main

import (
	"os/exec"
)

func GetPodmanHealth() (error, string) {
	out, err := exec.Command("podman", "pod", "ps", "--filter", "name=dewey-pod").Output()
	if err != nil {
		return err, ""
	}
	return nil, string(out)
}

func GetPodmanContainers() (error, string) {
	out, err := exec.Command("podman", "ps").Output()
	if err != nil {
		return err, ""
	}
	return nil, string(out)
}

func GetPodmanContainerLogs(containerID string) (error, string) {
	out, err := exec.Command("podman", "logs", containerID).Output()
	if err != nil {
		return err, ""
	}
	return nil, string(out)
}

func RestartPodmanContainer(containerID string) (error, string) {
	out, err := exec.Command("podman", "restart", containerID).Output()
	if err != nil {
		return err, ""
	}
	return nil, string(out)
}
