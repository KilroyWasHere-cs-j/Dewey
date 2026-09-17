package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParseMoveFileBodyRejectsMissingFields confirms moveFile's body
// parsing rejects a request missing current_path/new_path (issue #416) —
// these are the two fields the old :param-based route couldn't carry a
// "/" in, so this is the boundary that replaced it.
func TestParseMoveFileBodyRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty body", `{}`},
		{"missing new_path", `{"current_path": "image/somefile.png"}`},
		{"missing current_path", `{"new_path": "text/somefile.png"}`},
		{"malformed JSON", `not json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/core/files/move", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			c, w := newTestContext(req)

			_, _, ok := parseMoveFileBody(c)

			if ok {
				t.Fatalf("parseMoveFileBody() with body %q = ok, want rejected", tt.body)
			}
			if w.Code != http.StatusBadRequest {
				t.Fatalf("parseMoveFileBody() with body %q = status %d, want %d", tt.body, w.Code, http.StatusBadRequest)
			}
		})
	}
}

// TestParseMoveFileBodyAcceptsSubfolderPaths confirms current_path/new_path
// carrying a subfolder ("/") round-trip intact through the JSON body — the
// whole point of issue #416, since gin's old :param-based route truncated
// or 404'd on exactly this.
func TestParseMoveFileBodyAcceptsSubfolderPaths(t *testing.T) {
	body := `{"current_path": "image/somefile.png", "new_path": "text/somefile.png"}`
	req := httptest.NewRequest(http.MethodPost, "/core/files/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c, _ := newTestContext(req)

	currentPath, newPath, ok := parseMoveFileBody(c)

	if !ok {
		t.Fatalf("parseMoveFileBody() with subfolder paths = rejected, want accepted")
	}
	if currentPath != "image/somefile.png" {
		t.Fatalf("currentPath = %q, want %q", currentPath, "image/somefile.png")
	}
	if newPath != "text/somefile.png" {
		t.Fatalf("newPath = %q, want %q", newPath, "text/somefile.png")
	}
}
