package querycontext

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrChanged = errors.New("complete query projection changed")

type Page struct{ Number, Size int }
type Observation[T any] struct {
	Items       T
	Total       int64
	Fingerprint string
}
type Result[T any] struct {
	Items    T
	Total    int64
	Version  string
	Criteria json.RawMessage
}
type Strategy[T any] interface {
	Resource() string
	OpenRead(context.Context) (pgx.Tx, error)
	Prepare(context.Context, pgx.Tx, json.RawMessage, json.RawMessage) (json.RawMessage, json.RawMessage, error)
	Revisions(context.Context, pgx.Tx) (json.RawMessage, error)
	Observe(context.Context, pgx.Tx, json.RawMessage, Page) (Observation[T], error)
	Page(context.Context, pgx.Tx, json.RawMessage, Page) (T, error)
}
type Receipt struct{}

// Behavior is added only after a target RED in real PostgreSQL and Redis.
func Execute[T any](context.Context, *Store, string, string, json.RawMessage, Page, Strategy[T]) (Result[T], error) {
	return Result[T]{}, ErrInvalid
}
func ValidateSavedRead[T any](context.Context, *Store, string, string, Strategy[T]) (pgx.Tx, Receipt, error) {
	return nil, Receipt{}, ErrInvalid
}
func (Receipt) Commit(context.Context, pgx.Tx) error { return ErrInvalid }
