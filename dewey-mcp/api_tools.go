package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// uploadMetaFields lists the multipart form fields uploadFile
// (backend/routes.go) reads into MetaData, in the same order the backend
// struct declares them — kept here so the CLI can validate field=value args
// instead of silently dropping a typo'd key.
var uploadMetaFields = []string{
	"claim_number", "claimant_name", "date_of_injury", "employer", "adjuster",
	"support", "claim_type", "jurisdiction", "policy_number", "acts_id",
}

const defaultHost = "http://localhost:8080"

// requestTimeout bounds a quick API call (health, version, get/del/postJSON).
// uploadTimeout is longer since upload sends a whole file in one request —
// a timeout tuned for quick calls would false-positive-fail on a large or
// slow upload (issue #424).
const requestTimeout = 10 // In seconds
const uploadTimeout = 120 // In seconds

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

// doRequest fires req and prints the response body. On success (status <
// 400) it's handed to formatter for display, falling back to indented JSON
// if formatter is nil; error bodies always print as indented JSON in red.
// It's shared by every request-shaped command so output stays consistent
// across GET/POST/DELETE.
//
// timeout is per-call rather than a fixed value on a shared client because
// upload needs much more headroom than a quick API call.
func doRequest(req *http.Request, timeout time.Duration) (string, error) {
	client := http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status code: %d", resp.StatusCode)
	}
	return string(body), nil
}

func get(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	body, err := doRequest(req, time.Duration(requestTimeout)*time.Second)
	return body, err
}

func del(url string) (string, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return "", err
	}
	body, err := doRequest(req, time.Duration(requestTimeout)*time.Second)
	return body, err
}

func postJSON(url string, payload any) (string, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	body, err := doRequest(req, time.Duration(requestTimeout)*time.Second)
	return body, err
}

// upload sends filePath to the backend's /upload route as multipart/form-data
// under the "file" field, matching what uploadFile (backend/routes.go) reads.
// meta entries are attached as additional form fields (claim_number, etc.) —
// omitted entries are simply absent from the request, same as leaving a form
// field blank in the admin portal's upload dialog.
func upload(url, filePath string, meta map[string]string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, key := range uploadMetaFields {
		if value, ok := meta[key]; ok {
			if err := w.WriteField(key, value); err != nil {
				return err
			}
		}
	}

	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, err = doRequest(req, time.Duration(uploadTimeout)*time.Second)
	return err
}

// selfIP reports the local address the OS would use to reach host. A UDP
// "connect" never sends a packet - it just asks the OS to resolve the route -
// so this is the closest client-side guess at "what IP am I calling from".
// It can still differ from what the server actually sees (e.g. Podman's
// rootless port-forwarding NAT rewrites the source address in transit), so
// treat this as a starting point when deciding what to register in the
// backend's known_machines allowlist, not a guarantee.
func selfIP(host string) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", err
	}

	target := u.Host
	if !strings.Contains(target, ":") {
		target += ":80"
	}

	conn, err := net.Dial("udp", target)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
