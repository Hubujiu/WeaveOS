package applications

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type commitAckLoss struct {
	net.Conn
	pending *bytes.Reader
	dropped *atomic.Bool
}

func (c *commitAckLoss) Read(p []byte) (int, error) {
	if c.pending != nil && c.pending.Len() > 0 {
		return c.pending.Read(p)
	}
	h := make([]byte, 5)
	if _, err := io.ReadFull(c.Conn, h); err != nil {
		return 0, err
	}
	n := int(binary.BigEndian.Uint32(h[1:])) - 4
	if n < 0 || n > 16*1024*1024 {
		return 0, io.ErrUnexpectedEOF
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(c.Conn, b); err != nil {
		return 0, err
	}
	if h[0] == 'C' && string(b) == "COMMIT\x00" {
		c.dropped.Store(true)
		_ = c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(h, b...))
	return c.pending.Read(p)
}
func TestB5UnknownCommitOriginalKeyRecovery(t *testing.T) {
	f := fixture(t, true)
	cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.TLSConfig != nil {
		t.Fatal("isolated plaintext fault transport required")
	}
	var dropped atomic.Bool
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { _, err := c.Exec(ctx, "SET ROLE auth_app"); return err }
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &commitAckLoss{Conn: c, dropped: &dropped}, nil
	}
	bad, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Close()
	f.service.Application.Pool = bad
	op := f.operation(t)
	body := map[string]any{"name": "unknown-commit", "operationId": op}
	w := f.call("POST", "/api/v1/applications", body)
	if !dropped.Load() || w.Code != 503 || !strings.Contains(w.Body.String(), "APPLICATION_OPERATION_UNCONFIRMED") || len(w.Result().Cookies()) != 0 {
		t.Fatalf("lost real COMMIT acknowledgement must stay unconfirmed: %d %s", w.Code, w.Body.String())
	}
	f.service.Application.Pool = f.runtime
	d := data(t, f.call("GET", "/api/v1/application-operations/"+op, nil), 200)
	var result App
	if json.Unmarshal(d["result"], &result) != nil || result.ID == "" {
		t.Fatal("original operation query recovers durable result")
	}
	replayed := data(t, f.call("POST", "/api/v1/applications", body), 201)
	if value(t, replayed, "id") != result.ID {
		t.Fatal("retry must reuse original application")
	}
	var count int
	if err := f.owner.QueryRow(context.Background(), "SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$1", op).Scan(&count); err != nil || count != 1 {
		t.Fatal("unknown outcome recovery must not repeat business/audit")
	}
	if w := f.call("GET", "/api/v1/application-operations/"+f.uuid(t), nil); w.Code != 404 {
		t.Fatal("missing operation is 404, never a rollback claim")
	}
}

type revokeOnCommit struct {
	revoke func()
	once   atomic.Bool
}

func (t *revokeOnCommit) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, commitQueryKey{}, d.SQL)
}

type commitQueryKey struct{}

func (t *revokeOnCommit) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if ctx.Value(commitQueryKey{}) == "commit" && d.Err == nil && t.once.CompareAndSwap(false, true) {
		t.revoke()
	}
}
func TestB5CommittedWriteSurvivesRenewalRevocation(t *testing.T) {
	f := fixture(t, true)
	cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	trace := &revokeOnCommit{revoke: func() {
		if _, err := f.store.Revoke(context.Background(), f.sid); err != nil {
			t.Error(err)
		}
	}}
	cfg.ConnConfig.Tracer = trace
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { _, err := c.Exec(ctx, "SET ROLE auth_app"); return err }
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f.service.Application.Pool = pool
	op := f.operation(t)
	committed := f.call("POST", "/api/v1/applications", map[string]any{"name": "renewal", "operationId": op})
	data(t, committed, 201)
	if cookies := committed.Result().Cookies(); len(cookies) != 2 || cookies[0].MaxAge >= 0 || cookies[1].MaxAge >= 0 {
		t.Fatal("committed success must clear browser cookies after renewal failure")
	}
	if !trace.once.Load() {
		t.Fatal("real confirmed COMMIT must precede Session revocation")
	}
	w := f.call("GET", "/api/v1/applications", nil)
	if w.Code != 401 {
		t.Fatal("revocation must prevent next request")
	}
	var n int
	if err := f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2 AND http_status=201", f.actor, op).Scan(&n); err != nil || n != 1 {
		t.Fatal("confirmed write result persists after renewal failure")
	}
}
func TestB5AuditFailureRollsBackEntireCreate(t *testing.T) {
	f := fixture(t, true)
	ctx := context.Background()
	op := f.operation(t)
	// Isolated database-only fault: this exact operation's event is rejected.
	_, err := f.owner.Exec(ctx, `CREATE FUNCTION applications.b5_test_audit_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.change_summary->>'operationId'='`+op+`' THEN RAISE EXCEPTION 'isolated audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER b5_test_audit_fail BEFORE INSERT ON auth.authentication_events FOR EACH ROW EXECUTE FUNCTION applications.b5_test_audit_fail()`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.owner.Exec(ctx, "DROP TRIGGER b5_test_audit_fail ON auth.authentication_events; DROP FUNCTION applications.b5_test_audit_fail()")
	})
	name := "rollback-" + op
	w := f.call("POST", "/api/v1/applications", map[string]any{"name": name, "operationId": op})
	if w.Code != 503 {
		t.Fatalf("audit failure must reject write: %d %s", w.Code, w.Body.String())
	}
	for _, q := range []string{"SELECT count(*) FROM applications.apps WHERE name=$1", "SELECT count(*) FROM personnel.permission_catalog WHERE name=$1"} {
		var n int
		if err := f.owner.QueryRow(ctx, q, name).Scan(&n); err != nil || n != 0 {
			t.Fatal("failed audit must roll back app/menu/catalog")
		}
	}
	var n int
	if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&n); err != nil || n != 0 {
		t.Fatal("failed audit must roll back operation claim/result")
	}
}
func TestB5ConcurrentCASAndIdenticalOperation(t *testing.T) {
	f := fixture(t, true)
	app, _ := f.create(t)
	path := "/api/v1/applications/" + app + "/permission-groups"
	responses := make(chan *httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for _, op := range []string{f.operation(t), f.operation(t)} {
		go func(op string) {
			<-start
			responses <- f.call("POST", path, map[string]any{"name": "CAS", "operationId": op, "expectedPolicyRevision": 1})
		}(op)
	}
	close(start)
	statuses := map[int]int{}
	for range 2 {
		w := <-responses
		statuses[w.Code]++
	}
	if statuses[201] != 1 || statuses[409] != 1 {
		t.Fatalf("same revision has exactly one winner: %v", statuses)
	}
	op := f.operation(t)
	start = make(chan struct{})
	body := map[string]any{"name": "idem", "operationId": op, "expectedPolicyRevision": 2}
	for range 2 {
		go func() { <-start; responses <- f.call("POST", path, body) }()
	}
	close(start)
	first := data(t, <-responses, 201)
	second := data(t, <-responses, 201)
	if value(t, first, "id") != value(t, second, "id") {
		t.Fatal("same operation concurrent retry must reuse stable ID")
	}
	var rev, n int
	if err := f.owner.QueryRow(context.Background(), "SELECT policy_revision FROM applications.apps WHERE id=$1", app).Scan(&rev); err != nil || rev != 3 {
		t.Fatal("CAS winner and one idem execution increment once each")
	}
	if err := f.owner.QueryRow(context.Background(), "SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$1", op).Scan(&n); err != nil || n != 1 {
		t.Fatal("concurrent idem emits one audit")
	}
}
