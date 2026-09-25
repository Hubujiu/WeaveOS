package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

func NewCSRFToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hex.EncodeToString(digest[:]), nil
}

func MatchesCSRFToken(token, digest string) bool {
	if len(digest) != 64 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return false
	}
	want, err := hex.DecodeString(digest)
	if err != nil || len(want) != 32 || hex.EncodeToString(want) != digest {
		return false
	}
	actual := sha256.Sum256(raw)
	return subtle.ConstantTimeCompare(actual[:], want) == 1
}
