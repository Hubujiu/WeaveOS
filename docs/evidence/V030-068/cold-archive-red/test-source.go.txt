package appstructure

import (
	"context"
	auditmaintenance "github.com/Hubujiu/WeaveOS/services/bff/internal/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestRootDeletionCompletionArchivesWithOriginalBytes(t *testing.T) {
	p := deletionFixture(t)
	acceptDeletion(t, p, deletionInput(t, p))
	dispatchDeletion(t, deletionWorker(p, &deletionPeer{}))
	ctx := context.Background()
	if _, e := p.s.f.owner.Exec(ctx, "UPDATE auth.authentication_events SET occurred_at=date_trunc('month',now())-interval '1 day' WHERE reason_code='WORKFLOW_DELETION_COMPLETED' AND change_summary->>'flowId'=$1", p.flow); e != nil {
		t.Fatal(e)
	}
	var id, original string
	if e := p.s.f.owner.QueryRow(ctx, "SELECT id::text,change_summary::text FROM auth.authentication_events WHERE reason_code='WORKFLOW_DELETION_COMPLETED' AND change_summary->>'flowId'=$1", p.flow).Scan(&id, &original); e != nil {
		t.Fatal(e)
	}
	pool := func(dsn string) *pgxpool.Pool {
		cfg, e := pgxpool.ParseConfig(dsn)
		if e != nil {
			t.Fatal(e)
		}
		cfg.AfterConnect = func(c context.Context, x *pgx.Conn) error { _, e := x.Exec(c, "SET ROLE auth_maintenance"); return e }
		db, e := pgxpool.NewWithConfig(ctx, cfg)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(db.Close)
		return db
	}
	hot, cold := pool(os.Getenv("WEAVEOS_TEST_DATABASE_URL")), pool(os.Getenv("WEAVEOS_TEST_ARCHIVE_DATABASE_URL"))
	result, e := auditmaintenance.Maintain(ctx, hot, cold, time.Now())
	if e != nil || result.Archived < 1 {
		t.Fatal("deletion completion cannot reach durable cold history", result, e)
	}
	var archived string
	if e := cold.QueryRow(ctx, "SELECT change_summary::text FROM archive.authentication_events WHERE id=$1", id).Scan(&archived); e != nil || archived != original {
		t.Fatal("cold completion changed original bytes", e)
	}
	var remains bool
	if e := hot.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM auth.authentication_events WHERE id=$1)", id).Scan(&remains); e != nil || remains {
		t.Fatal("durably archived completion remained hot", e)
	}
}
