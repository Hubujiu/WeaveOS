package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"golang.org/x/crypto/argon2"
	"strings"
	"testing"
)

// PRD FR-018: uppercase/lowercase ASCII letters, digit and special symbol; no additional length floor.
// Printable ASCII boundaries approved in Q13 are covered in password_ascii_test.go.
func TestPasswordPolicy(t *testing.T) {
	for _, s := range []string{"Aa1!", "Abc@123456", "VeryLongTest@123"} {
		if !validPassword(s) {
			t.Errorf("four-class password rejected")
		}
	}
	for _, s := range []string{"", "aa1!", "AA1!", "Aa!!", "Aa11"} {
		if validPassword(s) {
			t.Errorf("missing class accepted")
		}
	}
}
func TestPasswordHashIsSaltedAndVerifiable(t *testing.T) {
	const password = "Aa1!"
	a, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("each password hash must have an independent random salt")
	}
	parts := strings.Split(a, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=19456,t=2,p=1" {
		t.Fatal("expected bounded Argon2id PHC")
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		t.Fatal("expected 16-byte salt")
	}
	digest, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(digest) != 32 {
		t.Fatal("expected 32-byte hash")
	}
	independentlyDerived := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	if subtle.ConstantTimeCompare(digest, independentlyDerived) != 1 {
		t.Fatal("stored digest does not verify independently")
	}
	if !verifyPassword(password, a) || verifyPassword("wrong", a) {
		t.Fatal("credential verification is incorrect")
	}
}
func TestVerifyIndependentPHCAndRejectMalformed(t *testing.T) {
	salt := []byte("0123456789abcdef")
	digest := argon2.IDKey([]byte("Aa1!"), salt, 2, 19456, 1, 32)
	phc := "$argon2id$v=19$m=19456,t=2,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(digest)
	if !verifyPassword("Aa1!", phc) {
		t.Fatal("must accept independently generated supported PHC")
	}
	for _, hash := range []string{"Aa1!", "", strings.Replace(phc, "m=19456", "m=999999999", 1), strings.Replace(phc, "v=19", "v=20", 1), phc + "$extra", strings.Replace(phc, "t=2", "t=0", 1)} {
		if verifyPassword("Aa1!", hash) {
			t.Fatal("malformed or unbounded PHC accepted")
		}
	}
}
