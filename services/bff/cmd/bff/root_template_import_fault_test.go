package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRootTemplateImportFaultRollsBackFullDDLMetadataAndReceipt(t *testing.T) {
	for _, phase := range []string{"form", "workflow", "audit", "receipt"} {
		t.Run(phase, func(t *testing.T) {
			f, m, b, p, _ := rootTemplateImportSource(t)
			op := f.id(t)
			name := "template_fault_" + strings.ReplaceAll(op, "-", "")
			table, event, condition := "applications.form_views", "INSERT", "NEW.app_id IN (SELECT id FROM applications.apps WHERE owner_user_id='"+p.UserID+"'::uuid)"
			switch phase {
			case "workflow":
				table = "applications.workflow_versions"
				condition = "NEW.created_by='" + p.UserID + "'::uuid"
			case "audit":
				table = "auth.authentication_events"
				condition = "NEW.change_summary->>'operationId'='" + op + "'"
			case "receipt":
				table = "applications.operations"
				event = "UPDATE"
				condition = "NEW.operation_id='" + op + "'::uuid"
			}
			fn := pgx.Identifier{"public", name}.Sanitize()
			trigger := pgx.Identifier{name}.Sanitize()
			if _, e := f.owner.Exec(f.ctx, "CREATE FUNCTION "+fn+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF "+condition+" THEN RAISE EXCEPTION USING ERRCODE='P0001',MESSAGE='isolated template fault'; END IF; RETURN NEW; END $$; CREATE TRIGGER "+trigger+" BEFORE "+event+" ON "+table+" FOR EACH ROW EXECUTE FUNCTION "+fn+"()"); e != nil {
				t.Fatal(e)
			}
			defer func() {
				if _, e := f.owner.Exec(f.ctx, "DROP TRIGGER "+trigger+" ON "+table+"; DROP FUNCTION "+fn+"()"); e != nil {
					t.Error(e)
				}
			}()
			before := rootTemplateReadCounts(t, f)
			if _, e := (&apptemplates.Service{Pool: f.runtime}).Import(f.ctx, p, op, m, b, applications.Metadata{RequestID: "fault-import"}); e == nil {
				t.Fatal("injected late fault succeeded")
			}
			if after := rootTemplateReadCounts(t, f); !reflect.DeepEqual(before, after) {
				t.Fatal("late failure retained app, audit, operation, source config or physical table", before, after)
			}
			var n int
			if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM personnel.permission_catalog WHERE app_id NOT IN (SELECT id::text FROM applications.apps) AND category='application'").Scan(&n); e != nil || n != 0 {
				t.Fatal("orphan app catalog registration", n, e)
			}
		})
	}
}

// Actual PG commits the import; only the wire acknowledgement is withheld.
type rootTemplateLostCommit struct {
	net.Conn
	pending *bytes.Reader
	dropped *atomic.Bool
}

func (c *rootTemplateLostCommit) Read(p []byte) (int, error) {
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
	if h[0] == 'C' && string(body) == "COMMIT\x00" && c.dropped.CompareAndSwap(false, true) {
		c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(h, body...))
	return c.pending.Read(p)
}
func TestRootTemplateImportCommitReplyLossRecoversOriginalWithoutDuplicate(t *testing.T) {
	f, m, b, p, _ := rootTemplateImportSource(t)
	op := f.id(t)
	cfg := f.runtime.Config()
	if cfg.ConnConfig.TLSConfig != nil {
		t.Fatal("isolated plaintext test transport required")
	}
	var dropped atomic.Bool
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		c, e := (&net.Dialer{}).DialContext(ctx, network, address)
		if e != nil {
			return nil, e
		}
		return &rootTemplateLostCommit{Conn: c, dropped: &dropped}, nil
	}
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	_, e = (&apptemplates.Service{Pool: pool}).Import(f.ctx, p, op, m, b, applications.Metadata{RequestID: "lost-commit"})
	if !dropped.Load() || !errors.Is(e, applications.ErrUnconfirmed) {
		t.Fatal("lost actual commit was not unconfirmed", dropped.Load(), e)
	}
	saved, e := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, p, op)
	if e != nil || saved.Status != "confirmed" {
		t.Fatal("durable result not recoverable", saved, e)
	}
	r, e := (&apptemplates.Service{Pool: f.runtime}).Import(f.ctx, p, op, m, b, applications.Metadata{RequestID: "recover"})
	if e != nil {
		t.Fatal(e)
	}
	app := rootTemplateImportReceipt(t, r, op)
	if !rootTemplateJSONEqual(saved.Result, r.Data) {
		t.Fatal("recovery changed original result")
	}
	var count int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$1", op).Scan(&count); e != nil || count != 1 {
		t.Fatal("retry duplicated audit", count, e)
	}
	out, e := (&apptemplates.Service{Pool: f.runtime}).ExportManifest(f.ctx, p, app)
	if e != nil || len(out.Tables) != 1 || len(out.Workflows) != 1 {
		t.Fatal("committed structure incomplete", e)
	}
}
func TestRootTemplateImportConcurrentSameKeyHasOneApplication(t *testing.T) {
	f, m, b, p, _ := rootTemplateImportSource(t)
	op := f.id(t)
	s := apptemplates.Service{Pool: f.runtime}
	start := make(chan struct{})
	var wg sync.WaitGroup
	var results [2]applications.Result
	var errs [2]error
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = s.Import(f.ctx, p, op, m, b, applications.Metadata{RequestID: "concurrent"})
		}(i)
	}
	close(start)
	wg.Wait()
	var successful int
	for i, e := range errs {
		if e == nil {
			rootTemplateImportReceipt(t, results[i], op)
			successful++
		} else if !errors.Is(e, applications.ErrOperationConflict) {
			t.Fatal("unexpected concurrency outcome", e)
		}
	}
	if successful < 1 {
		t.Fatal("no concurrent import accepted", errs)
	}
	r, e := s.Import(f.ctx, p, op, m, b, applications.Metadata{})
	if e != nil {
		t.Fatal(e)
	}
	app := rootTemplateImportReceipt(t, r, op)
	for i, e := range errs {
		if e == nil && !rootTemplateJSONEqual(results[i].Data, r.Data) {
			t.Fatal("two successful requests created different apps")
		}
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.apps WHERE owner_user_id=$1", p.UserID).Scan(&n); e != nil || n != 1 {
		t.Fatal("concurrent orphan or duplicate app", app, n, e)
	}
}
