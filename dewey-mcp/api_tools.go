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

func IsUp() (error, string) {
	host := os.Getenv("DEWEY_HOST")
	if host == "" {
		host = defaultHost
	}
	err, body := get(host + "/")
	if err != nil {
		return err, ""
	}
	return nil, body
}

func Version() (error, string) {
	host := os.Getenv("DEWEY_HOST")
	if host == "" {
		host = defaultHost
	}
	err, body := get(host + "/version")
	if err != nil {
		return err, ""
	}
	return nil, body
}

// doRequest fires req and prints the response body. On success (status <
// 400) it's handed to formatter for display, falling back to indented JSON
// if formatter is nil; error bodies always print as indented JSON in red.
// It's shared by every request-shaped command so output stays consistent
// across GET/POST/DELETE.
func doRequest(req *http.Request) (error, string) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err, ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err, ""
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("status code: %d", resp.StatusCode), ""
	}
	return nil, string(body)
}

func get(url string) (error, string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err, ""
	}
	err, body := doRequest(req)
	return err, body
}

func del(url string) (error, string) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err, ""
	}
	err, body := doRequest(req)
	return err, body
}

func postJSON(url string, payload any) (error, string) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return err, ""
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err, ""
	}
	req.Header.Set("Content-Type", "application/json")
	err, body := doRequest(req)
	return err, body
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
	err, _ = doRequest(req)
	return err
}

// selfIP reports the local address the OS would use to reach host. A UDP
// "connect" never sends a packet - it just asks the OS to resolve the route -
// so this is the closest client-side guess at "what IP am I calling from".
// It can still differ from what the server actually sees (e.g. Podman's
// rootless port-forwarding NAT rewrites the source address in transit), so
// treat this as a starting point when deciding what to register in the
// backend's known_machines allowlist, not a guarantee.
func selfIP(host string) (error, string) {
	u, err := url.Parse(host)
	if err != nil {
		return err, ""
	}

	target := u.Host
	if !strings.Contains(target, ":") {
		target += ":80"
	}

	conn, err := net.Dial("udp", target)
	if err != nil {
		return err, ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return nil, localAddr.IP.String()
}
