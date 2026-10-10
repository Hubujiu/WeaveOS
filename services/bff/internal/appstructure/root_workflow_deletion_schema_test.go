package appstructure

import (
	"context"
	"errors"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestRootDeletionSchemaAndRuntimeBoundary(t *testing.T) {
	p := deletionFixture(t)
	ctx := context.Background()
	var present bool
	if e := p.s.f.owner.QueryRow(ctx, "SELECT to_regclass('applications.workflow_deletions') IS NOT NULL").Scan(&present); e != nil {
		t.Fatal(e)
	}
	if !present {
		t.Fatal("durable deletion identity schema is absent")
	}
	in := deletionInput(t, p)
	acceptDeletion(t, p, in)
	for _, q := range []string{
		"UPDATE applications.workflow_deletions SET operation_id=gen_random_uuid() WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET flow_name='changed' WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET fingerprint=decode(repeat('00',32),'hex') WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET expected_revision=expected_revision+1 WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET created_at=created_at+interval '1 second' WHERE flow_id=$1",
	} {
		_, e := p.s.f.owner.Exec(ctx, q, p.flow)
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "55000" {
			t.Fatalf("accepted identity mutable: %s: %v", q, e)
		}
	}
	for _, q := range []string{
		"UPDATE applications.workflow_deletions SET status='invented' WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET status='deleted',completed_at=clock_timestamp() WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET lease_token=gen_random_uuid() WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET engine_deleted_versions=0 WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET attempts=-1 WHERE flow_id=$1",
		"UPDATE applications.workflow_deletions SET completed_at=clock_timestamp() WHERE flow_id=$1",
	} {
		_, e := p.s.f.owner.Exec(ctx, q, p.flow)
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "23514" {
			t.Fatalf("invalid persisted deletion state accepted: %s: %v", q, e)
		}
	}
	for _, q := range []string{
		"UPDATE applications.workflow_deletions SET flow_name='changed' WHERE flow_id=$1",
		"DELETE FROM applications.workflow_deletions WHERE flow_id=$1",
	} {
		_, e := p.s.f.runtime.Exec(ctx, q, p.flow)
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "42501" {
			t.Fatalf("runtime deletion identity privilege too broad: %s: %v", q, e)
		}
	}
	var backup bool
	if e := p.s.f.owner.QueryRow(ctx, "SELECT has_table_privilege('auth_backup','applications.workflow_deletions','SELECT')").Scan(&backup); e != nil || !backup {
		t.Fatal("deletion identity missing from backup boundary", e)
	}
	_, e := p.s.f.runtime.Exec(ctx, "UPDATE applications.workflow_deletions SET status='unknown',reason='engine_unavailable',attempts=attempts+1,updated_at=clock_timestamp() WHERE flow_id=$1", p.flow)
	if e != nil {
		t.Fatal("runtime cannot persist uncertain original operation", e)
	}
	_, e = p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET engine_deleted_versions=0,engine_deleted_at=clock_timestamp() WHERE flow_id=$1", p.flow)
	if e != nil {
		t.Fatal(e)
	}
	_, e = p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET engine_deleted_versions=1 WHERE flow_id=$1", p.flow)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatal("confirmed engine receipt was overwritten", e)
	}
	_, e = p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET status='deleted',reason=NULL,completed_at=clock_timestamp() WHERE flow_id=$1", p.flow)
	if e != nil {
		t.Fatal(e)
	}
	_, e = p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET updated_at=clock_timestamp() WHERE flow_id=$1", p.flow)
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatal("completed deletion was mutable", e)
	}
}
func TestRootDeletionCatalogEnableRejectsAfterCloseFinalization(t *testing.T) {
	p := deletionFixture(t)
	p.publish(t, uuid(t, p.s.f.owner), 1)
	p.dispatch(t)
	in := deletionInput(t, p)
	acceptDeletion(t, p, in)
	ctx := context.Background()
	p.s.tx(t, func(tx pgx.Tx) error {
		h, e := (wc.Catalog{}).GetInTx(ctx, tx, in.AppID, in.FlowID)
		if e != nil {
			return e
		}
		_, e = (wc.Catalog{}).FinalizeCloseInTx(ctx, tx, in.AppID, in.FlowID, h.Revision)
		return e
	})
	tx, e := p.s.f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	h, e := (wc.Catalog{}).GetInTx(ctx, tx, in.AppID, in.FlowID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = (wc.Catalog{}).EnableInTx(ctx, tx, in.AppID, in.FlowID, h.Revision)
	if !errors.Is(e, wc.ErrClosing) {
		t.Fatalf("Go catalog re-enabled permanent deletion identity: %v", e)
	}
}
