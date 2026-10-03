package appstructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgconn"
)

// Oracle: lead decision 2026-10-03: newly assigned members must be active;
// pre-existing inactive IDs may be retained or explicitly removed.
func TestMemberAssignmentRejectsNewInactiveAndPreservesExistingInactive(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	var group, member string
	if e := f.owner.QueryRow(ctx, "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'membership decision') RETURNING id::text", f.app).Scan(&group); e != nil {
		t.Fatal(e)
	}
	if e := f.owner.QueryRow(ctx, "INSERT INTO auth.users(account,status) VALUES('member-decision-'||gen_random_uuid(),'disabled') RETURNING id::text").Scan(&member); e != nil {
		t.Fatal(e)
	}
	a := &applications.Application{Pool: f.runtime}
	p := session.Principal{UserID: f.actor, Record: session.Record{AuthVersion: "1"}}
	op := uuid(t, f.owner)
	in := applications.Input{OperationID: op, ExpectedPolicyRevision: 1, MemberIDs: []string{member}}
	if _, e := a.Write(ctx, p, "members.replace", f.app, group, in, applications.Metadata{RequestID: "new-inactive-member"}); !errors.Is(e, applications.ErrResourceInvalid) {
		t.Fatalf("new inactive membership must reject, got %v", e)
	}
	var count, revision int
	if e := f.owner.QueryRow(ctx, "SELECT policy_revision,(SELECT count(*) FROM applications.operations WHERE actor_user_id=$2 AND operation_id=$3) FROM applications.apps WHERE id=$1", f.app, f.actor, op).Scan(&revision, &count); e != nil || revision != 1 || count != 0 {
		t.Fatalf("rejection must not persist policy or operation: %d %d %v", revision, count, e)
	}
	if _, e := f.owner.Exec(ctx, "UPDATE auth.users SET status='active' WHERE id=$1", member); e != nil {
		t.Fatal(e)
	}
	if out, e := a.Write(ctx, p, "members.replace", f.app, group, in, applications.Metadata{RequestID: "active-admission"}); e != nil || out.Status != 200 {
		t.Fatal(out, e)
	}
	if _, e := f.owner.Exec(ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", member); e != nil {
		t.Fatal(e)
	}
	// Confirmed replay is independent of new-selection validation and old CAS.
	if out, e := a.Write(ctx, p, "members.replace", f.app, group, in, applications.Metadata{RequestID: "admission-replay"}); e != nil || out.Status != 200 {
		t.Fatal(out, e)
	}
	in.OperationID, in.ExpectedPolicyRevision = uuid(t, f.owner), 2
	if out, e := a.Write(ctx, p, "members.replace", f.app, group, in, applications.Metadata{RequestID: "retain-disabled"}); e != nil || out.Status != 200 {
		t.Fatal(out, e)
	}
	in.OperationID, in.ExpectedPolicyRevision, in.MemberIDs = uuid(t, f.owner), 3, []string{}
	if out, e := a.Write(ctx, p, "members.replace", f.app, group, in, applications.Metadata{RequestID: "remove-disabled"}); e != nil || out.Status != 200 {
		t.Fatal(out, e)
	}
	if e := f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.group_members WHERE app_id=$1 AND group_id=$2", f.app, group).Scan(&count); e != nil || count != 0 {
		t.Fatal("inactive member was not explicitly removed", count, e)
	}
}

func TestMemberAssignmentLocksSourceStatusBeforeAppGate(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var group, member string
	if e := f.owner.QueryRow(ctx, "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'status race') RETURNING id::text", f.app).Scan(&group); e != nil {
		t.Fatal(e)
	}
	if e := f.owner.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('member-race-'||gen_random_uuid()) RETURNING id::text").Scan(&member); e != nil {
		t.Fatal(e)
	}
	blocker, e := f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer blocker.Rollback(context.Background())
	var blockerPID int
	if e := blocker.QueryRow(ctx, "SELECT pg_backend_pid() FROM applications.apps WHERE id=$1 FOR UPDATE", f.app).Scan(&blockerPID); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	op := uuid(t, f.owner)
	go func() {
		_, e := (&applications.Application{Pool: f.runtime}).Write(ctx, session.Principal{UserID: f.actor, Record: session.Record{AuthVersion: "1"}}, "members.replace", f.app, group, applications.Input{OperationID: op, ExpectedPolicyRevision: 1, MemberIDs: []string{member}}, applications.Metadata{RequestID: "source-status-race"})
		done <- e
	}()
	wait := time.NewTicker(5 * time.Millisecond)
	defer wait.Stop()
	for {
		var waiting bool
		if e := f.owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", blockerPID).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("writer never reached actual app gate", ctx.Err())
		case e := <-done:
			t.Fatal("writer completed before app gate release", e)
		case <-wait.C:
		}
	}
	status, e := f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer status.Rollback(context.Background())
	if _, e := status.Exec(ctx, "SET LOCAL lock_timeout='100ms'"); e != nil {
		t.Fatal(e)
	}
	// A direct row-lock probe distinguishes the required source lock from the
	// existing global query-revision trigger that also serializes status writes.
	var locked string
	probeErr := status.QueryRow(ctx, "SELECT id::text FROM auth.users WHERE id=$1 FOR UPDATE NOWAIT", member).Scan(&locked)
	status.Rollback(context.Background())
	blocker.Rollback(context.Background())
	if e := <-done; e != nil {
		t.Fatal("member admission must commit before later status update", e)
	}
	var pgerr *pgconn.PgError
	if !errors.As(probeErr, &pgerr) || pgerr.Code != "55P03" {
		t.Fatalf("actual source row must be locked before app gate, got %v", probeErr)
	}
	if _, e := f.owner.Exec(ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", member); e != nil {
		t.Fatal("status change must succeed after admission commit", e)
	}
}
