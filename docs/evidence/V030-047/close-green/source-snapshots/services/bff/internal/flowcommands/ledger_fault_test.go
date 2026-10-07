package flowcommands

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type rootPersistent struct {
	ctx    context.Context
	dsn    string
	db     *pgx.Conn
	ledger Ledger
	schema string
}

func rootPersistentFixture(t *testing.T) rootPersistent {
	t.Helper()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	db, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal("isolated PG connection failed")
	}
	var suffix string
	if e = db.QueryRow(ctx, "SELECT replace(gen_random_uuid()::text,'-','')").Scan(&suffix); e != nil {
		t.Fatal(e)
	}
	name := "root_cmd_" + suffix
	s := pgx.Identifier{name}.Sanitize()
	_, e = db.Exec(ctx, "CREATE SCHEMA "+s+`;
 CREATE TABLE `+s+`.workflow_commands(command_id uuid PRIMARY KEY,command_json jsonb NOT NULL,command_hash bytea NOT NULL CHECK(octet_length(command_hash)=32),state text NOT NULL CHECK(state IN ('pending','success','no_effect')),receipt_json jsonb,created_at timestamptz NOT NULL DEFAULT now(),CHECK((state='pending')=(receipt_json IS NULL)));
 CREATE TABLE `+s+`.workflow_dispatch(command_id uuid PRIMARY KEY REFERENCES `+s+`.workflow_commands(command_id),created_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE `+s+`.effects(id int PRIMARY KEY,sequence bigint NOT NULL,fenced boolean NOT NULL);INSERT INTO `+s+`.effects VALUES(1,11,true);
 CREATE TABLE `+s+`.audit(command_id uuid PRIMARY KEY,outcome text NOT NULL)`)
	if e != nil {
		_ = db.Close(ctx)
		t.Fatal(e)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_, _ = db.Exec(cleanup, "DROP SCHEMA "+s+" CASCADE")
		_ = db.Close(cleanup)
	})
	return rootPersistent{ctx, dsn, db, Ledger{Namespace: name}, s}
}
func (f rootPersistent) accept(t *testing.T, c Command) {
	t.Helper()
	tx, e := f.db.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = f.ledger.AcceptInTx(f.ctx, tx, c); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
}
func (f rootPersistent) effect(c Command) func(context.Context, pgx.Tx, ApplyPlan) error {
	return func(ctx context.Context, tx pgx.Tx, p ApplyPlan) error {
		if _, e := tx.Exec(ctx, "UPDATE "+f.schema+".effects SET sequence=$1,fenced=false WHERE id=1", p.Sequence); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, "INSERT INTO "+f.schema+".audit VALUES($1,$2)", c.CommandID, p.Outcome)
		return e
	}
}
func (f rootPersistent) assert(t *testing.T, state string, dispatch, audit int, seq int64, fenced bool) {
	t.Helper()
	var gotState string
	var n int
	if e := f.db.QueryRow(f.ctx, "SELECT count(*),min(state) FROM "+f.schema+".workflow_commands").Scan(&n, &gotState); e != nil || n != 1 || gotState != state {
		t.Fatalf("ledger %d/%s want1/%s err%v", n, gotState, state, e)
	}
	for _, v := range []struct {
		table string
		want  int
	}{{"workflow_dispatch", dispatch}, {"audit", audit}} {
		if e := f.db.QueryRow(f.ctx, "SELECT count(*) FROM "+f.schema+"."+v.table).Scan(&n); e != nil || n != v.want {
			t.Fatalf("%s count%d want%d err%v", v.table, n, v.want, e)
		}
	}
	var gotSeq int64
	var gotFence bool
	if e := f.db.QueryRow(f.ctx, "SELECT sequence,fenced FROM "+f.schema+".effects WHERE id=1").Scan(&gotSeq, &gotFence); e != nil || gotSeq != seq || gotFence != fenced {
		t.Fatalf("effects %d/%v err%v", gotSeq, gotFence, e)
	}
}
func (f rootPersistent) parallel(t *testing.T, n int, fn func(int, pgx.Tx) error) []error {
	t.Helper()
	start := make(chan struct{})
	result := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			db, e := pgx.Connect(f.ctx, f.dsn)
			if e != nil {
				result <- errors.New("parallel PG connect failed")
				return
			}
			defer db.Close(context.Background())
			tx, e := db.Begin(f.ctx)
			if e != nil {
				result <- e
				return
			}
			defer tx.Rollback(context.Background())
			e = fn(i, tx)
			if e == nil {
				e = tx.Commit(f.ctx)
			}
			result <- e
		}(i)
	}
	close(start)
	wg.Wait()
	close(result)
	out := []error{}
	for e := range result {
		out = append(out, e)
	}
	return out
}
func TestRootLedgerConcurrentIdenticalAcceptance(t *testing.T) {
	f := rootPersistentFixture(t)
	c := rootCommand()
	for _, e := range f.parallel(t, 16, func(_ int, tx pgx.Tx) error {
		v, e := f.ledger.AcceptInTx(f.ctx, tx, c)
		if e == nil && (v.Command != c || v.State != "pending") {
			return errors.New("wrong duplicate entry")
		}
		return e
	}) {
		if e != nil {
			t.Fatal(e)
		}
	}
	f.assert(t, "pending", 1, 0, 11, true)
}
func TestRootLedgerConcurrentConflictingPayloadHasOneWinner(t *testing.T) {
	f := rootPersistentFixture(t)
	c := rootCommand()
	success, conflict := 0, 0
	for _, e := range f.parallel(t, 16, func(i int, tx pgx.Tx) error {
		v := c
		v.PayloadHash[1] = byte(i%2 + 1)
		_, e := f.ledger.AcceptInTx(f.ctx, tx, v)
		return e
	}) {
		if e == nil {
			success++
		} else if errors.Is(e, ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 8 || conflict != 8 {
		t.Fatalf("wins=%d conflicts=%d", success, conflict)
	}
	f.assert(t, "pending", 1, 0, 11, true)
}
func TestRootLedgerConcurrentResultAppliesExactlyOnce(t *testing.T) {
	f := rootPersistentFixture(t)
	c := rootCommand()
	f.accept(t, c)
	r := rootReceipt(t, c)
	for _, e := range f.parallel(t, 16, func(_ int, tx pgx.Tx) error { _, e := f.ledger.ApplyInTx(f.ctx, tx, c, r, 11, f.effect(c)); return e }) {
		if e != nil {
			t.Fatal(e)
		}
	}
	f.assert(t, "success", 0, 1, 12, false)
}

type rootLostCommit struct {
	net.Conn
	pending *bytes.Reader
	dropped *atomic.Bool
}

func (c *rootLostCommit) Read(p []byte) (int, error) {
	if c.pending != nil && c.pending.Len() > 0 {
		return c.pending.Read(p)
	}
	header := make([]byte, 5)
	if _, e := io.ReadFull(c.Conn, header); e != nil {
		return 0, e
	}
	n := int(binary.BigEndian.Uint32(header[1:])) - 4
	if n < 0 || n > 16*1024*1024 {
		return 0, io.ErrUnexpectedEOF
	}
	body := make([]byte, n)
	if _, e := io.ReadFull(c.Conn, body); e != nil {
		return 0, e
	}
	if header[0] == 'C' && string(body) == "COMMIT\x00" && c.dropped.CompareAndSwap(false, true) {
		_ = c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(header, body...))
	return c.pending.Read(p)
}
func (f rootPersistent) faultConnection(t *testing.T, dropped *atomic.Bool) *pgx.Conn {
	t.Helper()
	cfg, e := pgx.ParseConfig(f.dsn)
	if e != nil || cfg.TLSConfig != nil {
		t.Fatal("plaintext isolated PG fault transport required")
	}
	cfg.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, e := (&net.Dialer{}).DialContext(ctx, network, address)
		if e != nil {
			return nil, e
		}
		return &rootLostCommit{Conn: conn, dropped: dropped}, nil
	}
	db, e := pgx.ConnectConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal("fault PG connection failed")
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	return db
}
func TestRootLedgerLostAcceptCommitAckRetainsOriginalPending(t *testing.T) {
	f := rootPersistentFixture(t)
	c := rootCommand()
	var dropped atomic.Bool
	bad := f.faultConnection(t, &dropped)
	tx, e := bad.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.ledger.AcceptInTx(f.ctx, tx, c); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e == nil || !dropped.Load() {
		t.Fatalf("real commit ack was not lost: %v", e)
	}
	f.assert(t, "pending", 1, 0, 11, true)
	good, e := f.db.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer good.Rollback(f.ctx)
	entry, e := f.ledger.GetInTx(f.ctx, good, c.CommandID)
	if e != nil || entry.State != "pending" || entry.Command != c {
		t.Fatalf("unknown falsely became failure %+v %v", entry, e)
	}
	if _, e = f.ledger.AcceptInTx(f.ctx, good, c); e != nil {
		t.Fatal(e)
	}
	if e = good.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	f.assert(t, "pending", 1, 0, 11, true)
}
func TestRootLedgerLostApplyCommitAckRecoversSuccessWithoutReplayEffects(t *testing.T) {
	f := rootPersistentFixture(t)
	c := rootCommand()
	f.accept(t, c)
	r := rootReceipt(t, c)
	var dropped atomic.Bool
	bad := f.faultConnection(t, &dropped)
	tx, e := bad.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.ledger.ApplyInTx(f.ctx, tx, c, r, 11, f.effect(c)); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e == nil || !dropped.Load() {
		t.Fatalf("real commit ack was not lost: %v", e)
	}
	f.assert(t, "success", 0, 1, 12, false)
	good, e := f.db.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer good.Rollback(f.ctx)
	entry, e := f.ledger.GetInTx(f.ctx, good, c.CommandID)
	if e != nil || entry.State != "success" || entry.Receipt == nil || *entry.Receipt != r {
		t.Fatalf("durable receipt missing %+v %v", entry, e)
	}
	_, e = f.ledger.ApplyInTx(f.ctx, good, c, r, 12, func(context.Context, pgx.Tx, ApplyPlan) error {
		return fmt.Errorf("duplicate effect callback executed")
	})
	if e != nil {
		t.Fatal(e)
	}
	if e = good.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	f.assert(t, "success", 0, 1, 12, false)
}
