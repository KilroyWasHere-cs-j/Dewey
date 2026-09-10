package main

import (
	"fmt"
	"os/exec"
	"os"
)

func reset_db() {
	ok := YesNoPrompt("You are about to reset the database. This is high risk action. Are you sure you want to reset the database? There is no rollback...", false)
	if ok {
		cmd := exec.Command("bash", "-c", "podman volume inspect mysql-data &>/dev/null && podman volume rm mysql-data")
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"reset_db failed:"+colorReset, err)
		}

	} else {
		fmt.Println("Cool cool cool, not resetting.")
	}
}

func clean_slate() {
	ok := YesNoPrompt("Your to nuke all stored data. This is high risk action. Are you sure you want to reset the database? There is no rollback...", false)
	if ok {
		cmd := exec.Command("bash", "-c", `for vol in dewey-store dewey-cache dewey-backup dewey-logs dewey-plugin-scratch; do
  podman volume inspect "$vol" &>/dev/null && podman volume rm "$vol"
done`)
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"reset_db failed:"+colorReset, err)
		}

	} else {
		fmt.Println("Cool cool cool, not resetting.")
	}
}
