package apprecordservice

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rootCaptureMeteredConn struct {
	net.Conn
	readBytes *atomic.Int64
}

func (c *rootCaptureMeteredConn) Read(p []byte) (int, error) {
	n, e := c.Conn.Read(p)
	c.readBytes.Add(int64(n))
	return n, e
}
func TestRootEvidenceCaptureRejectsOversizeBeforeTransferringValue(t *testing.T) {
	f := rootCaptureSetup(t)
	if _, e := f.owner.Exec(f.ctx, "UPDATE "+rootCaptureRelation(f)+" SET "+rootCaptureColumn(f.public)+"=repeat('x',$2) WHERE id=$1", f.ownRecord, ev.MaxFieldBytes+1); e != nil {
		t.Fatal(e)
	}
	var received atomic.Int64
	cfg := f.runtime.Config()
	cfg.MaxConns = 1
	dial := cfg.ConnConfig.DialFunc
	if dial == nil {
		d := new(net.Dialer)
		dial = d.DialContext
	}
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		c, e := dial(ctx, network, address)
		if e != nil {
			return nil, e
		}
		return &rootCaptureMeteredConn{Conn: c, readBytes: &received}, nil
	}
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	tx, facts, e := (&applications.Application{Pool: pool}).BeginRecordRead(f.ctx, f.principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	received.Store(0)
	bundle, e := captureWorkflowEvidence(f.ctx, tx, facts, f.ownRecord)
	wireBytes := received.Load()
	if !errors.Is(e, ev.ErrTooLarge) || len(bundle.Manifest) != 0 {
		t.Fatalf("oversize capture returned %v", e)
	}
	if wireBytes > 256*1024 {
		t.Fatalf("oversize value was fetched before its budget check: %d transport bytes", wireBytes)
	}
	t.Logf("oversize rejection received %d transport bytes instead of a >4MiB field", wireBytes)
}
