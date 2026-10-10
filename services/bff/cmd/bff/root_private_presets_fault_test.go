package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordhttp"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRootPrivatePresetsStoreAtomicLateFailureAndSameKeyConcurrency(t *testing.T) {
	for _, mode := range []string{"create", "update", "discard"} {
		t.Run(mode, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			s := rootPresetService(f)
			p := rootPresetActor(f, f.actor)
			r := rootPresetRequest(t, f, "事务方案")
			if mode != "create" {
				result, e := s.Create(f.ctx, p, r)
				if e != nil {
					t.Fatal(e)
				}
				r.ID = rootPresetReceipt(t, result, r.OperationID, 201, 1)
				r.ExpectedVersion = 1
				r.State.Name = "修改"
			}
			r.OperationID = f.id(t)
			name := "preset_fault_" + strings.ReplaceAll(r.OperationID, "-", "")
			fn := pgx.Identifier{"public", name}.Sanitize()
			trigger := pgx.Identifier{name}.Sanitize()
			_, e := f.owner.Exec(f.ctx, "CREATE FUNCTION "+fn+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation_id='"+r.OperationID+"'::uuid THEN RAISE EXCEPTION USING ERRCODE='P0001',MESSAGE='isolated preset receipt fault'; END IF; RETURN NEW; END $$; CREATE TRIGGER "+trigger+" BEFORE UPDATE ON applications.operations FOR EACH ROW EXECUTE FUNCTION "+fn+"()")
			if e != nil {
				t.Fatal(e)
			}
			defer func() {
				if _, e := f.owner.Exec(f.ctx, "DROP TRIGGER "+trigger+" ON applications.operations; DROP FUNCTION "+fn+"()"); e != nil {
					t.Error(e)
				}
			}()
			snapshot := func() string {
				var v string
				if e := f.owner.QueryRow(f.ctx, "SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY id),'[]')::text FROM applications.table_presets p WHERE app_id=$1", f.app).Scan(&v); e != nil {
					t.Fatal(e)
				}
				return v
			}
			before := snapshot()
			switch mode {
			case "create":
				_, e = s.Create(f.ctx, p, r)
			case "update":
				_, e = s.Update(f.ctx, p, r)
			case "discard":
				_, e = s.Delete(f.ctx, p, r)
			}
			if e == nil || errors.Is(e, applications.ErrUnconfirmed) {
				t.Fatal("known precommit receipt failure misclassified", e)
			}
			if after := snapshot(); before != after {
				t.Fatal("partial configuration mutation survived failure", before, after)
			}
			var n int
			if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, r.OperationID).Scan(&n); e != nil || n != 0 {
				t.Fatal("failed operation claim survived", n, e)
			}
		})
	}
	t.Run("same_key", func(t *testing.T) {
		f := rootHTTPResourceSetup(t)
		s := rootPresetService(f)
		p := rootPresetActor(f, f.actor)
		r := rootPresetRequest(t, f, "同键")
		var wg sync.WaitGroup
		var results [2]applications.Result
		var errs [2]error
		start := make(chan struct{})
		for i := range 2 {
			wg.Add(1)
			go func(i int) { defer wg.Done(); <-start; results[i], errs[i] = s.Create(f.ctx, p, r) }(i)
		}
		close(start)
		wg.Wait()
		if errs[0] != nil || errs[1] != nil || !rootPresetEqualJSON(results[0].Data, results[1].Data) {
			t.Fatal("same key not one result", errs)
		}
		var n int
		if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.table_presets WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 1 {
			t.Fatal("same key duplicated private configuration", n, e)
		}
	})
}

// Actual PostgreSQL commits; only its COMMIT acknowledgement is lost.
type rootPresetLostCommit struct {
	net.Conn
	pending *bytes.Reader
	dropped *atomic.Bool
}

func (c *rootPresetLostCommit) Read(p []byte) (int, error) {
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
		c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(header, body...))
	return c.pending.Read(p)
}

type rootPresetCommitHook struct {
	fn     func(context.Context)
	called atomic.Bool
}
type rootPresetCommitKey struct{}

func (h *rootPresetCommitHook) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, rootPresetCommitKey{}, strings.EqualFold(strings.TrimSpace(d.SQL), "commit"))
}
func (h *rootPresetCommitHook) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if commit, _ := ctx.Value(rootPresetCommitKey{}).(bool); commit && d.Err == nil && h.called.CompareAndSwap(false, true) {
		h.fn(ctx)
	}
}
func TestRootPrivatePresetsHTTPUnknownCommitAndPostcommitRevocation(t *testing.T) {
	for _, mode := range []string{"unknown", "revoke"} {
		t.Run(mode, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			op := f.id(t)
			cfg := f.runtime.Config()
			var dropped atomic.Bool
			revoked := make(chan error, 1)
			if mode == "unknown" {
				cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
					c, e := (&net.Dialer{}).DialContext(ctx, network, address)
					if e != nil {
						return nil, e
					}
					return &rootPresetLostCommit{Conn: c, dropped: &dropped}, nil
				}
			} else {
				cfg.ConnConfig.Tracer = &rootPresetCommitHook{fn: func(ctx context.Context) {
					ok, e := f.store.Revoke(ctx, f.sid)
					if !ok && e == nil {
						e = errors.New("expected current session revoke")
					}
					revoked <- e
				}}
			}
			pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer pool.Close()
			handler := &apprecordhttp.Service{Records: &apprecordservice.Service{Pool: pool, Limits: rootPresetService(f).Limits}, Authenticator: session.Authenticator{Sessions: f.store, DB: f.runtime}}
			server := httptest.NewTLSServer(handler)
			defer server.Close()
			handler.Authenticator.Origin = server.URL
			request, e := http.NewRequestWithContext(f.ctx, "POST", server.URL+rootPresetHTTPPath(f), strings.NewReader(rootPresetHTTPBody(t, f, op)))
			if e != nil {
				t.Fatal(e)
			}
			request.Header.Set("Origin", server.URL)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-CSRF-Token", f.csrf)
			request.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
			request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: f.csrf})
			response, e := server.Client().Do(request)
			if e != nil {
				t.Fatal(e)
			}
			defer response.Body.Close()
			raw, e := io.ReadAll(response.Body)
			if e != nil {
				t.Fatal(e)
			}
			var env struct {
				Code string
				Data map[string]json.RawMessage
				Meta map[string]string
			}
			if json.Unmarshal(raw, &env) != nil || env.Meta["requestId"] == "" || response.Header.Get("Cache-Control") != "no-store" || response.TLS == nil {
				t.Fatal("invalid real HTTPS response", string(raw))
			}
			if mode == "unknown" {
				if !dropped.Load() || response.StatusCode != 503 || env.Code != "APPLICATION_OPERATION_UNCONFIRMED" || len(env.Data) != 1 || rootHTTPString(t, env.Data, "operationId") != op || response.Header.Get("Location") != "" || len(response.Cookies()) != 0 {
					t.Fatal("ambiguous commit falsely successful", response.StatusCode, string(raw))
				}
			} else {
				if response.StatusCode != 201 || env.Code != "OK" || len(env.Data) != 3 || response.Header.Get("Location") == "" {
					t.Fatal("confirmed commit disguised as failure", response.StatusCode, string(raw))
				}
				select {
				case e := <-revoked:
					if e != nil {
						t.Fatal(e)
					}
				default:
					t.Fatal("successful response without synchronous postcommit revoke hook")
				}
				if len(response.Cookies()) != 2 {
					t.Fatal("revoked session not cleared")
				}
				for _, cookie := range response.Cookies() {
					if cookie.MaxAge >= 0 {
						t.Fatal("revoked session renewed")
					}
				}
			}
			saved, e := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, rootPresetActor(f, f.actor), op)
			if e != nil || saved.Status != "confirmed" {
				t.Fatal("actual committed result missing", e)
			}
			var n int
			if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.table_presets WHERE owner_user_id=$1 AND app_id=$2", f.actor, f.app).Scan(&n); e != nil || n != 1 {
				t.Fatal("actual commit count", n, e)
			}
		})
	}
}
