package personnel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"
)

var ErrQueryContextExpired = errors.New("query context expired")
var ErrQueryContextCAS = errors.New("query context revision advanced concurrently")

type QueryRevisions struct{ People, Configuration, Activity int64 }
type QueryContext struct {
	View            string          `json:"view"`
	Criteria        json.RawMessage `json:"criteria"`
	Total           int64           `json:"total"`
	Fingerprint     string          `json:"fingerprint"`
	ProtocolVersion int             `json:"protocolVersion"`
	Revisions       QueryRevisions  `json:"-"`
}
type QueryContextStore struct {
	client     redis.UniversalClient
	generation string
}

func NewQueryContextStore(client redis.UniversalClient, generation string) *QueryContextStore {
	return &QueryContextStore{client: client, generation: generation}
}
func (s *QueryContextStore) Create(ctx context.Context, sessionRef string, value QueryContext) (string, error) {
	return "", ErrNotImplemented
}
func (s *QueryContextStore) Load(ctx context.Context, sessionRef, id string) (QueryContext, error) {
	return QueryContext{}, ErrNotImplemented
}
func (s *QueryContextStore) Advance(ctx context.Context, sessionRef, id, fingerprint string, previous, next QueryRevisions) error {
	return ErrNotImplemented
}
