package personnel

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/redis/go-redis/v9"
)

// Preserve personnel error identity and the public Q36 API while the Redis
// scripts and lifecycle live in the one neutral querycontext package.
var ErrQueryContextExpired = querycontext.ErrExpired
var ErrQueryContextCAS = querycontext.ErrCAS

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
	client redis.UniversalClient // retained for existing package-level Redis tests
	shared *querycontext.Store
}

func NewQueryContextStore(client redis.UniversalClient, generation string) *QueryContextStore {
	policy := querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			var revision QueryRevisions
			return json.Unmarshal(m.Revision, &revision) == nil && validQueryMetadata(QueryContext{
				View: m.View, Criteria: m.Criteria, Total: m.Total, Fingerprint: m.Fingerprint,
				ProtocolVersion: m.ProtocolVersion, Revisions: revision,
			})
		},
		Forward: func(_ string, previous, next json.RawMessage) bool {
			var old, now QueryRevisions
			return json.Unmarshal(previous, &old) == nil && json.Unmarshal(next, &now) == nil &&
				validQueryRevisions(old) && validQueryRevisions(now) &&
				now.People >= old.People && now.Configuration >= old.Configuration && now.Activity >= old.Activity &&
				(old.Activity != 0 || now.Activity == 0)
		},
	}
	return &QueryContextStore{client: client, shared: querycontext.NewStore(client, "personnel", generation, policy)}
}

func personnelContextError(err error) error {
	if errors.Is(err, querycontext.ErrInvalid) {
		return ErrInvalid
	}
	return err
}
func (s *QueryContextStore) prefix(sessionRef string) (string, error) {
	if s == nil {
		return "", ErrInvalid
	}
	prefix, err := s.shared.Prefix(sessionRef)
	return prefix, personnelContextError(err)
}
func (s *QueryContextStore) Create(ctx context.Context, sessionRef string, value QueryContext) (string, error) {
	if s == nil || !validQueryMetadata(value) {
		return "", ErrInvalid
	}
	revision, _ := json.Marshal(value.Revisions)
	id, err := s.shared.Create(ctx, sessionRef, querycontext.Metadata{
		View: value.View, Criteria: value.Criteria, Total: value.Total, Fingerprint: value.Fingerprint,
		ProtocolVersion: value.ProtocolVersion, Revision: revision,
	})
	return id, personnelContextError(err)
}
func (s *QueryContextStore) Load(ctx context.Context, sessionRef, id string) (QueryContext, error) {
	if s == nil {
		return QueryContext{}, ErrInvalid
	}
	m, err := s.shared.Load(ctx, sessionRef, id)
	if err != nil {
		return QueryContext{}, personnelContextError(err)
	}
	var revision QueryRevisions
	if json.Unmarshal(m.Revision, &revision) != nil {
		return QueryContext{}, ErrInvalid
	}
	return QueryContext{View: m.View, Criteria: m.Criteria, Total: m.Total, Fingerprint: m.Fingerprint,
		ProtocolVersion: m.ProtocolVersion, Revisions: revision}, nil
}
func (s *QueryContextStore) Advance(ctx context.Context, sessionRef, id, fingerprint string, previous, next QueryRevisions) error {
	if s == nil {
		return ErrInvalid
	}
	before, _ := json.Marshal(previous)
	after, _ := json.Marshal(next)
	return personnelContextError(s.shared.Advance(ctx, sessionRef, id, fingerprint, before, after))
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
