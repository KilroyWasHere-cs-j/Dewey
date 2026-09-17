package httpclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestClientGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	var c Client
	body, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("Get() body = %q, want %q", body, "ok")
	}
}

func TestClientGetStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	var c Client
	body, err := c.Get(srv.URL)

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("Get() error = %T (%v), want *StatusError", err, err)
	}
	if statusErr.Code != http.StatusNotFound {
		t.Fatalf("StatusError.Code = %d, want %d", statusErr.Code, http.StatusNotFound)
	}
	if string(statusErr.Body) != `{"error":"not found"}` {
		t.Fatalf("StatusError.Body = %q, want the response body", statusErr.Body)
	}
	if string(body) != string(statusErr.Body) {
		t.Fatalf("Get() body = %q, want it to match the error's body too", body)
	}
}

func TestClientSetsPasswordHeaderOnlyWhenConfigured(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Dewey-Password")
	}))
	defer srv.Close()

	c := Client{Password: "secret"}
	if _, err := c.Get(srv.URL); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if gotHeader != "secret" {
		t.Fatalf("X-Dewey-Password header = %q, want %q", gotHeader, "secret")
	}

	var noAuth Client
	if _, err := noAuth.Get(srv.URL); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if gotHeader != "" {
		t.Fatalf("X-Dewey-Password header = %q, want empty when Password unset", gotHeader)
	}
}

func TestClientPostJSONSendsPayload(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		gotBody = string(buf)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
	}))
	defer srv.Close()

	var c Client
	if _, err := c.PostJSON(srv.URL, map[string]string{"ip": "1.2.3.4"}); err != nil {
		t.Fatalf("PostJSON() unexpected error: %v", err)
	}
	if gotBody != `{"ip":"1.2.3.4"}` {
		t.Fatalf("request body = %q, want %q", gotBody, `{"ip":"1.2.3.4"}`)
	}
}

func TestClientUploadSendsFileAndMeta(t *testing.T) {
	var gotFilename string
	var gotClaimNumber string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("server: ParseMultipartForm: %v", err)
		}
		gotClaimNumber = r.FormValue("claim_number")
		if fh := r.MultipartForm.File["file"]; len(fh) == 1 {
			gotFilename = fh[0].Filename
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	src := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(src, []byte("pdf content"), 0600); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	var c Client
	_, err := c.Upload(srv.URL, src, map[string]string{"claim_number": "CL-1", "unknown_field": "ignored"})
	if err != nil {
		t.Fatalf("Upload() unexpected error: %v", err)
	}
	if gotFilename != "report.pdf" {
		t.Fatalf("uploaded filename = %q, want %q", gotFilename, "report.pdf")
	}
	if gotClaimNumber != "CL-1" {
		t.Fatalf("claim_number field = %q, want %q", gotClaimNumber, "CL-1")
	}
}

func TestSelfIPReturnsAnAddress(t *testing.T) {
	// No real network I/O happens — see SelfIP's doc comment — so this is
	// safe to run in any environment, no live host required.
	ip, err := SelfIP("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("SelfIP() unexpected error: %v", err)
	}
	if ip == "" {
		t.Fatal("SelfIP() = \"\", want a non-empty address")
	}
}
