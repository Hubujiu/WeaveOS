package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

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
