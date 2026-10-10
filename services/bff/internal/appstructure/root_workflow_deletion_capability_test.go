package appstructure

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestRootDeletionCleanupCapabilityRequiresBoundConfirmedLease(t *testing.T) {
	for _, kind := range []string{"valid", "wrong-app", "wrong-operation", "wrong-lease", "expired", "unconfirmed"} {
		t.Run(kind, func(t *testing.T) {
			p := deletionFixture(t)
			ctx := context.Background()
			in := deletionInput(t, p)
			acceptDeletion(t, p, in)
			lease := uuid(t, p.s.f.owner)
			if _, e := p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET lease_token=$2,lease_until=clock_timestamp()+interval '45 seconds' WHERE flow_id=$1", p.flow, lease); e != nil {
				t.Fatal(e)
			}
			if kind != "unconfirmed" {
				if _, e := p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET engine_deleted_versions=0,engine_deleted_at='2026-10-01T00:00:00Z' WHERE flow_id=$1", p.flow); e != nil {
					t.Fatal(e)
				}
			}
			app, op, token := in.AppID, in.OperationID, lease
			switch kind {
			case "wrong-app":
				app = uuid(t, p.s.f.owner)
			case "wrong-operation":
				op = uuid(t, p.s.f.owner)
			case "wrong-lease":
				token = uuid(t, p.s.f.owner)
			case "expired":
				if _, e := p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET lease_until=clock_timestamp()-interval '1 second' WHERE flow_id=$1", p.flow); e != nil {
					t.Fatal(e)
				}
			}
			var completed bool
			e := p.s.f.runtime.QueryRow(ctx, "SELECT applications.complete_workflow_deletion($1,$2,$3,$4)", app, p.flow, op, token).Scan(&completed)
			switch kind {
			case "valid":
				if e != nil || !completed {
					t.Fatal("confirmed original lease cannot complete", completed, e)
				}
			case "wrong-app", "wrong-operation", "unconfirmed":
				var pg *pgconn.PgError
				want := "23503"
				if kind == "unconfirmed" {
					want = "55000"
				}
				if !errors.As(e, &pg) || pg.Code != want {
					t.Fatal("unbound completion capability not rejected", kind, e)
				}
			default:
				if e != nil || completed {
					t.Fatal("expired/replaced lease performed cleanup", kind, completed, e)
				}
			}
			if kind != "valid" {
				if s, n := deletionState(t, p); s != "pending" || n != 1 {
					t.Fatal("rejected capability mutated catalog", s, n)
				}
			}
		})
	}
}
func TestRootDeletionCleanupLeastPrivilegeAndFixedSearchPath(t *testing.T) {
	p := deletionFixture(t)
	ctx := context.Background()
	var definer, fixed, public bool
	if e := p.s.f.owner.QueryRow(ctx, `SELECT prosecdef,proconfig=ARRAY['search_path=pg_catalog'],EXISTS(SELECT 1 FROM aclexplode(COALESCE(proacl,acldefault('f',proowner))) WHERE grantee=0 AND privilege_type='EXECUTE') FROM pg_proc WHERE oid='applications.complete_workflow_deletion(uuid,uuid,uuid,uuid)'::regprocedure`).Scan(&definer, &fixed, &public); e != nil || !definer || !fixed || public {
		t.Fatal("cleanup function security boundary invalid", definer, fixed, public, e)
	}
	for _, role := range []string{"auth_app", "auth_reader", "auth_maintenance", "auth_backup"} {
		var allowed bool
		if e := p.s.f.owner.QueryRow(ctx, "SELECT has_function_privilege($1,'applications.complete_workflow_deletion(uuid,uuid,uuid,uuid)','EXECUTE')", role).Scan(&allowed); e != nil || allowed != (role == "auth_app") {
			t.Fatal("cleanup execute privilege drift", role, allowed, e)
		}
	}
	for _, table := range []string{"workflow_definitions", "workflow_versions", "workflow_instances", "workflow_tasks", "workflow_deletions"} {
		var broad bool
		if e := p.s.f.owner.QueryRow(ctx, "SELECT has_table_privilege('auth_app',$1,'DELETE,TRUNCATE')", "applications."+table).Scan(&broad); e != nil || broad {
			t.Fatal("runtime acquired broad destructive privilege", table, broad, e)
		}
	}
}
