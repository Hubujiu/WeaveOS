package appstructure

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"strings"
	"testing"
)

func TestRootDeletionSchemaRejectsUnboundIntent(t *testing.T) {
	for _, kind := range []string{"not-closing", "wrong-app", "wrong-view", "wrong-name", "wrong-revision"} {
		t.Run(kind, func(t *testing.T) {
			p := deletionFixture(t)
			other := deletionFixture(t)
			ctx := context.Background()
			app, table, view, actor := p.s.f.app, p.s.table, p.s.view, p.s.f.actor
			var name string
			if e := p.s.f.owner.QueryRow(ctx, "SELECT name FROM applications.workflow_definitions WHERE id=$1", p.flow).Scan(&name); e != nil {
				t.Fatal(e)
			}
			rev := int64(1)
			want := "23503"
			if kind != "not-closing" {
				if _, e := p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_definitions SET state='closing',revision=revision+1 WHERE id=$1", p.flow); e != nil {
					t.Fatal(e)
				}
			}
			switch kind {
			case "not-closing":
				want = "55000"
			case "wrong-app":
				app, table, view, actor = other.s.f.app, other.s.table, other.s.view, other.s.f.actor
			case "wrong-view":
				view = other.s.view
			case "wrong-name":
				name = "invented"
			case "wrong-revision":
				rev = 2
			}
			_, e := p.s.f.runtime.Exec(ctx, `INSERT INTO applications.workflow_deletions(flow_id,app_id,table_id,view_id,actor_user_id,operation_id,flow_name,expected_revision,fingerprint) VALUES($1,$2,$3,$4,$5,$6,$7,$8,decode(repeat('00',32),'hex'))`, p.flow, app, table, view, actor, uuid(t, p.s.f.owner), name, rev)
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != want {
				t.Fatalf("unbound %s deletion intent persisted: want %s got %v", kind, want, e)
			}
		})
	}
}
func TestRootDeletionNonemptyDownRefusesIdentityLoss(t *testing.T) {
	p := deletionFixture(t)
	acceptDeletion(t, p, deletionInput(t, p))
	raw, e := os.ReadFile("../../../../db/migrations/00029_workflow_deletions.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("missing explicit deletion migration Down")
	}
	ctx := context.Background()
	tx, e := p.s.f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, parts[1])
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatalf("Down erased accepted deletion identity: %v", e)
	}
}
