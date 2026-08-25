package main

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// requirePassword gates a route group behind a shared secret, layered on
// top of logConnections' known_machines allowlist rather than replacing it
// (issue #332). group selects which env var to check against, so different
// route groups (e.g. "files", "machines") can be gated by separate
// credentials instead of one shared password for everything.
func requirePassword(group string) gin.HandlerFunc {
	var envVar string
	switch group {
	case "files":
		envVar = "FILES_PASSWORD"
	case "machines":
		envVar = "MACHINES_PASSWORD"
	}
	want := os.Getenv(envVar)

	return func(c *gin.Context) {
		got := c.GetHeader("X-Dewey-Password")

		// subtle.ConstantTimeCompare instead of a plain == avoids a timing
		// side-channel: a byte-by-byte comparison returns faster the sooner
		// it hits a mismatch, which would let an attacker guess the
		// password one byte at a time by measuring response latency.
		//
		// want == "" fails closed rather than open — an unset env var
		// (envVar itself unset, or FILES_PASSWORD/MACHINES_PASSWORD never
		// configured) must reject every request, not accept an empty
		// password.
		if want == "" || got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.Next()
	}
}
