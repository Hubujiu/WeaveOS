package applications

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func createPermission(t *testing.T, f *webFixture, template bool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := f.owner.QueryRow(ctx, "INSERT INTO personnel.identities(name) VALUES('b5-create') RETURNING id::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", f.actor, id); err != nil {
		t.Fatal(err)
	}
	if template {
		var tid string
		if err := f.owner.QueryRow(ctx, "INSERT INTO personnel.permission_templates(name) VALUES('b5-create-template') RETURNING id::text").Scan(&tid); err != nil {
			t.Fatal(err)
		}
		if _, err := f.owner.Exec(ctx, "INSERT INTO personnel.identity_templates(identity_id,template_id) VALUES($1,$2)", id, tid); err != nil {
			t.Fatal(err)
		}
		if _, err := f.owner.Exec(ctx, "INSERT INTO personnel.template_permissions(template_id,permission_code) VALUES($1,'applications.create')", tid); err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err := f.owner.Exec(ctx, "INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'applications.create')", id); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

type queryGate struct {
	prefix  string
	entered chan struct{}
	release chan struct{}
	once    atomic.Bool
}

func (g *queryGate) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(d.SQL, g.prefix) && g.once.CompareAndSwap(false, true) {
		close(g.entered)
		if g.release != nil {
			select {
			case <-g.release:
			case <-ctx.Done():
			}
		}
	}
	return ctx
}
func (g *queryGate) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func tracedPool(t *testing.T, g *queryGate) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Tracer = g
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { _, err := c.Exec(ctx, "SET ROLE auth_app"); return err }
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
func waitGate(t *testing.T, g *queryGate) {
	t.Helper()
	select {
	case <-g.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real SQL synchronization point not reached")
	}
}
func blocked(t *testing.T, f *webFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		var yes bool
		if err := f.owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND cardinality(pg_blocking_pids(pid))>0)").Scan(&yes); err != nil {
			t.Fatal(err)
		}
		if yes {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("real dependency blocking not observed")
		case <-time.After(5 * time.Millisecond):
		}
	}
}
func TestB5RevokeFirstPreventsWaitingCreate(t *testing.T) {
	for _, mode := range []string{"capability", "account"} {
		t.Run(mode, func(t *testing.T) {
			f := fixture(t, false)
			id := createPermission(t, f, false)
			ctx := context.Background()
			revoker, err := f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer revoker.Rollback(ctx)
			if _, err := revoker.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err != nil {
				t.Fatal(err)
			}
			if mode == "capability" {
				_, err = revoker.Exec(ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", id)
			} else {
				_, err = revoker.Exec(ctx, "UPDATE auth.users SET status='disabled',auth_version=auth_version+1 WHERE id=$1", f.actor)
			}
			if err != nil {
				t.Fatal(err)
			}
			g := &queryGate{prefix: "SELECT personnel.lock_query_revisions()", entered: make(chan struct{})}
			f.service.Application.Pool = tracedPool(t, g)
			op := f.operation(t)
			result := make(chan int, 1)
			go func() {
				result <- f.call("POST", "/api/v1/applications", map[string]any{"name": "revoke-first", "operationId": op}).Code
			}()
			waitGate(t, g)
			blocked(t, f)
			if err := revoker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			want := 403
			if mode == "account" {
				want = 401
			}
			if status := <-result; status != want {
				t.Fatalf("waiting create must recheck committed %s revoke: %d", mode, status)
			}
			var n int
			if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op).Scan(&n); err != nil || n != 0 {
				t.Fatal("revoked write must leave no operation/business result")
			}
		})
	}
}
func TestB5WriteFirstProtectedUntilCommitThenRevoke(t *testing.T) {
	f := fixture(t, false)
	id := createPermission(t, f, false)
	g := &queryGate{prefix: "UPDATE applications.operations SET result_json=", entered: make(chan struct{}), release: make(chan struct{})}
	f.service.Application.Pool = tracedPool(t, g)
	op := f.operation(t)
	writer := make(chan int, 1)
	go func() {
		writer <- f.call("POST", "/api/v1/applications", map[string]any{"name": "write-first", "operationId": op}).Code
	}()
	waitGate(t, g)
	revoked := make(chan error, 1)
	go func() {
		_, err := f.owner.Exec(context.Background(), "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", id)
		revoked <- err
	}()
	blocked(t, f)
	close(g.release)
	if status := <-writer; status != 201 {
		t.Fatalf("qualified protected writer must commit first: %d", status)
	}
	if err := <-revoked; err != nil {
		t.Fatal(err)
	}
	if w := f.call("POST", "/api/v1/applications", map[string]any{"name": "after-revoke", "operationId": f.operation(t)}); w.Code != 403 {
		t.Fatal("same Session's next create must observe revoke")
	}
	// Revocation of create capability cannot invalidate readable committed replay.
	data(t, f.call("POST", "/api/v1/applications", map[string]any{"name": "write-first", "operationId": op}), 201)
}
func TestB5CreateCapabilityViaPersonnelTemplate(t *testing.T) {
	f := fixture(t, false)
	createPermission(t, f, true)
	f.create(t)
}
