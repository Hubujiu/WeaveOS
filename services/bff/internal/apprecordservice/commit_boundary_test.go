package apprecordservice

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

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This transport reads actual PostgreSQL frames. It closes only after the
// server has sent the successful COMMIT tag, so the client loses the reply to
// a real committed transaction; no business transaction is mocked.
type lostRecordCommitAck struct {
	net.Conn
	pending *bytes.Reader
	dropped *atomic.Bool
}

func (c *lostRecordCommitAck) Read(p []byte) (int, error) {
	if c.pending != nil && c.pending.Len() > 0 {
		return c.pending.Read(p)
	}
	header := make([]byte, 5)
	if _, err := io.ReadFull(c.Conn, header); err != nil {
		return 0, err
	}
	length := int(binary.BigEndian.Uint32(header[1:])) - 4
	if length < 0 || length > 16*1024*1024 {
		return 0, io.ErrUnexpectedEOF
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.Conn, body); err != nil {
		return 0, err
	}
	if header[0] == 'C' && string(body) == "COMMIT\x00" && c.dropped.CompareAndSwap(false, true) {
		_ = c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(header, body...))
	return c.pending.Read(p)
}

// A real SQL error marks the transaction aborted immediately before COMMIT.
// PostgreSQL then returns ROLLBACK, which is definite rollback evidence.
type abortRecordBeforeCommit struct{ injected atomic.Bool }

func (a *abortRecordBeforeCommit) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if data.SQL == "commit" && a.injected.CompareAndSwap(false, true) {
		_, _ = conn.Exec(ctx, "SELECT 1/0")
	}
	return ctx
}
func (*abortRecordBeforeCommit) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func recordCommitFaultPool(t *testing.T, lost *atomic.Bool, abort *abortRecordBeforeCommit) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil || cfg.ConnConfig.TLSConfig != nil {
		t.Fatal("isolated plaintext PostgreSQL fault transport required", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE auth_app")
		return err
	}
	if lost != nil {
		cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &lostRecordCommitAck{Conn: conn, dropped: lost}, nil
		}
	}
	if abort != nil {
		cfg.ConnConfig.Tracer = abort
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func recordOperationID(t *testing.T, f recordFixture) string {
	t.Helper()
	var id string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func draftForCommitFault(t *testing.T, f recordFixture) appdrafts.Draft {
	t.Helper()
	draft, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: recordOperationID(t, f), SchemaVersion: 1, Values: map[string]any{f.public: "submit once"}}, applications.Metadata{RequestID: "v015-commit-draft-create"})
	if err != nil {
		t.Fatal(err)
	}
	return draft
}

func assertRecordCommitState(t *testing.T, f recordFixture, operation, draftID string, rows, drafts, audits, operations int) {
	t.Helper()
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	checks := []struct {
		query string
		args  []any
		want  int
	}{
		{"SELECT count(*) FROM " + relation, nil, rows},
		{"SELECT count(*) FROM applications.record_drafts WHERE id=$1", []any{draftID}, drafts},
		{"SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1 AND operation_id=$2", []any{f.app, operation}, audits},
		{"SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2 AND result_json IS NOT NULL", []any{f.actor, operation}, operations},
	}
	for _, check := range checks {
		var got int
		if err := f.runtime.QueryRow(f.ctx, check.query, check.args...).Scan(&got); err != nil || got != check.want {
			t.Fatalf("atomic state %q: got %d want %d err %v", check.query, got, check.want, err)
		}
	}
}

func TestRestrictedLostCommitAckRecoversExactRecordDraftAuditOperation(t *testing.T) {
	f := newRecordFixture(t)
	draft := draftForCommitFault(t, f)
	op := recordOperationID(t, f)
	req := CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "submit once"}, DraftRef: &DraftRef{ID: draft.ID, DraftVersion: 1}}
	meta := applications.Metadata{RequestID: "v015-lost-commit-ack"}
	var dropped atomic.Bool
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, &dropped, nil)
	if _, err := faulty.Create(f.ctx, f.principal, req, meta); !dropped.Load() || !errors.Is(err, applications.ErrUnconfirmed) {
		t.Fatalf("lost real COMMIT ack must be unconfirmed, dropped=%v err=%v", dropped.Load(), err)
	}
	assertRecordCommitState(t, f, op, draft.ID, 3, 0, 1, 1)
	confirmed, err := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, f.principal, op)
	if err != nil || confirmed.Status != "confirmed" || confirmed.HTTPStatus != 201 {
		t.Fatalf("original operation result %+v %v", confirmed, err)
	}
	var saved MutationResult
	if err = json.Unmarshal(confirmed.Result, &saved); err != nil || saved.ID == "" || saved.OperationID != op || saved.RecordVersion != 1 {
		t.Fatalf("durable minimum result %+v %v", saved, err)
	}
	replay, err := f.service.Create(f.ctx, f.principal, req, meta)
	if err != nil || replay != saved {
		t.Fatalf("original-key replay %+v want %+v err %v", replay, saved, err)
	}
	assertRecordCommitState(t, f, op, draft.ID, 3, 0, 1, 1)
	changed := req
	changed.Values = map[string]any{f.public: "different fingerprint"}
	if _, err = f.service.Create(f.ctx, f.principal, changed, meta); !errors.Is(err, applications.ErrOperationConflict) {
		t.Fatalf("changed fingerprint reused key: %v", err)
	}
	assertRecordCommitState(t, f, op, draft.ID, 3, 0, 1, 1)
}

func TestRestrictedDefinitePreCommitRollbackKeepsDraftAndOriginalKey(t *testing.T) {
	f := newRecordFixture(t)
	draft := draftForCommitFault(t, f)
	op := recordOperationID(t, f)
	req := CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "submit once"}, DraftRef: &DraftRef{ID: draft.ID, DraftVersion: 1}}
	meta := applications.Metadata{RequestID: "v015-definite-rollback"}
	abort := &abortRecordBeforeCommit{}
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, nil, abort)
	if _, err := faulty.Create(f.ctx, f.principal, req, meta); !abort.injected.Load() || !errors.Is(err, pgx.ErrTxCommitRollback) || errors.Is(err, applications.ErrUnconfirmed) {
		t.Fatalf("definite ROLLBACK must not be unknown: injected=%v err=%v", abort.injected.Load(), err)
	}
	assertRecordCommitState(t, f, op, draft.ID, 2, 1, 0, 0)
	if _, err := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, f.principal, op); !errors.Is(err, applications.ErrMissing) {
		t.Fatalf("rolled-back operation appeared confirmed: %v", err)
	}
	created, err := f.service.Create(f.ctx, f.principal, req, meta)
	if err != nil || created.RecordVersion != 1 {
		t.Fatalf("original-key retry after proven rollback %+v %v", created, err)
	}
	assertRecordCommitState(t, f, op, draft.ID, 3, 0, 1, 1)
}
