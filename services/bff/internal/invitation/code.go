package invitation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

type Code struct {
	Value  string
	Digest [32]byte
}

var ErrInvalid = errors.New("invalid invitation")

func Generate() (Code, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Code{}, err
	}
	return Code{Value: base64.RawURLEncoding.EncodeToString(raw), Digest: sha256.Sum256(raw)}, nil
}
func Digest(code string) ([32]byte, error) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(code)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != code {
		return [32]byte{}, ErrInvalid
	}
	return sha256.Sum256(raw), nil
}
