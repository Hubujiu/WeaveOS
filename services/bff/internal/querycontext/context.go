// Package querycontext owns the single Redis-backed Q36 context lifecycle.
// Domain policy validates the canonical criteria and revision vector.
package querycontext

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

var ErrInvalid = errors.New("invalid query context")
var ErrExpired = errors.New("query context expired")
var ErrCAS = errors.New("query context revision advanced concurrently")

var generationPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// Revision is separate from data so Advance cannot replace the immutable
// criteria, total or fingerprint. JSON data field order matches Q36 v1.
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
	Forward  func(previous, next json.RawMessage) bool
}

type Store struct {
	client     redis.UniversalClient
	namespace  string
	generation string
	policy     Policy
}

func NewStore(client redis.UniversalClient, namespace, generation string, policy Policy) *Store {
	return &Store{client: client, namespace: namespace, generation: generation, policy: policy}
}

// Prefix preserves personnel's v1 key byte format and Redis Cluster hash tag.
func (s *Store) Prefix(sessionRef string) (string, error) {
	var id pgtype.UUID
	if s == nil || s.client == nil || !namespacePattern.MatchString(s.namespace) || !generationPattern.MatchString(s.generation) || id.Scan(sessionRef) != nil || !id.Valid || id.String() != sessionRef {
		return "", ErrInvalid
	}
	digest := sha256.Sum256([]byte(strings.ToLower(sessionRef)))
	return "ems:" + s.namespace + ":query:" + s.generation + ":v1:{" + hex.EncodeToString(digest[:]) + "}:", nil
}

func validToken(id string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(id)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == id
}
func validFingerprint(v string) bool {
	raw, err := hex.DecodeString(v)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == v
}
func (s *Store) valid(m Metadata) bool {
	return s != nil && s.policy.Validate != nil && m.ProtocolVersion == 1 && m.Total >= 0 && m.Total <= 9007199254740991 && validFingerprint(m.Fingerprint) && len(m.Criteria) <= 65536 && s.policy.Validate(m)
}

// Scores are monotonic across same-microsecond requests. Successful operations
// refresh the shared 20-entry, 30-minute Session LRU.
const touchLua = `
local function touch()
 local top=redis.call('ZREVRANGE',KEYS[1],0,0,'WITHSCORES')
 local score=1
 if #top>0 then score=tonumber(top[2])+1 end
 redis.call('ZADD',KEYS[1],score,ARGV[1])
 redis.call('PEXPIRE',KEYS[1],1800000)
 redis.call('PEXPIRE',KEYS[2],1800000)
end
`

var createScript = redis.NewScript(touchLua + `
if redis.call('EXISTS',KEYS[2])==1 then return 0 end
redis.call('HSET',KEYS[2],'data',ARGV[2],'revision',ARGV[3])
touch()
while redis.call('ZCARD',KEYS[1])>20 do
 local oldest=redis.call('ZRANGE',KEYS[1],0,0)[1]
 redis.call('ZREM',KEYS[1],oldest)
 redis.call('DEL',ARGV[4]..oldest)
end
return 1
`)
var loadScript = redis.NewScript(touchLua + `
local data=redis.call('HGET',KEYS[2],'data')
local revision=redis.call('HGET',KEYS[2],'revision')
if not data or not revision then redis.call('ZREM',KEYS[1],ARGV[1]);return false end
touch()
return {data,revision}
`)
var advanceScript = redis.NewScript(touchLua + `
local data=redis.call('HGET',KEYS[2],'data')
if not data then redis.call('ZREM',KEYS[1],ARGV[1]);return 0 end
if redis.call('HGET',KEYS[2],'revision')~=ARGV[2] or cjson.decode(data).fingerprint~=ARGV[4] then return -1 end
redis.call('HSET',KEYS[2],'revision',ARGV[3])
touch()
return 1
`)

func (s *Store) Create(ctx context.Context, sessionRef string, value Metadata) (string, error) {
	prefix, err := s.Prefix(sessionRef)
	if err != nil || !s.valid(value) {
		return "", ErrInvalid
	}
	token := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, token); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(token)
	var data bytes.Buffer
	enc := json.NewEncoder(&data)
	enc.SetEscapeHTML(false)
	if err = enc.Encode(value); err != nil {
		return "", err
	}
	result, err := createScript.Run(ctx, s.client, []string{prefix + "lru", prefix + id}, id, bytes.TrimSpace(data.Bytes()), []byte(value.Revision), prefix).Int()
	if err != nil {
		return "", err
	}
	if result != 1 {
		return "", ErrCAS
	}
	return id, nil
}

func (s *Store) Load(ctx context.Context, sessionRef, id string) (Metadata, error) {
	prefix, err := s.Prefix(sessionRef)
	if err != nil {
		return Metadata{}, err
	}
	if !validToken(id) {
		return Metadata{}, ErrExpired
	}
	result, err := loadScript.Run(ctx, s.client, []string{prefix + "lru", prefix + id}, id).Slice()
	if errors.Is(err, redis.Nil) {
		return Metadata{}, ErrExpired
	}
	if err != nil {
		return Metadata{}, err
	}
	if len(result) != 2 {
		return Metadata{}, ErrInvalid
	}
	data, ok := result[0].(string)
	revision, rok := result[1].(string)
	if !ok || !rok {
		return Metadata{}, ErrInvalid
	}
	var out Metadata
	if json.Unmarshal([]byte(data), &out) != nil {
		return Metadata{}, ErrInvalid
	}
	out.Revision = json.RawMessage(revision)
	if !s.valid(out) {
		return Metadata{}, ErrInvalid
	}
	return out, nil
}

func (s *Store) Advance(ctx context.Context, sessionRef, id, fingerprint string, previous, next json.RawMessage) error {
	prefix, err := s.Prefix(sessionRef)
	if err != nil {
		return err
	}
	if !validToken(id) {
		return ErrExpired
	}
	if !validFingerprint(fingerprint) || s.policy.Forward == nil || !s.policy.Forward(previous, next) {
		return ErrInvalid
	}
	result, err := advanceScript.Run(ctx, s.client, []string{prefix + "lru", prefix + id}, id, []byte(previous), []byte(next), fingerprint).Int()
	if err != nil {
		return err
	}
	switch result {
	case 0:
		return ErrExpired
	case -1:
		return ErrCAS
	case 1:
		return nil
	default:
		return ErrInvalid
	}
}
