package personnel

import (
	"context"
	"errors"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// B5.2/B5.6: application writes need a current protected access snapshot, not
// a personnel.manage prerequisite. A registered synthetic capability exercises
// existing central identity/template facts without freezing a new product code.
func TestB5WriteAccessDoesNotRequirePersonnelManage(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template); err != nil {
		t.Fatal(err)
	}
	tx := b5WriteTx(t, f)
	actor := f.actor
	actor.BootstrapAdmin = true // Caller flag is not a trusted database fact.
	got, err := f.app.AccessForWrite(ctx, tx, actor)
	if err != nil {
		t.Fatalf("current application-capability holder needs a snapshot without personnel.manage: %v", err)
	}
	if got.PersonnelManage || got.BootstrapAdmin || got.User.ID != actor.UserID {
		t.Fatal("live DB access must not copy caller flags or impose personnel management")
	}
	if len(codes(got)["app.test.A"].Sources) != 2 || len(codes(got)["app.test.B"].Sources) != 1 {
		t.Fatal("transaction snapshot must preserve actual direct and template sources")
	}
}

func TestB5WriteAccessRechecksAccount(t *testing.T) {
	for _, mode := range []string{"stale", "disabled", "absent"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			actor := f.actor
			switch mode {
			case "stale":
				actor.Record.AuthVersion = "0"
			case "disabled":
				if _, err := f.owner.Exec(context.Background(), "UPDATE auth.users SET status='disabled' WHERE id=$1", actor.UserID); err != nil {
					t.Fatal(err)
				}
			case "absent":
				actor.UserID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
			}
			_, err := f.app.AccessForWrite(context.Background(), b5WriteTx(t, f), actor)
			if !errors.Is(err, session.ErrUnauthorized) {
				t.Fatalf("current account gate must reject %s, got %v", mode, err)
			}
		})
	}
}

func TestB5WriteAccessProtectsActualDependencies(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// Probe a committed dependency. A row first inserted by the writer is
	// invisible to this second transaction and is protected by unique-key wait,
	// not by locking an as-yet-invisible row (the approved Q25 distinction).
	if _, err := f.owner.Exec(ctx, "INSERT INTO personnel.member_configuration(user_id) VALUES($1)", f.actor.UserID); err != nil {
		t.Fatal(err)
	}
	tx := b5WriteTx(t, f)
	if _, err := f.app.AccessForWrite(ctx, tx, f.actor); err != nil {
		t.Fatalf("qualified transaction must protect actual dependencies: %v", err)
	}
	for _, probe := range []struct {
		sql string
		arg string
	}{
		{"SELECT id FROM auth.users WHERE id=$1 FOR UPDATE NOWAIT", f.actor.UserID},
		{"SELECT user_id FROM personnel.member_configuration WHERE user_id=$1 FOR UPDATE NOWAIT", f.actor.UserID},
		{"SELECT id FROM personnel.identities WHERE id=$1 FOR UPDATE NOWAIT", f.i1},
		{"SELECT id FROM personnel.permission_templates WHERE id=$1 FOR UPDATE NOWAIT", f.template},
		{"SELECT code FROM personnel.permission_catalog WHERE code=$1 FOR UPDATE NOWAIT", "app.test.A"},
	} {
		other, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = other.Exec(ctx, probe.sql, probe.arg)
		_ = other.Rollback(ctx)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "55P03" {
			t.Fatalf("revoker must wait for protected actual dependency %s: %v", probe.sql, err)
		}
	}
}

func TestB5WriteAccessObservesRevokeWithoutRelogin(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tx := b5WriteTx(t, f)
	before, err := f.app.AccessForWrite(ctx, tx, f.actor)
	if err != nil {
		t.Fatalf("pre-revoke snapshot: %v", err)
	}
	if _, ok := codes(before)["app.test.B"]; !ok {
		t.Fatal("synthetic template grant missing")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='app.test.B'", f.template); err != nil {
		t.Fatal(err)
	}
	next, err := f.app.AccessForWrite(ctx, b5WriteTx(t, f), f.actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := codes(next)["app.test.B"]; ok {
		t.Fatal("same session must observe template revoke in next write transaction")
	}
	if len(codes(next)["app.test.A"].Sources) != 2 {
		t.Fatal("independent direct grants must survive template revoke")
	}
}

func b5WriteTx(t *testing.T, f *fixture) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := f.app.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err != nil {
		t.Fatal(err)
	}
	return tx
}
