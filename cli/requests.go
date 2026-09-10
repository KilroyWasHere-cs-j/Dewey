package main

import (
	"fmt"
	"bytes"
	"os"
	"io"
	"net/http"
	"encoding/json"
	"mime/multipart"
	"path/filepath"
)

// doRequest fires req and prints the response body. On success (status <
// 400) it's handed to formatter for display, falling back to indented JSON
// if formatter is nil; error bodies always print as indented JSON in red.
// It's shared by every request-shaped command so output stays consistent
// across GET/POST/DELETE.
func doRequest(req *http.Request, formatter func([]byte) string) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"request failed:"+colorReset, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"reading response failed:"+colorReset, err)
		os.Exit(1)
	}

	if resp.StatusCode >= 400 {
		fmt.Println(colorRed + prettyJSON(body) + colorReset)
		return
	}

	if formatter != nil {
		fmt.Println(formatter(body))
		return
	}
	fmt.Println(colorGreen + prettyJSON(body) + colorReset)
}

// setAuthHeader attaches the X-Dewey-Password header when a password was
// configured (issue #332) — commands against the base group (health,
// version) pass "" and skip it, since those routes aren't gated.
func setAuthHeader(req *http.Request, password string) {
	if password != "" {
		req.Header.Set("X-Dewey-Password", password)
	}
}

func get(url string, password string, formatter func([]byte) string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	setAuthHeader(req, password)
	doRequest(req, formatter)
}

func del(url string, password string, formatter func([]byte) string) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	setAuthHeader(req, password)
	doRequest(req, formatter)
}

func postJSON(url string, password string, payload any, formatter func([]byte) string) {
	buf, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to encode request body:"+colorReset, err)
		os.Exit(1)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthHeader(req, password)
	doRequest(req, formatter)
}

// upload sends filePath to the backend's /upload route as multipart/form-data
// under the "file" field, matching what uploadFile (backend/routes.go) reads.
// meta entries are attached as additional form fields (claim_number, etc.) —
// omitted entries are simply absent from the request, same as leaving a form
// field blank in the admin portal's upload dialog.
func upload(url, password, filePath string, meta map[string]string) {
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"could not open file:"+colorReset, err)
		os.Exit(1)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, key := range uploadMetaFields {
		if value, ok := meta[key]; ok {
			if err := w.WriteField(key, value); err != nil {
				fmt.Fprintln(os.Stderr, colorRed+"failed to build upload:"+colorReset, err)
				os.Exit(1)
			}
		}
	}

	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to build upload:"+colorReset, err)
		os.Exit(1)
	}
	if _, err := io.Copy(part, f); err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to read file:"+colorReset, err)
		os.Exit(1)
	}
	if err := w.Close(); err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to build upload:"+colorReset, err)
		os.Exit(1)
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	setAuthHeader(req, password)
	doRequest(req, nil)
}
