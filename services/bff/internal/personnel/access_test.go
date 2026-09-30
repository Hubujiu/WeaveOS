package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

type fixture struct {
	owner            *pgxpool.Pool
	app              *Application
	actor            session.Principal
	i1, i2, template string
}

func TestAuthorizationLocksActualDependenciesQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// A current configuration row exists; a first-write insertion is protected
	// separately by its unique key and the revoker's matching insert-before-lock.
	if _, err := f.owner.Exec(ctx, "INSERT INTO personnel.member_configuration(user_id) VALUES($1)", f.actor.UserID); err != nil {
		t.Fatal(err)
	}
	tx, err := f.app.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := f.app.AuthorizeWrite(ctx, tx, f.actor); err != nil {
		t.Fatalf("qualified write must acquire dependency protection: %v", err)
	}
	for _, sql := range []string{
		"SELECT id FROM personnel.identities WHERE id='" + f.i1 + "' FOR UPDATE NOWAIT",
		"SELECT id FROM personnel.permission_templates WHERE id='" + f.template + "' FOR UPDATE NOWAIT",
		"SELECT user_id FROM personnel.member_configuration WHERE user_id='" + f.actor.UserID + "' FOR UPDATE NOWAIT",
		"SELECT code FROM personnel.permission_catalog WHERE code='personnel.manage' FOR UPDATE NOWAIT",
	} {
		other, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = other.Exec(ctx, sql)
		_ = other.Rollback(ctx)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "55P03" {
			t.Fatalf("revoker must wait for actual dependency lock, got %v", err)
		}
	}
	// No global policy lock: an unrelated identity remains independently writable.
	other, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(ctx)
	var id string
	if err := other.QueryRow(ctx, "INSERT INTO personnel.identities(name) VALUES('unrelated') RETURNING id::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Exec(ctx, "SELECT id FROM personnel.identities WHERE id=$1 FOR UPDATE NOWAIT", id); err != nil {
		t.Fatal("unrelated object blocked by authorization")
	}
}

func TestRootOnlyKnownEnabledApplicationsQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	actor := f.actor
	var root string
	err := f.owner.QueryRow(ctx, "SELECT id::text FROM auth.users WHERE is_bootstrap_admin").Scan(&root)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET is_bootstrap_admin=true WHERE id=$1", actor.UserID); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	} else {
		actor.UserID = root
		if err := f.owner.QueryRow(ctx, "SELECT auth_version::text FROM auth.users WHERE id=$1", root).Scan(&actor.Record.AuthVersion); err != nil {
			t.Fatal(err)
		}
	}
	actor.BootstrapAdmin = false // Trusted database fact wins over a false caller flag too.
	a, err := f.app.Me(ctx, actor)
	if err != nil {
		t.Fatalf("Root access missing: %v", err)
	}
	if !a.BootstrapAdmin || !a.PersonnelManage {
		t.Fatal("existing trusted Root must retain management")
	}
	if err := f.app.AllowApplication(ctx, actor, "test-B"); err != nil {
		t.Fatal("Root may enter an actual enabled application")
	}
	if err := f.app.AllowApplication(ctx, actor, "unknown"); !errors.Is(err, ErrDenied) {
		t.Fatal("Root must deny unknown applications")
	}
	if _, err := f.owner.Exec(ctx, "UPDATE personnel.permission_catalog SET enabled=false WHERE code='app.test.B'"); err != nil {
		t.Fatal(err)
	}
	if err := f.app.AllowApplication(ctx, actor, "test-B"); !errors.Is(err, ErrDenied) {
		t.Fatal("Root must deny disabled applications")
	}
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	owner, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	// All fixtures are synthetic and constrained to the isolated test store.
	tx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var user, i1, i2, template string
	for _, row := range []struct {
		sql string
		out *string
	}{
		{"INSERT INTO auth.users(account) VALUES('personnel-'||gen_random_uuid()) RETURNING id::text", &user},
		{"INSERT INTO personnel.identities(name) VALUES('I1') RETURNING id::text", &i1},
		{"INSERT INTO personnel.identities(name) VALUES('I2') RETURNING id::text", &i2},
		{"INSERT INTO personnel.permission_templates(name) VALUES('T') RETURNING id::text", &template},
	} {
		if err := tx.QueryRow(ctx, row.sql).Scan(row.out); err != nil {
			t.Fatal(err)
		}
	}
	_, err = tx.Exec(ctx, "INSERT INTO personnel.permission_catalog(code,name,category,app_id) VALUES('app.test.A','A','application','test-A'),('app.test.B','B','application','test-B') ON CONFLICT (code) DO UPDATE SET enabled=true")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2),($1,$3)", user, i1, i2)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'app.test.A'),($2,'app.test.A')", i1, i2)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO personnel.identity_templates(identity_id,template_id) VALUES($1,$2)", i1, template)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO personnel.template_permissions(template_id,permission_code) VALUES($1,'app.test.B'),($1,'personnel.manage')", template)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Delete owned relations explicitly; durable enterprise root and unrelated records remain.
		_, _ = owner.Exec(ctx, "DELETE FROM auth.authentication_events WHERE actor_user_id=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=ANY($1::uuid[])", []string{i1, i2})
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1", template)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.identity_templates WHERE identity_id=ANY($1::uuid[])", []string{i1, i2})
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.member_identities WHERE user_id=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.member_configuration WHERE user_id=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.identities WHERE id=ANY($1::uuid[])", []string{i1, i2})
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.permission_templates WHERE id=$1", template)
		_, _ = owner.Exec(ctx, "DELETE FROM auth.users WHERE id=$1", user)
		_, _ = owner.Exec(ctx, "DELETE FROM personnel.permission_catalog WHERE code IN ('app.test.A','app.test.B')")
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { _, err := c.Exec(ctx, "SET ROLE auth_app"); return err }
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	return &fixture{owner: owner, app: &Application{Pool: runtime}, actor: session.Principal{UserID: user, Record: session.Record{AuthVersion: "1"}}, i1: i1, i2: i2, template: template}
}
func codes(a Access) map[string]Permission {
	m := map[string]Permission{}
	for _, p := range a.Permissions {
		m[p.Code] = p
	}
	return m
}
func TestLiveUnionAndSourcesQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.app.Me(ctx, f.actor)
	if err != nil {
		t.Fatalf("real live union missing: %v", err)
	}
	grants := codes(a)
	if a.BootstrapAdmin || !a.PersonnelManage || len(a.Identities) != 2 || len(a.Applications) != 2 {
		t.Fatalf("independent I1/I2 + T fixture union incorrect")
	}
	if len(grants) != 3 || len(grants["app.test.A"].Sources) != 2 || len(grants["app.test.B"].Sources) != 1 || grants["app.test.B"].Sources[0].TemplateID != f.template {
		t.Fatal("dedupe permission, preserve all direct/template sources")
	}
	if _, err := f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='app.test.B'", f.template); err != nil {
		t.Fatal(err)
	}
	next, err := f.app.Me(ctx, f.actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := codes(next)["app.test.B"]; ok {
		t.Fatal("shared revoke must affect next request without relogin")
	}
	if len(codes(next)["app.test.A"].Sources) != 2 {
		t.Fatal("another identity source must survive revoke")
	}
}
func TestNoIdentityNoGrantAndNoForgedRootQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.owner.Exec(ctx, "DELETE FROM personnel.member_identities WHERE user_id=$1", f.actor.UserID); err != nil {
		t.Fatal(err)
	}
	forged := f.actor
	forged.BootstrapAdmin = true
	a, err := f.app.Me(ctx, forged)
	if err != nil {
		t.Fatal(err)
	}
	if a.BootstrapAdmin || a.PersonnelManage || len(a.Permissions) > 0 || len(a.Applications) > 0 {
		t.Fatal("caller flag and unassigned member must never gain permissions")
	}
	b, _ := json.Marshal(a)
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	for _, name := range []string{"identities", "permissions", "applications"} {
		if string(raw[name]) != "[]" {
			t.Fatalf("%s must be empty array", name)
		}
	}
}
func TestDisabledAndStaleAccountFailClosedQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	stale := f.actor
	stale.Record.AuthVersion = "0"
	if _, err := f.app.Me(ctx, stale); !errors.Is(err, session.ErrUnauthorized) {
		t.Fatalf("stale auth version must deny, got %v", err)
	}
	if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.actor.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.Me(ctx, f.actor); !errors.Is(err, session.ErrUnauthorized) {
		t.Fatalf("inactive account must deny, got %v", err)
	}
}
func TestWriteAuthorizationUsesCurrentConfigurationQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tx, err := f.app.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := f.app.AuthorizeWrite(ctx, tx, f.actor); err != nil {
		t.Fatalf("live qualified manager must authorize, got %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template); err != nil {
		t.Fatal(err)
	}
	tx, err = f.app.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := f.app.AuthorizeWrite(ctx, tx, f.actor); !errors.Is(err, ErrDenied) {
		t.Fatalf("revoke first must deny write, got %v", err)
	}
}
