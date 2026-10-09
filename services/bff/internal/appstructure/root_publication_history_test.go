package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func publicationHistoryRows(t *testing.T, p rootPublicationFixture) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"workflow_publications", "workflow_engine_receipts", "workflow_deployments"} {
		var raw string
		if err := p.s.f.owner.QueryRow(context.Background(), "SELECT coalesce(jsonb_agg(to_jsonb(h) ORDER BY to_jsonb(h)::text),'[]'::jsonb)::text FROM applications."+table+" h WHERE app_id=$1 AND flow_id=$2", p.s.f.app, p.flow).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		out[table] = raw
	}
	return out
}
func TestRootPublicationHistorySurvivesCatalogRemoval(t *testing.T) {
	p := rootPublicationSetup(t)
	ctx := context.Background()
	id := uuid(t, p.s.f.owner)
	p.publish(t, id, 1)
	p.dispatch(t)
	beforeResult := p.result(t, id)
	if beforeResult["status"] != "confirmed" {
		t.Fatal("fixture not terminal")
	}
	other := uuid(t, p.s.f.owner)
	data(t, rootWorkflowHTTPCall(t, p.s, other, "PUT", "definition", rootWorkflowHTTPBody(t, p.s, p.s.f.actor)), 201)
	before := publicationHistoryRows(t, p)
	for _, table := range []string{"workflow_publications", "workflow_engine_receipts", "workflow_deployments"} {
		if before[table] == "[]" {
			t.Fatal("missing original history fixture", table)
		}
	}
	var auditBefore int
	if err := p.s.f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'flowId'=$1", p.flow).Scan(&auditBefore); err != nil {
		t.Fatal(err)
	}
	tx, err := p.s.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2", p.s.f.app, p.flow); err != nil {
		t.Fatalf("immutable publication history still owns full configuration lifecycle: %v", err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", p.s.f.app, p.flow); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, publicationHistoryRows(t, p)) {
		t.Fatal("catalog cleanup deleted or rewrote publication history")
	}
	if !reflect.DeepEqual(beforeResult, p.result(t, id)) {
		t.Fatal("original publication status lost after catalog cleanup")
	}
	raw, _ := json.Marshal(map[string]any{"operationId": id, "expectedRevision": 1, "expectedSchemaVersion": 1})
	response := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", rootWorkflowHTTPPath(p.s, p.flow, "publish"), string(raw), true, true, "", nil)
	if !reflect.DeepEqual(beforeResult, data(t, response, 200)) {
		t.Fatal("terminal replay changed its original result")
	}
	changed, _ := json.Marshal(map[string]any{"operationId": id, "expectedRevision": 2, "expectedSchemaVersion": 1})
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", rootWorkflowHTTPPath(p.s, p.flow, "publish"), string(changed), true, true, "", nil), 409, "APPLICATION_OPERATION_CONFLICT")
	fresh, _ := json.Marshal(map[string]any{"operationId": uuid(t, p.s.f.owner), "expectedRevision": 1, "expectedSchemaVersion": 1})
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", rootWorkflowHTTPPath(p.s, p.flow, "publish"), string(fresh), true, true, "", nil), 404, "APPLICATION_NOT_FOUND")
	unauth := rootWorkflowHTTPRequest(t, p.s, p.handler, "GET", rootWorkflowHTTPPath(p.s, p.flow, "publications/"+id), "", false, false, "", nil)
	if unauth.Code != 401 {
		t.Fatal("historical publication bypassed current session", unauth.Code)
	}
	var auditAfter, otherCount, removed int
	if err = p.s.f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'flowId'=$1", p.flow).Scan(&auditAfter); err != nil || auditAfter != auditBefore {
		t.Fatal("replay added audit", err)
	}
	if err = p.s.f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", p.s.f.app, other).Scan(&otherCount); err != nil || otherCount != 1 {
		t.Fatal("other flow removed", err)
	}
	if err = p.s.f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", p.s.f.app, p.flow).Scan(&removed); err != nil || removed != 0 {
		t.Fatal("replay reconstructed deleted flow", err)
	}
	if len(p.peer.requests) != 1 {
		t.Fatal("history replay called deployment again")
	}
}

func TestRootPublicationHistoryPendingAndUnknownBlockCatalogCleanup(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "unknown"}[unknown], func(t *testing.T) {
			p := rootPublicationSetup(t)
			ctx := context.Background()
			id := uuid(t, p.s.f.owner)
			p.publish(t, id, 1)
			if unknown {
				if _, err := p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_publications SET status='unknown',reason='DEPENDENCY_UNAVAILABLE' WHERE actor_user_id=$1 AND operation_id=$2", p.s.f.actor, id); err != nil {
					t.Fatal(err)
				}
			}
			for _, q := range []string{"DELETE FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2", "DELETE FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2"} {
				_, err := p.s.f.owner.Exec(ctx, q, p.s.f.app, p.flow)
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != "55000" {
					t.Fatalf("explicit undrained catalog guard missing: %v", err)
				}
			}
		})
	}
}

func TestRootPublicationHistoryRejectsStaleWriteSnapshots(t *testing.T) {
	for _, iso := range []pgx.TxIsoLevel{pgx.RepeatableRead, pgx.Serializable} {
		for _, action := range []string{"insert", "version-delete", "definition-delete"} {
			t.Run(string(iso)+"/"+action, func(t *testing.T) {
				p := rootPublicationSetup(t)
				ctx := context.Background()
				if action == "definition-delete" {
					if _, err := p.s.f.owner.Exec(ctx, "DELETE FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2", p.s.f.app, p.flow); err != nil {
						t.Fatal(err)
					}
				}
				tx, err := p.s.f.owner.BeginTx(ctx, pgx.TxOptions{IsoLevel: iso})
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				q := "INSERT INTO applications.workflow_deployments(app_id,flow_id,version,deployment_id) VALUES($1,$2,1,'isolated-snapshot-test')"
				if action == "version-delete" {
					q = "DELETE FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2"
				}
				if action == "definition-delete" {
					q = "DELETE FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2"
				}
				_, err = tx.Exec(ctx, q, p.s.f.app, p.flow)
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != "25001" {
					t.Fatalf("stale write snapshot was not explicitly rejected: %v", err)
				}
			})
		}
	}
}
