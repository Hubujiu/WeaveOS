package security_test

import (
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
)

func TestOnlyActiveMatchingAuthenticationVersionKeepsSessionValid(t *testing.T) {
	if !security.CurrentUserMatchesSession("1", "active", 1) {
		t.Fatal("active user with the exact login-time version must remain authenticated")
	}
	for _, candidate := range []struct {
		name, snapshot, status string
		version                int64
	}{
		{"password reset invalidates all old sessions", "1", "active", 2},
		{"disabled account", "1", "disabled", 1},
		{"reenabled account uses a new version", "1", "active", 3},
		{"noncanonical leading zero", "01", "active", 1},
		{"noninteger JSON-like version", "1.0", "active", 1},
		{"negative snapshot", "-1", "active", 1},
		{"zero current version", "0", "active", 0},
		{"unknown account state", "1", "pending", 1},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			if security.CurrentUserMatchesSession(candidate.snapshot, candidate.status, candidate.version) {
				t.Error("invalidated or malformed session was accepted")
			}
		})
	}
}
