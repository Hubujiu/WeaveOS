package security_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/security"
)

func TestCSRFTokenIsIndependentCanonicalSecretBoundBySHA256(t *testing.T) {
	token, digest, err := security.NewCSRFToken()
	if err != nil {
		t.Fatalf("generate separate CSRF secret: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		t.Fatalf("CSRF token is not canonical Base64URL of 32 bytes")
	}
	wantDigest := sha256.Sum256(raw)
	if digest != hex.EncodeToString(wantDigest[:]) {
		t.Errorf("CSRF hash does not bind the generated secret")
	}
	if !security.MatchesCSRFToken(token, digest) {
		t.Error("correct CSRF token did not match stored digest")
	}
	second, _, err := security.NewCSRFToken()
	if err != nil || second == token {
		t.Errorf("CSRF generation reused a secret or failed: %v", err)
	}
}

func TestCSRFTokenRejectsMalformedOrDifferentValue(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if !security.MatchesCSRFToken(token, digest) {
		t.Fatal("independent known token/hash pair must match")
	}
	for _, candidate := range []string{"", token + "=", token + "!", strings.Repeat("A", 43), base64.RawURLEncoding.EncodeToString(raw[:31])} {
		if security.MatchesCSRFToken(candidate, digest) {
			t.Errorf("malformed or unrelated token was accepted: %q", candidate)
		}
	}
	if security.MatchesCSRFToken(token, strings.ToUpper(digest)) || security.MatchesCSRFToken(token, strings.Repeat("0", 64)) {
		t.Error("noncanonical or unrelated digest was accepted")
	}
}
