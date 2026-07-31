package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultHost = "http://localhost:8080"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dewey-cli <command>")
		fmt.Fprintln(os.Stderr, "commands: health, version")
		os.Exit(1)
	}

	host := os.Getenv("DEWEY_HOST")
	if host == "" {
		host = defaultHost
	}

	switch os.Args[1] {
	case "health":
		get(host + "/")
	case "version":
		get(host + "/version")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func get(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reading response failed:", err)
		os.Exit(1)
	}

	fmt.Println(string(body))
}
