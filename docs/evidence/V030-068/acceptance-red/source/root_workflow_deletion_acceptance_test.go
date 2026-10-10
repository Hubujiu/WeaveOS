package appstructure

import (
	"context"
	"errors"
	"reflect"
	"testing"

	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func deletionInput(t *testing.T, p rootPublicationFixture) wc.DeletionInput {
	t.Helper()
	var rev int64
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT revision FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", p.s.f.app, p.flow).Scan(&rev); e != nil {
		t.Fatal(e)
	}
	return wc.DeletionInput{AppID: p.s.f.app, ViewID: p.s.view, FlowID: p.flow, ActorID: p.s.f.actor, OperationID: uuid(t, p.s.f.owner), ExpectedRevision: rev}
}
func acceptDeletion(t *testing.T, p rootPublicationFixture, in wc.DeletionInput) wc.Deletion {
	t.Helper()
	var result wc.Deletion
	p.s.tx(t, func(tx pgx.Tx) error {
		var e error
		result, e = (wc.Catalog{}).RequestDeletionInTx(context.Background(), tx, in)
		return e
	})
	return result
}
func deletionCallError(t *testing.T, p rootPublicationFixture, in wc.DeletionInput) error {
	t.Helper()
	ctx := context.Background()
	tx, e := p.s.f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	_, e = (wc.Catalog{}).RequestDeletionInTx(ctx, tx, in)
	return e
}
func TestRootDeletionAcceptanceClosesWithoutDeletingConfiguration(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "unpublished-disabled", true: "published-enabled"}[enabled], func(t *testing.T) {
			p := rootPublicationSetup(t)
			if enabled {
				p.publish(t, uuid(t, p.s.f.owner), 1)
				p.dispatch(t)
				p.s.tx(t, func(tx pgx.Tx) error {
					h, e := (wc.Catalog{}).GetInTx(context.Background(), tx, p.s.f.app, p.flow)
					if e != nil {
						return e
					}
					_, e = (wc.Catalog{}).EnableInTx(context.Background(), tx, p.s.f.app, p.flow, h.Revision)
					return e
				})
			}
			in := deletionInput(t, p)
			var epoch int64
			var name string
			if e := p.s.f.owner.QueryRow(context.Background(), "SELECT close_epoch,name FROM applications.workflow_definitions WHERE id=$1", p.flow).Scan(&epoch, &name); e != nil {
				t.Fatal(e)
			}
			got := acceptDeletion(t, p, in)
			if got.Status != "pending" || got.FlowID != p.flow || got.OperationID != in.OperationID || got.ActorID != in.ActorID || got.TableID != p.s.table || got.FlowName != name || got.DeletedAt != nil || got.DeletedVersions != nil || got.CompletedAt != nil {
				t.Fatalf("acceptance fabricated completion or lost binding: %+v", got)
			}
			var state string
			var rev, afterEpoch int64
			if e := p.s.f.owner.QueryRow(context.Background(), "SELECT state,revision,close_epoch FROM applications.workflow_definitions WHERE id=$1", p.flow).Scan(&state, &rev, &afterEpoch); e != nil || state != "closing" || rev != in.ExpectedRevision+1 || afterEpoch != epoch+1 {
				t.Fatalf("acceptance did not atomically stop new work: %s %d %d %v", state, rev, afterEpoch, e)
			}
			var versions int
			if e := p.s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.workflow_versions WHERE flow_id=$1", p.flow).Scan(&versions); e != nil || versions != 1 {
				t.Fatal("acceptance prematurely removed runnable config", e, versions)
			}
			if !reflect.DeepEqual(acceptDeletion(t, p, in), got) {
				t.Fatal("same original request lost exact pending receipt")
			}
			var n int
			if e := p.s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.workflow_deletions WHERE flow_id=$1", p.flow).Scan(&n); e != nil || n != 1 {
				t.Fatal("duplicate durable deletion", e, n)
			}
		})
	}
}
func TestRootDeletionAcceptanceRejectsChangedRequestAndScope(t *testing.T) {
	p := rootPublicationSetup(t)
	in := deletionInput(t, p)
	acceptDeletion(t, p, in)
	for _, kind := range []string{"revision", "operation", "view", "actor"} {
		t.Run(kind, func(t *testing.T) {
			q := in
			switch kind {
			case "revision":
				q.ExpectedRevision++
			case "operation":
				q.OperationID = uuid(t, p.s.f.owner)
			case "view":
				q.ViewID = uuid(t, p.s.f.owner)
			case "actor":
				q.ActorID = uuid(t, p.s.f.owner)
			}
			if e := deletionCallError(t, p, q); !errors.Is(e, wc.ErrConflict) {
				t.Fatalf("changed accepted binding was not rejected: %v", e)
			}
		})
	}
}
func TestRootDeletionAcceptanceRollbackPreservesOriginalDefinition(t *testing.T) {
	p := rootPublicationSetup(t)
	in := deletionInput(t, p)
	ctx := context.Background()
	tx, e := p.s.f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = (wc.Catalog{}).RequestDeletionInTx(ctx, tx, in); e != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("deletion acceptance missing: %v", e)
	}
	if e = tx.Rollback(ctx); e != nil {
		t.Fatal(e)
	}
	var state string
	var revision int64
	var count int
	if e = p.s.f.owner.QueryRow(ctx, "SELECT state,revision FROM applications.workflow_definitions WHERE id=$1", p.flow).Scan(&state, &revision); e != nil || state != "disabled" || revision != in.ExpectedRevision {
		t.Fatal("rollback changed definition", state, revision, e)
	}
	if e = p.s.f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.workflow_deletions WHERE flow_id=$1", p.flow).Scan(&count); e != nil || count != 0 {
		t.Fatal("rollback retained accepted intent", count, e)
	}
	acceptDeletion(t, p, in)
}
func TestRootDeletionAcceptanceRejectsStaleRevisionAndWrongViewWithoutMutation(t *testing.T) {
	for _, kind := range []string{"stale-revision", "wrong-view", "invalid-operation"} {
		t.Run(kind, func(t *testing.T) {
			p := rootPublicationSetup(t)
			in := deletionInput(t, p)
			expected := wc.ErrConflict
			switch kind {
			case "stale-revision":
				in.ExpectedRevision++
			case "wrong-view":
				in.ViewID = uuid(t, p.s.f.owner)
				expected = wc.ErrMissing
			case "invalid-operation":
				in.OperationID = "00000000-0000-0000-0000-000000000000"
				expected = wc.ErrInvalid
			}
			if e := deletionCallError(t, p, in); !errors.Is(e, expected) {
				t.Fatalf("wrong rejection: %v want %v", e, expected)
			}
			var state string
			if e := p.s.f.owner.QueryRow(context.Background(), "SELECT state FROM applications.workflow_definitions WHERE id=$1", p.flow).Scan(&state); e != nil || state != "disabled" {
				t.Fatal("rejected request mutated definition", state, e)
			}
		})
	}
}
func TestRootDeletionIdentityGuardsConfigurationAndAllowsCloseCompletion(t *testing.T) {
	p := rootPublicationSetup(t)
	p.publish(t, uuid(t, p.s.f.owner), 1)
	p.dispatch(t)
	in := deletionInput(t, p)
	acceptDeletion(t, p, in)
	ctx := context.Background()
	for _, q := range []string{
		"UPDATE applications.workflow_definitions SET state='enabled' WHERE id=$1",
		"UPDATE applications.workflow_definitions SET name='resurrection' WHERE id=$1",
		"INSERT INTO applications.workflow_versions(app_id,flow_id,version,version_id,schema_version,graph_json,bpmn_xml,allow_withdraw,created_by,triggers_json) SELECT app_id,flow_id,version+1,gen_random_uuid(),schema_version,graph_json,bpmn_xml,allow_withdraw,created_by,triggers_json FROM applications.workflow_versions WHERE flow_id=$1 LIMIT 1",
	} {
		_, e := p.s.f.owner.Exec(ctx, q, p.flow)
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "55000" {
			t.Fatalf("deletion identity configuration barrier absent: %v", e)
		}
	}
	p.s.tx(t, func(tx pgx.Tx) error {
		h, e := (wc.Catalog{}).GetInTx(ctx, tx, in.AppID, in.FlowID)
		if e != nil {
			return e
		}
		_, e = (wc.Catalog{}).FinalizeCloseInTx(ctx, tx, in.AppID, in.FlowID, h.Revision)
		return e
	})
	// Explicit owner-only synthetic catalog removal, not the product cleanup API.
	for _, q := range []string{"DELETE FROM applications.workflow_versions WHERE flow_id=$1", "DELETE FROM applications.workflow_definitions WHERE id=$1"} {
		if _, e := p.s.f.owner.Exec(ctx, q, p.flow); e != nil {
			t.Fatal(e)
		}
	}
	other := rootPublicationSetup(t)
	_, e := other.s.f.owner.Exec(ctx, "INSERT INTO applications.workflow_definitions(id,app_id,table_id,view_id,name,revision,state,candidate_version) VALUES($1,$2,$3,$4,'New name',1,'disabled',1)", p.flow, other.s.f.app, other.s.table, other.s.view)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatalf("deleted global flow UUID resurrected across app: %v", e)
	}
	replay := acceptDeletion(t, p, in)
	if replay.Status != "pending" || replay.OperationID != in.OperationID {
		t.Fatal("missing catalog changed original accepted operation")
	}
}
