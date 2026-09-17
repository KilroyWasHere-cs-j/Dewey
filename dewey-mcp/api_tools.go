package main

import (
	"os"

	"dewey-httpclient"
)

const defaultHost = "http://localhost:8080"

func IsUp() (string, error) {
	host := os.Getenv("DEWEY_HOST")
	if host == "" {
		host = defaultHost
	}
	body, err := get(host + "/")
	if err != nil {
		return "", err
	}
	return body, nil
}

func Version() (string, error) {
	host := os.Getenv("DEWEY_HOST")
	if host == "" {
		host = defaultHost
	}
	body, err := get(host + "/version")
	if err != nil {
		return "", err
	}
	return body, nil
}

// get/del/postJSON/upload are thin wrappers over httpclient.Client — this
// package never sets a password (nothing it currently calls requires
// auth), unlike cli's equivalents which thread FILES_PASSWORD/
// MACHINES_PASSWORD through (issue #433).

func get(url string) (string, error) {
	var c httpclient.Client
	body, err := c.Get(url)
	return string(body), err
}

func del(url string) (string, error) {
	var c httpclient.Client
	body, err := c.Del(url)
	return string(body), err
}

func postJSON(url string, payload any) (string, error) {
	var c httpclient.Client
	body, err := c.PostJSON(url, payload)
	return string(body), err
}

// upload sends filePath to the backend's /upload route as multipart/form-data
// under the "file" field, matching what uploadFile (backend/routes.go) reads,
// delegating the actual request-building to httpclient.Client.Upload.
func upload(url, filePath string, meta map[string]string) error {
	var c httpclient.Client
	_, err := c.Upload(url, filePath, meta)
	return err
}

// selfIP reports the local address the OS would use to reach host, via
// httpclient.SelfIP — see that function's doc comment for what this
// actually measures and its limits.
func selfIP(host string) (string, error) {
	return httpclient.SelfIP(host)
}
