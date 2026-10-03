package appstructure

// The wire transport technique is reused from fixed PR26 applications/fault_test.go.
// The COMMIT and durable data come from actual PG18; only its reply is withheld.
import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
)

type lostCommitAck struct {
	net.Conn
	pending *bytes.Reader
	dropped *atomic.Bool
}

func (c *lostCommitAck) Read(p []byte) (int, error) {
	if c.pending != nil && c.pending.Len() > 0 {
		return c.pending.Read(p)
	}
	h := make([]byte, 5)
	if _, e := io.ReadFull(c.Conn, h); e != nil {
		return 0, e
	}
	n := int(binary.BigEndian.Uint32(h[1:])) - 4
	if n < 0 || n > 16*1024*1024 {
		return 0, io.ErrUnexpectedEOF
	}
	body := make([]byte, n)
	if _, e := io.ReadFull(c.Conn, body); e != nil {
		return 0, e
	}
	if h[0] == 'C' && string(body) == "COMMIT\x00" {
		c.dropped.Store(true)
		c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(h, body...))
	return c.pending.Read(p)
}
func TestRealDefinitionCommitAckLossUsesOriginalOperation(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	cfg, e := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	if cfg.ConnConfig.TLSConfig != nil {
		t.Fatal("isolated plaintext fault transport required")
	}
	var dropped atomic.Bool
	cfg.AfterConnect = func(c context.Context, conn *pgx.Conn) error { _, e := conn.Exec(c, "SET ROLE auth_app"); return e }
	cfg.ConnConfig.DialFunc = func(c context.Context, network, address string) (net.Conn, error) {
		conn, e := (&net.Dialer{}).DialContext(c, network, address)
		if e != nil {
			return nil, e
		}
		return &lostCommitAck{Conn: conn, dropped: &dropped}, nil
	}
	bad, e := pgxpool.NewWithConfig(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer bad.Close()
	f.service.Application.Pool = bad
	in := input(t, f, 0, 0, field(t, f, "number", "1", map[string]any{}))
	w := f.call(t, "PUT", "/forms/"+view+"/definition", in)
	var envelope struct {
		Code string
		Data struct{ OperationID string }
	}
	json.Unmarshal(w.Body.Bytes(), &envelope)
	if !dropped.Load() || w.Code != 503 || envelope.Code != "APPLICATION_OPERATION_UNCONFIRMED" || envelope.Data.OperationID != in["operationId"] || len(w.Result().Cookies()) != 0 {
		t.Fatalf("lost real commit acknowledgement %d %s", w.Code, w.Body.String())
	}
	f.service.Application.Pool = f.runtime
	op, e := (&applications.Application{Pool: f.runtime}).Operation(context.Background(), session.Principal{UserID: f.actor, Record: session.Record{AuthVersion: "1"}}, in["operationId"].(string))
	if e != nil || op.Status != "confirmed" {
		t.Fatal("original operation recovery", e, op)
	}
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	var version, audits int
	e = f.owner.QueryRow(context.Background(), "SELECT schema_version,(SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$2) FROM applications.logical_tables WHERE id=$1", table, in["operationId"]).Scan(&version, &audits)
	if e != nil || version != 1 || audits != 1 {
		t.Fatal("unknown replay duplicated write", version, audits, e)
	}
}
