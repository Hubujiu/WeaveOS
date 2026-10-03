// Package querycontext will host the shared Q36 context lifecycle.
// This is a test-loading declaration only; behavior follows a recorded RED.
package querycontext

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"
)

var ErrInvalid = errors.New("invalid query context")
var ErrExpired = errors.New("query context expired")
var ErrCAS = errors.New("query context revision advanced concurrently")

type Metadata struct {
	View            string          `json:"view"`
	Criteria        json.RawMessage `json:"criteria"`
	Total           int64           `json:"total"`
	Fingerprint     string          `json:"fingerprint"`
	ProtocolVersion int             `json:"protocolVersion"`
	Revision        json.RawMessage `json:"-"`
}
type Policy struct {
	Validate func(Metadata) bool
	Forward  func(view string, previous, next json.RawMessage) bool
}
type Store struct{}

func NewStore(_ redis.UniversalClient, _, _ string, _ Policy) *Store    { return &Store{} }
func (*Store) Create(context.Context, string, Metadata) (string, error) { return "", ErrInvalid }
func (*Store) Load(context.Context, string, string) (Metadata, error)   { return Metadata{}, ErrExpired }
func (*Store) Advance(context.Context, string, string, string, json.RawMessage, json.RawMessage) error {
	return ErrInvalid
}
