package personnel

import (
	"context"
	"testing"
)

func TestQ36BusinessRevisionLockPrecedesAuthorization(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	parent := rootDepartment(t, f)
	blocker, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = blocker.Exec(ctx, "SELECT revision FROM personnel.query_revisions WHERE scope='people' FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	var pid int32
	if err = blocker.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		d   Department
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		d, e := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "revision-first", ParentID: parent}, RequestMetadata{RequestID: "q36-lock-order"})
		done <- outcome{d, e}
	}()
	defer func() {
		_ = blocker.Rollback(ctx)
		r := <-done
		if r.err != nil {
			t.Error(r.err)
		}
		if r.d.ID != "" {
			_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.departments WHERE id=$1", r.d.ID)
		}
	}()
	awaitBlockedBy(t, f, pid)
	probe, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Rollback(ctx)
	if _, err = probe.Exec(ctx, "SELECT id FROM auth.users WHERE id=$1 FOR UPDATE NOWAIT", f.actor.UserID); err != nil {
		t.Fatalf("writer locked actor before revision, reversing approved lock order: %v", err)
	}
}
