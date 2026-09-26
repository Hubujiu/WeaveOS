package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

func validPassword(s string) bool {
	var upper, lower, digit, special bool
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
		switch {
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= '0' && r <= '9':
			digit = true
		case r >= 33 && r <= 126:
			special = true
		}
	}
	return upper && lower && digit && special
}
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	digest := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(digest)), nil
}
func verifyPassword(password, hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=19456,t=2,p=1" {
		return false
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	digest, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(digest) != 32 {
		return false
	}
	candidate := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	return subtle.ConstantTimeCompare(candidate, digest) == 1
}
