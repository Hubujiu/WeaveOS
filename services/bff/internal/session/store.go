package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const idleTTL = time.Hour

var (
	ErrNotFound     = errors.New("session not found")
	ErrInvalid      = errors.New("invalid session")
	validUUID       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	validGeneration = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	validDigest     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Record contains only facts needed for a server-side session, without replayable tokens.
type Record struct {
	UserID          string `json:"user_id"`
	SessionRef      string `json:"session_ref"`
	AuthVersion     string `json:"auth_version"`
	SchemaVersion   int    `json:"schema_version"`
	CreatedAtUnixMS int64  `json:"created_at_unix_ms"`
	LastSeenUnixMS  int64  `json:"last_seen_at_unix_ms"`
	CSRFTokenHash   string `json:"csrf_token_hash"`
}

type Store struct {
	client     redis.UniversalClient
	parseErr   error
	generation string
}

func NewStore(redisURL, generation string) *Store {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return &Store{parseErr: err}
	}
	if !validGeneration.MatchString(generation) {
		return &Store{parseErr: ErrInvalid}
	}
	// Authentication dependencies must fail within the caller's deadline.
	// Do not retry ambiguous session writes or spend the HTTP response budget dialing.
	options.ContextTimeoutEnabled = true
	options.DialTimeout = time.Second
	options.DialerRetries = 1
	options.ReadTimeout = time.Second
	options.WriteTimeout = time.Second
	options.PoolTimeout = time.Second
	options.MaxRetries = -1
	return &Store{client: redis.NewClient(options), generation: generation}
}

func (s *Store) ready() error {
	if s == nil || (s.parseErr == nil && s.client == nil) {
		return ErrInvalid
	}
	return s.parseErr
}

func randomToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	return base64.RawURLEncoding.EncodeToString(raw), raw, nil
}

func (s *Store) key(sid string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(sid)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != sid {
		return "", ErrInvalid
	}
	digest := sha256.Sum256(raw)
	return "ems:auth:session:" + s.generation + ":v1:" + hex.EncodeToString(digest[:]), nil
}

func validVersion(value string) bool {
	if value == "" || value[0] == '0' {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func validRecord(value Record) bool {
	return validUUID.MatchString(value.UserID) && validUUID.MatchString(value.SessionRef) &&
		validVersion(value.AuthVersion) && value.SchemaVersion == 1 &&
		value.CreatedAtUnixMS > 0 && value.LastSeenUnixMS >= value.CreatedAtUnixMS &&
		validDigest.MatchString(value.CSRFTokenHash)
}

func (s *Store) Create(ctx context.Context, input Record) (string, string, error) {
	if err := s.ready(); err != nil {
		return "", "", err
	}
	if !validUUID.MatchString(input.UserID) || !validUUID.MatchString(input.SessionRef) || !validVersion(input.AuthVersion) {
		return "", "", ErrInvalid
	}
	csrf, csrfRaw, err := randomToken()
	if err != nil {
		return "", "", err
	}
	csrfHash := sha256.Sum256(csrfRaw)
	now := time.Now().UnixMilli()
	value := Record{UserID: input.UserID, SessionRef: input.SessionRef, AuthVersion: input.AuthVersion,
		SchemaVersion: 1, CreatedAtUnixMS: now, LastSeenUnixMS: now,
		CSRFTokenHash: hex.EncodeToString(csrfHash[:])}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", "", err
	}
	for attempt := 0; attempt < 5; attempt++ {
		sid, _, err := randomToken()
		if err != nil {
			return "", "", err
		}
		key, _ := s.key(sid)
		created, err := s.client.SetNX(ctx, key, encoded, idleTTL).Result()
		if err != nil {
			return "", "", err
		}
		if created {
			return sid, csrf, nil
		}
	}
	return "", "", errors.New("session identifier collision")
}

func (s *Store) Load(ctx context.Context, sid string) (Record, error) {
	if err := s.ready(); err != nil {
		return Record{}, err
	}
	key, err := s.key(sid)
	if err != nil {
		return Record{}, err
	}
	value, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	var record Record
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil || !validRecord(record) {
		return Record{}, ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Record{}, ErrInvalid
	}
	ttl, err := s.client.PTTL(ctx, key).Result()
	if err != nil {
		return Record{}, err
	}
	if ttl <= 0 {
		return Record{}, ErrNotFound
	}
	if ttl > idleTTL {
		return Record{}, ErrInvalid
	}
	return record, nil
}

const touchScript = `
local raw = redis.call('GET', KEYS[1])
if not raw then return 0 end
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 or ttl > 3600000 then return 0 end
local ok, value = pcall(cjson.decode, raw)
if not ok or type(value) ~= 'table' or value.schema_version ~= 1 or value.session_ref ~= ARGV[1]
  or value.auth_version ~= ARGV[2] or value.user_id ~= ARGV[3]
  or value.csrf_token_hash ~= ARGV[4] then return 0 end
if type(value.created_at_unix_ms) ~= 'number' or value.created_at_unix_ms <= 0
  or type(value.last_seen_at_unix_ms) ~= 'number'
  or value.last_seen_at_unix_ms < value.created_at_unix_ms
  or math.floor(value.created_at_unix_ms) ~= value.created_at_unix_ms
  or math.floor(value.last_seen_at_unix_ms) ~= value.last_seen_at_unix_ms then return 0 end
local clock = redis.call('TIME')
local now_ms = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
value.last_seen_at_unix_ms = math.max(value.last_seen_at_unix_ms, now_ms)
local reply = redis.call('SET', KEYS[1], cjson.encode(value), 'XX', 'PX', 3600000)
if reply then return 1 end
return 0
`

func (s *Store) Touch(ctx context.Context, sid string, loaded Record) (bool, error) {
	if err := s.ready(); err != nil {
		return false, err
	}
	if !validRecord(loaded) {
		return false, ErrInvalid
	}
	key, err := s.key(sid)
	if err != nil {
		return false, err
	}
	result, err := s.client.Eval(ctx, touchScript, []string{key}, loaded.SessionRef,
		loaded.AuthVersion, loaded.UserID, loaded.CSRFTokenHash).Int64()
	return result == 1, err
}

func (s *Store) Revoke(ctx context.Context, sid string) (bool, error) {
	if err := s.ready(); err != nil {
		return false, err
	}
	key, err := s.key(sid)
	if err != nil {
		return false, err
	}
	removed, err := s.client.Del(ctx, key).Result()
	return removed == 1, err
}
