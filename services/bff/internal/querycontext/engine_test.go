package querycontext

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

var errFixtureDenied = errors.New("current principal denied")

type pgStrategy struct {
	db              *pgx.Conn
	table, revision string
	full, pages     int
	denied          bool
}

func (*pgStrategy) Resource() string { return "fixture" }
func (s *pgStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	if s.denied {
		return nil, errFixtureDenied
	}
	return s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
}
func canonicalPrefix(raw json.RawMessage) (json.RawMessage, error) {
	var c struct {
		Prefix string `json:"prefix"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || c.Prefix == "" {
		return nil, ErrInvalid
	}
	return json.Marshal(c)
}
func (s *pgStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	var old json.RawMessage
	if saved != nil {
		var err error
		old, err = canonicalPrefix(saved)
		if err != nil {
			return nil, nil, err
		}
	}
	if incoming == nil {
		return old, old, nil
	}
	current, err := canonicalPrefix(incoming)
	return old, current, err
}
func (s *pgStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	var revision int64
	err := tx.QueryRow(ctx, "SELECT rev FROM "+s.revision).Scan(&revision)
	if err != nil {
		return nil, err
	}
	return json.Marshal(revision)
}
func (s *pgStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page Page) (Observation[[]string], error) {
	s.full++
	var c struct {
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return Observation[[]string]{}, err
	}
	rows, err := tx.Query(ctx, "SELECT id,txt FROM "+s.table+" WHERE txt LIKE $1 ORDER BY id", c.Prefix+"%")
	if err != nil {
		return Observation[[]string]{}, err
	}
	defer rows.Close()
	all := []string{}
	h := sha256.New()
	for rows.Next() {
		var id int
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			return Observation[[]string]{}, err
		}
		fmt.Fprintf(h, "%d:%d:%s;", id, len(text), text)
		all = append(all, text)
	}
	if err := rows.Err(); err != nil {
		return Observation[[]string]{}, err
	}
	start := (page.Number - 1) * page.Size
	items := []string{}
	if start < len(all) {
		end := start + page.Size
		if end > len(all) {
			end = len(all)
		}
		items = all[start:end]
	}
	return Observation[[]string]{Items: items, Total: int64(len(all)), Fingerprint: hex.EncodeToString(h.Sum(nil))}, nil
}
func (s *pgStrategy) Page(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page Page) ([]string, error) {
	s.pages++
	var c struct {
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT txt FROM "+s.table+" WHERE txt LIKE $1 ORDER BY id LIMIT $2 OFFSET $3", c.Prefix+"%", page.Size, (page.Number-1)*page.Size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func TestRealPGRRRedisLifecycleAndReceipt(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	opts, err := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("v015_q36_extract_%d", time.Now().UnixNano())
	table := pgx.Identifier{name}.Sanitize()
	rev := pgx.Identifier{name + "_rev"}.Sanitize()
	if _, err := db.Exec(ctx, "CREATE TABLE "+table+"(id integer PRIMARY KEY,txt text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, "DROP TABLE "+table)
	if _, err := db.Exec(ctx, "CREATE TABLE "+rev+"(rev bigint NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, "DROP TABLE "+rev)
	if _, err := db.Exec(ctx, "INSERT INTO "+table+" VALUES(1,'apple'),(2,'apricot'),(3,'blueberry'); INSERT INTO "+rev+" VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	policy := Policy{
		Validate: func(m Metadata) bool {
			var n int64
			_, err := canonicalPrefix(m.Criteria)
			return m.View == "fixture" && err == nil && json.Unmarshal(m.Revision, &n) == nil && n > 0
		},
		Forward: func(previous, next json.RawMessage) bool {
			var a, b int64
			return json.Unmarshal(previous, &a) == nil && json.Unmarshal(next, &b) == nil && a > 0 && b >= a
		},
	}
	store := NewStore(client, "q36test", name, policy)
	sessionRef := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	prefix, err := store.Prefix(sessionRef)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		keys, _ := client.Keys(ctx, "ems:q36test:query:"+name+":*").Result()
		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
	}()
	strategy := &pgStrategy{db: db, table: table, revision: rev}
	criteria := json.RawMessage(`{"prefix":"a"}`)
	first, err := Execute(ctx, store, sessionRef, "", criteria, Page{1, 1}, strategy)
	if err != nil || first.Total != 2 || len(first.Items) != 1 || first.Items[0] != "apple" || first.Version == "" {
		t.Fatalf("new query: %+v %v", first, err)
	}
	second, err := Execute(ctx, store, sessionRef, first.Version, criteria, Page{2, 1}, strategy)
	if err != nil || second.Version != first.Version || len(second.Items) != 1 || second.Items[0] != "apricot" || strategy.full != 1 || strategy.pages != 1 {
		t.Fatalf("unchanged revision must use page path: %+v calls=%d/%d %v", second, strategy.full, strategy.pages, err)
	}
	if _, err := db.Exec(ctx, "UPDATE "+rev+" SET rev=2"); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(ctx, store, sessionRef, first.Version, criteria, Page{2, 1}, strategy); err != nil || strategy.full != 2 {
		t.Fatalf("equal P must recheck and CAS advance: %v", err)
	}
	stored, err := store.Load(ctx, sessionRef, first.Version)
	if err != nil || string(stored.Revision) != "2" {
		t.Fatalf("revision advance: %+v %v", stored, err)
	}
	if _, err := db.Exec(ctx, "UPDATE "+table+" SET txt='banana' WHERE id=2; UPDATE "+rev+" SET rev=3"); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(ctx, store, sessionRef, first.Version, criteria, Page{1, 1}, strategy); !errors.Is(err, ErrChanged) {
		t.Fatalf("changed old P must reject: %v", err)
	}
	stored, err = store.Load(ctx, sessionRef, first.Version)
	if err != nil || string(stored.Revision) != "2" {
		t.Fatalf("rejected read published Redis: %+v %v", stored, err)
	}
	if _, err := db.Exec(ctx, "UPDATE "+table+" SET txt='apricot' WHERE id=2; UPDATE "+rev+" SET rev=4"); err != nil {
		t.Fatal(err)
	}
	tx, receipt, err := ValidateSavedRead(ctx, store, sessionRef, first.Version, strategy)
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.Load(ctx, sessionRef, first.Version)
	if err != nil || string(stored.Revision) != "2" {
		t.Fatalf("receipt published before RR commit: %+v %v", stored, err)
	}
	if err := receipt.Commit(ctx, tx); err != nil {
		t.Fatal(err)
	}
	stored, err = store.Load(ctx, sessionRef, first.Version)
	if err != nil || string(stored.Revision) != "4" {
		t.Fatalf("receipt failed after RR commit: %+v %v", stored, err)
	}
	different, err := Execute(ctx, store, sessionRef, first.Version, json.RawMessage(`{"prefix":"b"}`), Page{2, 1}, strategy)
	if err != nil || different.Version == first.Version || different.Total != 1 || len(different.Items) != 0 {
		t.Fatalf("changed criteria valid deep page: %+v %v", different, err)
	}
	strategy.denied = true
	if _, err := Execute(ctx, store, sessionRef, "expired", criteria, Page{1, 1}, strategy); !errors.Is(err, errFixtureDenied) {
		t.Fatalf("live authorization must precede expired-token error: %v", err)
	}
	strategy.denied = false
	if n, err := client.Exists(ctx, prefix+different.Version).Result(); err != nil || n != 1 {
		t.Fatalf("new Redis context not committed: %d %v", n, err)
	}
}
