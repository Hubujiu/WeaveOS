package personnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/redis/go-redis/v9"
)

func TestQ36RealRedisOldNewContextBytesAndTokens(t *testing.T) {
	ctx := context.Background()
	opts, err := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	generation := fmt.Sprintf("q36-parity-%d", time.Now().UnixNano())
	sessionRef := "00000000-0000-4000-8000-000000000001"
	hash := sha256.Sum256([]byte(sessionRef))
	prefix := "ems:personnel:query:" + generation + ":v1:{" + hex.EncodeToString(hash[:]) + "}:"
	t.Cleanup(func() {
		keys, _ := client.Keys(ctx, "ems:personnel:query:"+generation+":*").Result()
		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
	})
	fingerprint := strings.Repeat("a", 64)
	value := QueryContext{View: "members", Criteria: json.RawMessage(`{"search":"Alice","filter":null}`), Total: 23, Fingerprint: fingerprint, ProtocolVersion: 1, Revisions: QueryRevisions{People: 1, Configuration: 1}}
	wantData := fmt.Sprintf(`{"view":"members","criteria":{"search":"Alice","filter":null},"total":23,"fingerprint":"%s","protocolVersion":1}`, fingerprint)
	wantRevision := `{"People":1,"Configuration":1,"Activity":0}`
	legacy := NewQueryContextStore(client, generation)
	oldToken, err := legacy.Create(ctx, sessionRef, value)
	if err != nil {
		t.Fatal(err)
	}
	oldFields, err := client.HGetAll(ctx, prefix+oldToken).Result()
	if err != nil || len(oldFields) != 2 || oldFields["data"] != wantData || oldFields["revision"] != wantRevision {
		t.Fatalf("pre-extraction Redis bytes differ: fields=%v err=%v", oldFields, err)
	}
	policy := querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			var r QueryRevisions
			if json.Unmarshal(m.Revision, &r) != nil {
				return false
			}
			return validQueryMetadata(QueryContext{View: m.View, Criteria: m.Criteria, Total: m.Total, Fingerprint: m.Fingerprint, ProtocolVersion: m.ProtocolVersion, Revisions: r})
		},
		Forward: func(old, next json.RawMessage) bool {
			var a, b QueryRevisions
			return json.Unmarshal(old, &a) == nil && json.Unmarshal(next, &b) == nil && validQueryRevisions(a) && validQueryRevisions(b) && b.People >= a.People && b.Configuration >= a.Configuration && b.Activity >= a.Activity && (a.Activity != 0 || b.Activity == 0)
		},
	}
	shared := querycontext.NewStore(client, "personnel", generation, policy)
	loaded, err := shared.Load(ctx, sessionRef, oldToken)
	if err != nil || loaded.View != "members" || string(loaded.Criteria) != string(value.Criteria) || string(loaded.Revision) != wantRevision {
		t.Fatalf("old token must load in neutral store: %+v %v", loaded, err)
	}
	meta := querycontext.Metadata{View: value.View, Criteria: value.Criteria, Total: value.Total, Fingerprint: value.Fingerprint, ProtocolVersion: value.ProtocolVersion, Revision: json.RawMessage(wantRevision)}
	newToken, err := shared.Create(ctx, sessionRef, meta)
	if err != nil {
		t.Fatal(err)
	}
	newFields, err := client.HGetAll(ctx, prefix+newToken).Result()
	if err != nil || len(newFields) != 2 || newFields["data"] != wantData || newFields["revision"] != wantRevision {
		t.Fatalf("new context bytes must be readable by old code: fields=%v err=%v", newFields, err)
	}
	if old, err := legacy.Load(ctx, sessionRef, newToken); err != nil || old.View != value.View || string(old.Criteria) != string(value.Criteria) || old.Total != value.Total || old.Fingerprint != value.Fingerprint || old.Revisions != value.Revisions {
		t.Fatalf("new token must load in old store: %+v %v", old, err)
	}
	next := QueryRevisions{People: 2, Configuration: 1}
	nextBytes := json.RawMessage(`{"People":2,"Configuration":1,"Activity":0}`)
	if err := shared.Advance(ctx, sessionRef, oldToken, fingerprint, json.RawMessage(wantRevision), nextBytes); err != nil {
		t.Fatal(err)
	}
	if old, err := legacy.Load(ctx, sessionRef, oldToken); err != nil || old.Revisions != next {
		t.Fatalf("new CAS must advance old token: %+v %v", old, err)
	}
	if err := legacy.Advance(ctx, sessionRef, newToken, fingerprint, value.Revisions, next); err != nil {
		t.Fatal(err)
	}
	if now, err := shared.Load(ctx, sessionRef, newToken); err != nil || string(now.Revision) != string(nextBytes) {
		t.Fatalf("old CAS must advance new token: %+v %v", now, err)
	}
}

func TestQ36RealRedisContextOwnershipLimitsAndCAS(t *testing.T) {
	ctx := context.Background()
	opts, err := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	c := redis.NewClient(opts)
	defer c.Close()
	if err = c.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	generation := fmt.Sprintf("q36-%d", time.Now().UnixNano())
	store := NewQueryContextStore(c, generation)
	owner := "00000000-0000-4000-8000-000000000001"
	other := "00000000-0000-4000-8000-000000000002"
	value := QueryContext{View: "members", Criteria: json.RawMessage(`{"search":"Alice","filter":null}`), Total: 23, Fingerprint: strings.Repeat("a", 64), ProtocolVersion: 1, Revisions: QueryRevisions{People: 1, Configuration: 1}}
	ids := []string{}
	for i := 0; i < 20; i++ {
		id, e := store.Create(ctx, owner, value)
		if e != nil {
			t.Fatalf("valid metadata creation unavailable: %v", e)
		}
		ids = append(ids, id)
	}
	loaded, err := store.Load(ctx, owner, ids[0])
	if err != nil || loaded.Total != 23 || string(loaded.Criteria) != string(value.Criteria) {
		t.Fatalf("metadata must roundtrip: %+v %v", loaded, err)
	}
	if _, err = store.Load(ctx, other, ids[0]); !errors.Is(err, ErrQueryContextExpired) {
		t.Fatalf("foreign session must not read: %v", err)
	}
	last, err := store.Create(ctx, owner, value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(ctx, owner, ids[1]); !errors.Is(err, ErrQueryContextExpired) {
		t.Fatalf("LRU must evict untouched second context, not recently read first: %v", err)
	}
	if _, err = store.Load(ctx, owner, ids[0]); err != nil {
		t.Fatal("recently touched context evicted", err)
	}
	next := QueryRevisions{People: 2, Configuration: 1}
	if err = store.Advance(ctx, owner, last, value.Fingerprint, value.Revisions, next); err != nil {
		t.Fatal(err)
	}
	if err = store.Advance(ctx, owner, last, value.Fingerprint, value.Revisions, QueryRevisions{People: 3, Configuration: 1}); !errors.Is(err, ErrQueryContextCAS) {
		t.Fatalf("late response must not overwrite CAS: %v", err)
	}
	if err = store.Advance(ctx, owner, last, strings.Repeat("b", 64), next, QueryRevisions{People: 3, Configuration: 1}); !errors.Is(err, ErrQueryContextCAS) {
		t.Fatalf("cannot replace baseline fingerprint: %v", err)
	}
	loaded, err = store.Load(ctx, owner, last)
	if err != nil || loaded.Revisions != next || loaded.Fingerprint != value.Fingerprint {
		t.Fatalf("immutable baseline/monotonic revision: %+v %v", loaded, err)
	}
	// Inspect real Redis expiry and payload. Test manipulates only its own key to
	// exercise expiration without sleeping 30 minutes or pretending a fake clock.
	keys, err := c.Keys(ctx, "ems:personnel:query:"+generation+":*").Result()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Del(ctx, keys...)
	var target string
	for _, key := range keys {
		if strings.HasSuffix(key, ":"+last) {
			target = key
		}
	}
	if target == "" {
		t.Fatal("opaque context was not persisted")
	}
	ttl, err := c.PTTL(ctx, target).Result()
	if err != nil || ttl > 30*time.Minute || ttl < 29*time.Minute {
		t.Fatalf("idle TTL must be30m: %v %v", ttl, err)
	}
	if err = c.PExpire(ctx, target, 10*time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(ctx, owner, last); err != nil {
		t.Fatal(err)
	}
	ttl, _ = c.PTTL(ctx, target).Result()
	if ttl < 29*time.Minute {
		t.Fatal("successful access did not refresh idle TTL")
	}
	fields, err := c.HGetAll(ctx, target).Result()
	if err != nil {
		t.Fatal(err)
	}
	for key := range fields {
		if key != "data" && key != "revision" {
			t.Fatalf("unexpected metadata field: %s", key)
		}
	}
	if strings.Contains(fields["data"], `"items"`) || strings.Contains(fields["data"], `"rows"`) {
		t.Fatal("query store must not contain business results")
	}
	if err = c.PExpire(ctx, target, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(ctx, owner, last); !errors.Is(err, ErrQueryContextExpired) {
		t.Fatalf("lost context must be expired, not changed: %v", err)
	}
}
