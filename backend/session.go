package main

import (
	"crypto/subtle"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// sessionTokenBytes is how many random bytes back a session token —
// randomSuffix hex-encodes them, so the token string itself is twice
// this length. 32 bytes is comfortably infeasible to guess.
const sessionTokenBytes = 32

// sessionTTL is how long a session token stays valid after its last use
// (sliding expiry — see requireSession). Generous enough that normal use
// never hits it (the frontend already self-clears its cached token after
// 90s idle, issue #351), tight enough that a leaked/stolen token doesn't
// stay valid indefinitely (issue #409).
const sessionTTL = 5 * time.Minute

type sessionEntry struct {
	group     string
	expiresAt time.Time
}

// sessionStore holds every currently-valid session token in memory, keyed
// by the token itself. Issued by exchangeForSession, checked and slid
// forward by requireSession. Deliberately not persisted anywhere — a
// server restart requires every client to re-authenticate with their
// password, an acceptable tradeoff for this single-instance admin tool.
// Cleanup is lazy (an expired entry is dropped the next time something
// tries to use it, in requireSession below) rather than a background
// sweep — at this tool's scale (a handful of admin users, 5-minute TTLs)
// an abandoned, never-reused entry is a few dozen bytes that was going to
// expire anyway; a dedicated GC goroutine would be solving a problem this
// deployment doesn't have.
var (
	sessionStoreMu sync.Mutex
	sessionStore   = make(map[string]sessionEntry)
)

// passwordEnvVar maps a route group name to the env var holding its
// password — shared by exchangeForSession here and requirePassword
// (auth.go), so both stay in sync on what "files"/"machines" mean.
func passwordEnvVar(group string) string {
	switch group {
	case "files":
		return "FILES_PASSWORD"
	case "machines":
		return "MACHINES_PASSWORD"
	}
	return ""
}

// exchangeForSession validates the password (X-Dewey-Password header,
// same constant-time check requirePassword has always done) and, on
// success, issues a fresh session token instead of the client going on
// to resend the raw password on every future request (issue #409). This
// is the only point where the actual password crosses the wire more than
// once per login. Registered directly on the core route group rather
// than behind requirePassword — this endpoint's entire job is to perform
// that check itself, once.
func exchangeForSession(group string) gin.HandlerFunc {
	want := os.Getenv(passwordEnvVar(group))

	return func(c *gin.Context) {
		got := c.GetHeader("X-Dewey-Password")

		// Same reasoning as requirePassword: constant-time compare to
		// avoid a timing side-channel, and want == "" fails closed so an
		// unconfigured password env var rejects every request rather than
		// accepting an empty one.
		if want == "" || got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		token, err := randomSuffix(sessionTokenBytes)
		if err != nil {
			Warn("failed to generate session token: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to create session"})
			return
		}

		expiresAt := time.Now().Add(sessionTTL)
		sessionStoreMu.Lock()
		sessionStore[token] = sessionEntry{group: group, expiresAt: expiresAt}
		sessionStoreMu.Unlock()

		c.JSON(http.StatusOK, gin.H{"token": token, "expires_at": expiresAt.Format(time.RFC3339)})
	}
}

// requireSession gates a route group behind a previously-issued session
// token (X-Dewey-Session-Token header) instead of the raw password
// itself (issue #409) — replaces requirePassword on every files/machines
// route except the /session exchange endpoint. A valid request slides
// the token's expiry forward by sessionTTL rather than letting it march
// toward a fixed cutoff, so an actively-used session doesn't die out from
// under an admin mid-task the way a fixed expiry would.
func requireSession(group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Dewey-Session-Token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		sessionStoreMu.Lock()
		entry, ok := sessionStore[token]
		if !ok || entry.group != group || time.Now().After(entry.expiresAt) {
			// Drop it if it was merely expired (not just unknown) — no
			// reason to keep a stale entry around once something's
			// actually tried to use it.
			if ok {
				delete(sessionStore, token)
			}
			sessionStoreMu.Unlock()
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		entry.expiresAt = time.Now().Add(sessionTTL)
		sessionStore[token] = entry
		sessionStoreMu.Unlock()

		c.Next()
	}
}
