package personnel

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"net/http/httptest"
	"testing"
	"time"
)

// Independent Q25 ordering oracle. Observe actual PostgreSQL blockers, never a
// sleep guessed to mean a request has authenticated or acquired authorization.
func awaitBlockedBy(t *testing.T, f *fixture, pid int32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var blocked bool
		err := f.owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&blocked)
		if err != nil {
			t.Fatal("failed to observe real transaction blocking")
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("expected real transaction did not block")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func lockedDepartment(t *testing.T, f *fixture) (Department, pgx.Tx, int32) {
	t.Helper()
	ctx := context.Background()
	d, err := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "before", ParentID: rootDepartment(t, f)}, RequestMetadata{RequestID: "11111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	cleanDepartment(t, f, d.ID)
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	if _, err = tx.Exec(ctx, "SELECT id FROM personnel.departments WHERE id=$1 FOR UPDATE", d.ID); err != nil {
		t.Fatal(err)
	}
	var pid int32
	if err = tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	return d, tx, pid
}
func TestPersonnelCommittedWriteSurvivesRealRenewalFailureQ19(t *testing.T) {
	for _, mode := range []string{"revoked", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			f := setupWeb(t)
			ctx := context.Background()
			d, blocker, pid := lockedDepartment(t, f.fixture)
			queryVersion := baselineVersion(t, f, "")
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				done <- f.request("PUT", "/api/v1/personnel/departments/"+d.ID, `{"name":"committed","version":1,"queryVersion":"`+queryVersion+`"}`, true, true)
			}()
			awaitBlockedBy(t, f.fixture, pid)
			if mode == "revoked" {
				if _, err := f.store.Revoke(ctx, f.sid); err != nil {
					t.Fatal("real Redis revoke failed")
				}
			} else {
				if err := f.store.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := blocker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var w *httptest.ResponseRecorder
			select {
			case w = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("write did not finish")
			}
			if w.Code != 200 {
				t.Fatalf("committed department must return success after renewal failure, got %d", w.Code)
			}
			cleared := 0
			for _, cookie := range w.Result().Cookies() {
				if cookie.Name == session.SessionCookieName || cookie.Name == session.CSRFCookieName {
					if cookie.MaxAge >= 0 || cookie.Value != "" {
						t.Fatal("committed success must clear failed session cookies")
					}
					cleared++
				}
			}
			if cleared != 2 {
				t.Fatal("both cookies must be cleared")
			}
			var name string
			var version int64
			var count int
			if err := f.owner.QueryRow(ctx, "SELECT name,version FROM personnel.departments WHERE id=$1", d.ID).Scan(&name, &version); err != nil {
				t.Fatal(err)
			}
			if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events WHERE object_id=$1 AND reason_code='DEPARTMENT_UPDATED'", d.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if name != "committed" || version != 2 || count != 1 {
				t.Fatal("business and audit must commit exactly once")
			}
		})
	}
}
func rootActor(t *testing.T, f *fixture) session.Principal {
	t.Helper()
	p := f.actor
	ctx := context.Background()
	err := f.owner.QueryRow(ctx, "SELECT id::text,auth_version::text FROM auth.users WHERE is_bootstrap_admin").Scan(&p.UserID, &p.Record.AuthVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err = f.owner.Exec(ctx, "UPDATE auth.users SET is_bootstrap_admin=true WHERE id=$1", p.UserID); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestPersonnelWriteFirstProtectsAuthorizationUntilCommitQ25(t *testing.T) {
	f := setup(t)
	r := setup(t)
	root := rootActor(t, r)
	ctx := context.Background()
	d, blocker, pid := lockedDepartment(t, f)
	written := make(chan error, 1)
	go func() {
		_, err := f.app.SaveDepartment(ctx, f.actor, d.ID, DepartmentInput{Name: "in-flight", Version: 1}, RequestMetadata{RequestID: "11111111111111111111111111111111"})
		written <- err
	}()
	awaitBlockedBy(t, f, pid)
	revoked := make(chan error, 1)
	go func() {
		_, err := r.app.SaveDefinition(ctx, root, Template, f.template, DefinitionInput{Name: "T", Version: 1, PermissionCodes: []string{"app.test.B"}, TemplateIDs: []string{}}, RequestMetadata{RequestID: "11111111111111111111111111111111"})
		revoked <- err
	}()
	// Observe the revoker waiting on the writer's real dependency lock.
	var writerPID int32
	if err := f.owner.QueryRow(ctx, "SELECT pid FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) LIMIT 1", pid).Scan(&writerPID); err != nil {
		t.Fatal(err)
	}
	awaitBlockedBy(t, f, writerPID)
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-written:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("authorized write did not complete")
	}
	select {
	case err := <-revoked:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoker did not complete after write")
	}
	if _, err := f.app.SaveDepartment(ctx, f.actor, d.ID, DepartmentInput{Name: "denied", Version: 2}, RequestMetadata{RequestID: "11111111111111111111111111111111"}); !errors.Is(err, ErrDenied) {
		t.Fatal("a new write after committed revoke must deny")
	}
	var name string
	if err := f.owner.QueryRow(ctx, "SELECT name FROM personnel.departments WHERE id=$1", d.ID).Scan(&name); err != nil || name != "in-flight" {
		t.Fatal("revoke must not undo the already authorized write or permit a later one")
	}
}

func TestPersonnelRevocationFirstRejectsBusinessAndAuditQ25(t *testing.T) {
	f := setup(t)
	r := setup(t)
	root := rootActor(t, r)
	ctx := context.Background()
	d, err := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "unchanged", ParentID: rootDepartment(t, f)}, RequestMetadata{RequestID: "revoke-first-create"})
	if err != nil {
		t.Fatal(err)
	}
	cleanDepartment(t, f, d.ID)
	_, err = r.app.SaveDefinition(ctx, root, Template, f.template, DefinitionInput{Name: "T", Version: 1, PermissionCodes: []string{"app.test.B"}, TemplateIDs: []string{}}, RequestMetadata{RequestID: "revoke-first-template"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.app.SaveDepartment(ctx, f.actor, d.ID, DepartmentInput{Name: "forbidden", Version: 1}, RequestMetadata{RequestID: "revoke-first-denied"}); !errors.Is(err, ErrDenied) {
		t.Fatal("committed revocation must deny the actual business write")
	}
	var name string
	var count int
	if err = f.owner.QueryRow(ctx, "SELECT name FROM personnel.departments WHERE id=$1", d.ID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if err = f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events WHERE object_id=$1 AND reason_code='DEPARTMENT_UPDATED'", d.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if name != "unchanged" || count != 0 {
		t.Fatal("denied write must not change business or append a success audit")
	}
}
func TestConcurrentFirstMemberEditUsesOneVersionRowQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	target := newTarget(t, f)
	start := make(chan struct{})
	done := make(chan error, 2)
	var count int
	if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.member_configuration WHERE user_id=$1", target).Scan(&count); err != nil || count != 0 {
		t.Fatal("fixture must begin without a member configuration")
	}
	for _, id := range []string{f.i1, f.i2} {
		go func(identity string) {
			<-start
			_, err := f.app.SetMemberIdentities(ctx, f.actor, target, []string{identity}, 0, RequestMetadata{RequestID: "concurrent-first-member"})
			done <- err
		}(id)
	}
	close(start)
	success, conflicts := 0, 0
	for range 2 {
		select {
		case err := <-done:
			if err == nil {
				success++
			} else if errors.Is(err, ErrConflict) {
				conflicts++
			} else {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent first edits did not finish")
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("exactly one first edit commits, stale peer conflicts")
	}
	var version int64
	if err := f.owner.QueryRow(ctx, "SELECT version FROM personnel.member_configuration WHERE user_id=$1", target).Scan(&version); err != nil || version != 1 {
		t.Fatal("one initial version row, one committed increment")
	}
}
