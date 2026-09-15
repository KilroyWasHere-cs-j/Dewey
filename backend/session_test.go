package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// resetSessionStore clears the package-level session store so tests don't
// leak tokens into each other — sessionStore is shared process-wide, same
// as sessionStoreMu guarding it.
func resetSessionStore(t *testing.T) {
	t.Helper()
	sessionStoreMu.Lock()
	sessionStore = make(map[string]sessionEntry)
	sessionStoreMu.Unlock()
}

// newTestContext builds a minimal gin.Context + ResponseRecorder wired to
// req, for calling a gin.HandlerFunc directly without spinning up a real
// router/server.
func newTestContext(req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestExchangeForSessionValidPassword(t *testing.T) {
	resetSessionStore(t)
	t.Setenv("FILES_PASSWORD", "correct-horse")

	req := httptest.NewRequest(http.MethodPost, "/core/files/session", nil)
	req.Header.Set("X-Dewey-Password", "correct-horse")
	c, w := newTestContext(req)

	exchangeForSession("files")(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token"`) {
		t.Fatalf("expected a token in the response body, got %s", w.Body.String())
	}
}

func TestExchangeForSessionWrongPassword(t *testing.T) {
	resetSessionStore(t)
	t.Setenv("FILES_PASSWORD", "correct-horse")

	req := httptest.NewRequest(http.MethodPost, "/core/files/session", nil)
	req.Header.Set("X-Dewey-Password", "wrong-guess")
	c, w := newTestContext(req)

	exchangeForSession("files")(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExchangeForSessionMissingPassword(t *testing.T) {
	resetSessionStore(t)
	t.Setenv("FILES_PASSWORD", "correct-horse")

	req := httptest.NewRequest(http.MethodPost, "/core/files/session", nil)
	c, w := newTestContext(req)

	exchangeForSession("files")(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// An unconfigured password env var must fail closed — reject every
// request rather than accept an empty password matching an empty want,
// same reasoning requirePassword always had.
func TestExchangeForSessionUnconfiguredPassword(t *testing.T) {
	resetSessionStore(t)
	t.Setenv("FILES_PASSWORD", "")

	req := httptest.NewRequest(http.MethodPost, "/core/files/session", nil)
	req.Header.Set("X-Dewey-Password", "")
	c, w := newTestContext(req)

	exchangeForSession("files")(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// issueTestToken bypasses the HTTP handler and inserts a token directly,
// for tests that only care about requireSession's own validation logic.
func issueTestToken(t *testing.T, group string, expiresAt time.Time) string {
	t.Helper()
	token, err := randomSuffix(sessionTokenBytes)
	if err != nil {
		t.Fatalf("randomSuffix: %v", err)
	}
	sessionStoreMu.Lock()
	sessionStore[token] = sessionEntry{group: group, expiresAt: expiresAt}
	sessionStoreMu.Unlock()
	return token
}

func TestRequireSessionValidToken(t *testing.T) {
	resetSessionStore(t)
	token := issueTestToken(t, "files", time.Now().Add(sessionTTL))

	req := httptest.NewRequest(http.MethodGet, "/core/files", nil)
	req.Header.Set("X-Dewey-Session-Token", token)
	c, w := newTestContext(req)

	requireSession("files")(c)

	if c.IsAborted() {
		t.Fatalf("expected the chain to continue, but it was aborted (status %d)", w.Code)
	}
}

func TestRequireSessionMissingToken(t *testing.T) {
	resetSessionStore(t)

	req := httptest.NewRequest(http.MethodGet, "/core/files", nil)
	c, w := newTestContext(req)

	requireSession("files")(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !c.IsAborted() {
		t.Fatal("expected the chain to be aborted")
	}
}

func TestRequireSessionUnknownToken(t *testing.T) {
	resetSessionStore(t)

	req := httptest.NewRequest(http.MethodGet, "/core/files", nil)
	req.Header.Set("X-Dewey-Session-Token", "not-a-real-token")
	c, w := newTestContext(req)

	requireSession("files")(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// A token issued for one group must not authorize the other — files and
// machines are deliberately separate credentials (issue #332), and a
// token is just as scoped as the password it replaced.
func TestRequireSessionWrongGroup(t *testing.T) {
	resetSessionStore(t)
	token := issueTestToken(t, "files", time.Now().Add(sessionTTL))

	req := httptest.NewRequest(http.MethodGet, "/core/machines", nil)
	req.Header.Set("X-Dewey-Session-Token", token)
	c, w := newTestContext(req)

	requireSession("machines")(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireSessionExpiredToken(t *testing.T) {
	resetSessionStore(t)
	token := issueTestToken(t, "files", time.Now().Add(-time.Second))

	req := httptest.NewRequest(http.MethodGet, "/core/files", nil)
	req.Header.Set("X-Dewey-Session-Token", token)
	c, w := newTestContext(req)

	requireSession("files")(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	sessionStoreMu.Lock()
	_, stillPresent := sessionStore[token]
	sessionStoreMu.Unlock()
	if stillPresent {
		t.Fatal("expected the expired entry to be removed from the store")
	}
}

// A valid request should extend the token's life (sliding expiry) rather
// than let it march toward the moment it was first issued — an actively
// used session shouldn't die mid-task (issue #409).
func TestRequireSessionSlidesExpiry(t *testing.T) {
	resetSessionStore(t)
	original := time.Now().Add(time.Second) // about to expire
	token := issueTestToken(t, "files", original)

	req := httptest.NewRequest(http.MethodGet, "/core/files", nil)
	req.Header.Set("X-Dewey-Session-Token", token)
	c, _ := newTestContext(req)

	requireSession("files")(c)

	sessionStoreMu.Lock()
	entry, ok := sessionStore[token]
	sessionStoreMu.Unlock()
	if !ok {
		t.Fatal("expected the token to still be present")
	}
	if !entry.expiresAt.After(original) {
		t.Fatalf("expected expiry to move forward from %v, got %v", original, entry.expiresAt)
	}
}
