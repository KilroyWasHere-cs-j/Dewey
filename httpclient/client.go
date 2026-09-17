// Package httpclient holds the request-building logic cli and dewey-mcp
// both need to talk to the Dewey backend — previously implemented twice,
// nearly identically, in cli/requests.go and dewey-mcp/api_tools.go
// (issue #433). This package covers only the HTTP mechanics (building the
// request, applying a timeout, reading the body, turning a >=400 status
// into an error); it deliberately has no opinion on what a caller does
// with the result — cli prints colored output and exits non-zero on
// failure, dewey-mcp returns (value, error) pairs, and both stay that way
// through a thin wrapper in their own package rather than this one
// growing two personalities.
package httpclient

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

// RequestTimeout bounds a quick API call (health, version, get/del/postJSON).
// UploadTimeout is longer since upload sends a whole file in one request —
// a timeout tuned for quick calls would false-positive-fail on a large or
// slow upload (issue #424).
const (
	RequestTimeout = 10 * time.Second
	UploadTimeout  = 120 * time.Second
)

// UploadMetaFields lists the multipart form fields uploadFile
// (backend/routes.go) reads into MetaData, in the same order the backend
// struct declares them — callers validate field=value args against this so
// a typo'd key fails fast instead of being silently dropped.
var UploadMetaFields = []string{
	"claim_number", "claimant_name", "date_of_injury", "employer", "adjuster",
	"support", "claim_type", "jurisdiction", "policy_number", "acts_id",
}

// Client makes requests against the Dewey backend. The zero value is ready
// to use and sends no auth header — set Password for routes that require
// one (cli threads FILES_PASSWORD/MACHINES_PASSWORD through; dewey-mcp
// currently only calls unauthenticated routes, so it never sets this).
type Client struct {
	Password string
}

// setAuthHeader attaches the X-Dewey-Password header when a password was
// configured (issue #332) — requests against unauthenticated routes
// (health, version) use a zero-value Client and skip this entirely.
func (c *Client) setAuthHeader(req *http.Request) {
	if c.Password != "" {
		req.Header.Set("X-Dewey-Password", c.Password)
	}
}

// StatusError reports that a request completed but the backend returned a
// >=400 status — distinct from a transport/read failure so callers can
// tell "got a response, it just wasn't a success" apart from "never got a
// response at all" (cli's wrapper treats these differently: it prints and
// continues for a StatusError, but exits non-zero for anything else).
// Body holds the raw response body, since some callers display the
// backend's error JSON even on failure.
type StatusError struct {
	Code int
	Body []byte
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("status code: %d", e.Code)
}

// doRequest fires req with timeout and returns the response body. A
// transport or body-read failure comes back as a plain error; a >=400
// response comes back as *StatusError so callers can distinguish the two.
func (c *Client) doRequest(req *http.Request, timeout time.Duration) ([]byte, error) {
	client := http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return body, &StatusError{Code: resp.StatusCode, Body: body}
	}
	return body, nil
}

// Get issues a GET request against url.
func (c *Client) Get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	return c.doRequest(req, RequestTimeout)
}

// Del issues a DELETE request against url.
func (c *Client) Del(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	return c.doRequest(req, RequestTimeout)
}

// PostJSON issues a POST request against url with payload marshaled as the
// JSON body.
func (c *Client) PostJSON(url string, payload any) ([]byte, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	return c.doRequest(req, RequestTimeout)
}

// Upload sends filePath to url as multipart/form-data under the "file"
// field, matching what uploadFile (backend/routes.go) reads. meta entries
// are attached as additional form fields (claim_number, etc.) — omitted
// entries are simply absent from the request, same as leaving a form field
// blank in the admin portal's upload dialog.
func (c *Client) Upload(url, filePath string, meta map[string]string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, key := range UploadMetaFields {
		if value, ok := meta[key]; ok {
			if err := w.WriteField(key, value); err != nil {
				return nil, err
			}
		}
	}

	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	c.setAuthHeader(req)
	return c.doRequest(req, UploadTimeout)
}

// SelfIP reports the local address the OS would use to reach host. A UDP
// "connect" never sends a packet - it just asks the OS to resolve the
// route - so this is the closest client-side guess at "what IP am I
// calling from". It can still differ from what the server actually sees
// (e.g. Podman's rootless port-forwarding NAT rewrites the source address
// in transit), so treat this as a starting point when deciding what to
// register in the backend's known_machines allowlist, not a guarantee.
//
// Unlike Get/Del/PostJSON/Upload, this doesn't touch the backend at all —
// it's a standalone function rather than a Client method since it needs
// no auth/timeout configuration.
func SelfIP(host string) (string, error) {
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
