package personnel

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

	"github.com/redis/go-redis/v9"
)

var ErrQueryContextExpired = errors.New("query context expired")
var ErrQueryContextCAS = errors.New("query context revision advanced concurrently")
var queryGeneration = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type QueryRevisions struct{ People, Configuration, Activity int64 }

// Criteria contains normalized conditions/fixed range, never page data or IDs of
// matched results. Revisions are kept separately for atomic monotonic CAS.
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

const queryIdleMS = 1800000

// Index scores form a monotonic access order, independent of client clocks and
// same-microsecond requests. All keys share the session hash tag for Redis cluster.
const queryTouchLua = `
local function touch()
 local top=redis.call('ZREVRANGE',KEYS[1],0,0,'WITHSCORES')
 local score=1
 if #top>0 then score=tonumber(top[2])+1 end
 redis.call('ZADD',KEYS[1],score,ARGV[1])
 redis.call('PEXPIRE',KEYS[1],1800000)
 redis.call('PEXPIRE',KEYS[2],1800000)
end
`

var createQueryScript = redis.NewScript(queryTouchLua + `
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
var loadQueryScript = redis.NewScript(queryTouchLua + `
local data=redis.call('HGET',KEYS[2],'data')
local revision=redis.call('HGET',KEYS[2],'revision')
if not data or not revision then redis.call('ZREM',KEYS[1],ARGV[1]);return false end
touch()
return {data,revision}
`)
var advanceQueryScript = redis.NewScript(queryTouchLua + `
local data=redis.call('HGET',KEYS[2],'data')
if not data then redis.call('ZREM',KEYS[1],ARGV[1]);return 0 end
if redis.call('HGET',KEYS[2],'revision')~=ARGV[2] or cjson.decode(data).fingerprint~=ARGV[4] then return -1 end
redis.call('HSET',KEYS[2],'revision',ARGV[3])
touch()
return 1
`)

func (s *QueryContextStore) prefix(sessionRef string) (string, error) {
	if s == nil || s.client == nil || !queryGeneration.MatchString(s.generation) || !validID(sessionRef) {
		return "", ErrInvalid
	}
	digest := sha256.Sum256([]byte(strings.ToLower(sessionRef)))
	return "ems:personnel:query:" + s.generation + ":v1:{" + hex.EncodeToString(digest[:]) + "}:", nil
}
func validQueryID(id string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(id)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == id
}
func validFingerprint(v string) bool {
	raw, err := hex.DecodeString(v)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == v
}
func validQueryRevisions(r QueryRevisions) bool {
	return r.People > 0 && r.Configuration > 0 && r.Activity >= 0
}
func validQueryMetadata(v QueryContext) bool {
	if v.ProtocolVersion != 1 || v.Total < 0 || v.Total > maxSafeVersion || !validFingerprint(v.Fingerprint) || !validQueryRevisions(v.Revisions) || len(v.Criteria) > 65536 {
		return false
	}
	if v.View != "members" && v.View != "events" || v.View == "events" && v.Revisions.Activity == 0 || v.View == "members" && v.Revisions.Activity != 0 {
		return false
	}
	// A closed top-level vocabulary forbids a caller from persisting result sets.
	// The query engine independently validates values and frozen time bounds.
	var fields map[string]json.RawMessage
	if json.Unmarshal(v.Criteria, &fields) != nil || fields == nil {
		return false
	}
	for k, raw := range fields {
		switch k {
		case "search", "departmentId", "identityId", "action":
			var text string
			if json.Unmarshal(raw, &text) != nil {
				return false
			}
			if (k == "departmentId" || k == "identityId") && text != "" && !validID(text) {
				return false
			}
			if k == "action" && text != "" && !activityActions[text] {
				return false
			}
		case "filter":
			if string(raw) != "null" {
				if _, err := CompileFilter(v.View, raw, 1); err != nil {
					return false
				}
			}
		case "sort":
			if string(raw) == "null" {
				continue
			}
			var sort QuerySort
			if !decodeQueryMetadata(raw, &sort) || v.View != "events" || sort.Key != "occurredAt" || (sort.Direction != "asc" && sort.Direction != "desc") {
				return false
			}
		case "range":
			var r QueryRange
			if !decodeQueryMetadata(raw, &r) || v.View != "events" || r.From.IsZero() || !r.From.Before(r.To) {
				return false
			}
		case "timeBounds":
			var bounds []QueryRange
			if !decodeQueryMetadata(raw, &bounds) || len(bounds) > 20 {
				return false
			}
			for _, r := range bounds {
				if r.From.IsZero() || !r.From.Before(r.To) {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func decodeQueryMetadata(raw []byte, out any) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(out) == nil
}
func (s *QueryContextStore) Create(ctx context.Context, sessionRef string, value QueryContext) (string, error) {
	prefix, err := s.prefix(sessionRef)
	if err != nil || !validQueryMetadata(value) {
		return "", ErrInvalid
	}
	token := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, token); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(token)
	var data bytes.Buffer
	e := json.NewEncoder(&data)
	e.SetEscapeHTML(false)
	if err = e.Encode(value); err != nil {
		return "", err
	}
	revision, _ := json.Marshal(value.Revisions)
	result, err := createQueryScript.Run(ctx, s.client, []string{prefix + "lru", prefix + id}, id, bytes.TrimSpace(data.Bytes()), revision, prefix).Int()
	if err != nil {
		return "", err
	}
	if result != 1 {
		return "", ErrQueryContextCAS
	}
	return id, nil
}
func (s *QueryContextStore) Load(ctx context.Context, sessionRef, id string) (QueryContext, error) {
	prefix, err := s.prefix(sessionRef)
	if err != nil {
		return QueryContext{}, err
	}
	if !validQueryID(id) {
		return QueryContext{}, ErrQueryContextExpired
	}
	result, err := loadQueryScript.Run(ctx, s.client, []string{prefix + "lru", prefix + id}, id).Slice()
	if errors.Is(err, redis.Nil) {
		return QueryContext{}, ErrQueryContextExpired
	}
	if err != nil {
		return QueryContext{}, err
	}
	if len(result) != 2 {
		return QueryContext{}, ErrInvalid
	}
	data, ok := result[0].(string)
	revision, rok := result[1].(string)
	if !ok || !rok {
		return QueryContext{}, ErrInvalid
	}
	var out QueryContext
	if json.Unmarshal([]byte(data), &out) != nil || json.Unmarshal([]byte(revision), &out.Revisions) != nil || !validQueryMetadata(out) {
		return QueryContext{}, ErrInvalid
	}
	return out, nil
}
func (s *QueryContextStore) Advance(ctx context.Context, sessionRef, id, fingerprint string, previous, next QueryRevisions) error {
	prefix, err := s.prefix(sessionRef)
	if err != nil {
		return err
	}
	if !validQueryID(id) {
		return ErrQueryContextExpired
	}
	if !validFingerprint(fingerprint) || !validQueryRevisions(previous) || !validQueryRevisions(next) || next.People < previous.People || next.Configuration < previous.Configuration || next.Activity < previous.Activity || previous.Activity == 0 && next.Activity != 0 {
		return ErrInvalid
	}
	before, _ := json.Marshal(previous)
	after, _ := json.Marshal(next)
	result, err := advanceQueryScript.Run(ctx, s.client, []string{prefix + "lru", prefix + id}, id, before, after, fingerprint).Int()
	if err != nil {
		return err
	}
	switch result {
	case 0:
		return ErrQueryContextExpired
	case -1:
		return ErrQueryContextCAS
	case 1:
		return nil
	default:
		return ErrInvalid
	}
}
