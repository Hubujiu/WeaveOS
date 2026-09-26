package session

import (
	"context"
	"errors"
)

// Record contains only the facts required to validate a server-side session.
// SID and the readable CSRF token are independent credentials and are not fields here.
type Record struct {
	UserID          string
	SessionRef      string
	AuthVersion     string
	SchemaVersion   int
	CreatedAtUnixMS int64
	LastSeenUnixMS  int64
	CSRFTokenHash   string
}

type Store struct {
	RedisURL   string
	Generation string
}

func NewStore(redisURL, generation string) *Store {
	return &Store{RedisURL: redisURL, Generation: generation}
}

var errNotImplemented = errors.New("session storage not implemented")

func (*Store) Create(context.Context, Record) (string, string, error) {
	return "", "", errNotImplemented
}

func (*Store) Load(context.Context, string) (Record, error) {
	return Record{}, errNotImplemented
}

func (*Store) Touch(context.Context, string, Record) (bool, error) {
	return false, errNotImplemented
}

func (*Store) Revoke(context.Context, string) (bool, error) {
	return false, errNotImplemented
}
