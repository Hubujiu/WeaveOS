package invitation

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// Data dictionary: canonical Base64URL of 32 random bytes, SHA-256 of decoded bytes.
func TestCanonicalInvitationDigestVector(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	digest, err := Digest(token)
	if err != nil || hex.EncodeToString(digest[:]) != "66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925" {
		t.Fatal("decoded 32-byte independent digest vector must match")
	}
	for _, code := range []string{"", token + "=", strings.Repeat("a", 42), token[:42] + "B"} {
		if _, err := Digest(code); err == nil {
			t.Fatal("noncanonical or wrong-sized invitation accepted")
		}
	}
}
func TestGenerateIndependentCanonicalCodes(t *testing.T) {
	a, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(a.Value)
	if err != nil || len(raw) != 32 || len(a.Value) != 43 || a.Value == b.Value {
		t.Fatal("each invitation must be independent canonical32")
	}
	digest, err := Digest(a.Value)
	if err != nil || digest != a.Digest {
		t.Fatal("generated digest must belong to the displayed code")
	}
}
