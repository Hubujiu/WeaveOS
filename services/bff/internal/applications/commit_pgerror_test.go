package applications

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Replace an actual successful COMMIT frame with a PostgreSQL ErrorResponse.
// ReadyForQuery still comes from the real server; no transaction is mocked.
type pgErrorAfterCommit struct {
	net.Conn
	pending  *bytes.Reader
	code     string
	injected *atomic.Bool
}

const privateCommitCause = "synthetic-private-commit-cause"

func (c *pgErrorAfterCommit) Read(p []byte) (int, error) {
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
	body := make([]byte, n)
	if _, err := io.ReadFull(c.Conn, body); err != nil {
		return 0, err
	}
	if h[0] == 'C' && string(body) == "COMMIT\x00" && c.injected.CompareAndSwap(false, true) {
		body = []byte("SERROR\x00VERROR\x00C" + c.code + "\x00M" + privateCommitCause + "\x00\x00")
		h[0] = 'E'
		binary.BigEndian.PutUint32(h[1:], uint32(len(body)+4))
	}
	c.pending = bytes.NewReader(append(h, body...))
	return c.pending.Read(p)
}
func commitPgPool(t *testing.T, code string, injected *atomic.Bool) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.TLSConfig != nil {
		t.Fatal("isolated plaintext fault transport required")
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { _, err := c.Exec(ctx, "SET ROLE auth_app"); return err }
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &pgErrorAfterCommit{Conn: c, code: code, injected: injected}, nil
	}
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
func principal(t *testing.T, f *webFixture) session.Principal {
	t.Helper()
	r, err := f.store.Load(context.Background(), f.sid)
	if err != nil {
		t.Fatal(err)
	}
	return session.Principal{UserID: f.actor, SessionRef: r.SessionRef, Record: r, SID: f.sid}
}

func TestB5CommitPgErrorAfterSuccessIsUnconfirmed(t *testing.T) {
	for _, code := range []string{"40003", "23514"} {
		for _, kind := range []string{"create", "policy"} {
			t.Run(code+"/"+kind, func(t *testing.T) {
				f := fixture(t, true)
				path := "/api/v1/applications"
				method := "POST"
				op := f.operation(t)
				body := map[string]any{"name": "pgerror", "operationId": op}
				app := ""
				wantRev := int64(1)
				if kind == "policy" {
					app, _ = f.create(t)
					base := "/api/v1/applications/" + app + "/permission-groups"
					gid := value(t, data(t, f.call("POST", base, map[string]any{"name": "pgerror", "operationId": f.operation(t), "expectedPolicyRevision": 1}), 201), "id")
					path = base + "/" + gid + "/members"
					method = "PUT"
					body = map[string]any{"memberIds": []string{}, "operationId": op, "expectedPolicyRevision": 2}
					wantRev = 3
				}
				var injected atomic.Bool
				f.service.Application.Pool = commitPgPool(t, code, &injected)
				w := f.call(method, path, body)
				if !injected.Load() {
					t.Fatal("actual successful COMMIT frame was not intercepted")
				}
				if w.Code != 503 || !strings.Contains(w.Body.String(), "APPLICATION_OPERATION_UNCONFIRMED") {
					t.Errorf("post-commit PgError %s must stay UNCONFIRMED: %d %s", code, w.Code, w.Body.String())
				}
				if strings.Contains(w.Body.String(), privateCommitCause) || len(w.Result().Cookies()) != 0 || w.Header().Get("Location") != "" {
					t.Fatal("unknown outcome must not expose internal cause, cookies or successful result")
				}
				f.service.Application.Pool = f.runtime
				d := data(t, f.call("GET", "/api/v1/application-operations/"+op, nil), 200)
				if value(t, d, "status") != "confirmed" {
					t.Fatal("original operation must recover real confirmed result")
				}
				var result map[string]json.RawMessage
				if json.Unmarshal(d["result"], &result) != nil {
					t.Fatal("durable result")
				}
				if kind == "create" {
					app = value(t, result, "id")
				}
				wantStatus := 200
				if kind == "create" {
					wantStatus = 201
				}
				data(t, f.call(method, path, body), wantStatus)
				var rev int64
				var count int
				if err := f.owner.QueryRow(context.Background(), "SELECT policy_revision FROM applications.apps WHERE id=$1", app).Scan(&rev); err != nil || rev != wantRev {
					t.Fatal("original-key recovery must not repeat revision")
				}
				if err := f.owner.QueryRow(context.Background(), "SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$1", op).Scan(&count); err != nil || count != 1 {
					t.Fatal("original-key recovery must not repeat audit")
				}
			})
		}
	}
}
func TestB5UnconfirmedRetainsInternalPgErrorCause(t *testing.T) {
	f := fixture(t, true)
	var injected atomic.Bool
	a := &Application{Pool: commitPgPool(t, "40003", &injected)}
	op := f.operation(t)
	_, err := a.Write(context.Background(), principal(t, f), "application.create", "", "", Input{Name: "cause", OperationID: op}, Metadata{RequestID: "commit-cause-test"})
	var pg *pgconn.PgError
	if !injected.Load() || !errors.Is(err, ErrUnconfirmed) || !errors.As(err, &pg) || pg.Code != "40003" || pg.Message != privateCommitCause {
		t.Fatalf("unconfirmed error must retain internal PgError without inferring rollback: %v", err)
	}
}

type abortBeforeCommit struct{ once atomic.Bool }

func (a *abortBeforeCommit) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if d.SQL == "commit" && a.once.CompareAndSwap(false, true) {
		_, _ = c.Exec(ctx, "SELECT 1/0")
	}
	return ctx
}
func (a *abortBeforeCommit) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func TestB5ExplicitCommitRollbackIsDefinite(t *testing.T) {
	f := fixture(t, true)
	cfg := f.runtime.Config()
	abort := &abortBeforeCommit{}
	cfg.ConnConfig.Tracer = abort
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	op := f.operation(t)
	_, err = (&Application{Pool: pool}).Write(context.Background(), principal(t, f), "application.create", "", "", Input{Name: "rollback", OperationID: op}, Metadata{RequestID: "commit-rollback-test"})
	if !abort.once.Load() || !errors.Is(err, pgx.ErrTxCommitRollback) || errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("actual ROLLBACK command tag is definite rollback evidence: %v", err)
	}
	if w := f.call("GET", "/api/v1/application-operations/"+op, nil); w.Code != 404 {
		t.Fatal("explicit rollback must leave no durable operation")
	}
}
