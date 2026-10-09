package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestRootPublicationHistoryConcurrentInsertAndCleanup(t *testing.T) {
	for _, insertFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "insert-first", false: "delete-first"}[insertFirst], func(t *testing.T) {
			p := rootPublicationSetup(t)
			original := uuid(t, p.s.f.owner)
			p.publish(t, original, 1)
			p.dispatch(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			writer, err := p.s.f.runtime.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback(context.Background())
			cleaner, err := p.s.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer cleaner.Rollback(context.Background())
			var writerPID, cleanerPID int
			if err = writer.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&writerPID); err != nil {
				t.Fatal(err)
			}
			if err = cleaner.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&cleanerPID); err != nil {
				t.Fatal(err)
			}
			operation := uuid(t, p.s.f.owner)
			insert := func() error {
				_, e := writer.Exec(ctx, `INSERT INTO applications.workflow_publications
     (actor_user_id,operation_id,app_id,view_id,flow_id,version,version_id,bpmn_sha256,actor_auth_version,accepted_close_epoch,expected_revision,expected_schema_version,fingerprint)
     SELECT actor_user_id,$3,app_id,view_id,flow_id,version,version_id,bpmn_sha256,actor_auth_version,accepted_close_epoch,expected_revision,expected_schema_version,fingerprint
     FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2`, p.s.f.actor, original, operation)
				return e
			}
			remove := func() error {
				_, e := cleaner.Exec(ctx, "DELETE FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2", p.s.f.app, p.flow)
				return e
			}
			blocker, waiter := writerPID, cleanerPID
			done := make(chan error, 1)
			if insertFirst {
				if err = insert(); err != nil {
					t.Fatal(err)
				}
				go func() { done <- remove() }()
			} else {
				if err = remove(); err != nil {
					t.Fatal(err)
				}
				blocker, waiter = cleanerPID, writerPID
				go func() { done <- insert() }()
			}
			// Observe an actual PostgreSQL wait edge, not merely elapsed time.
			for {
				var blocked bool
				if err = p.s.f.owner.QueryRow(ctx, "SELECT $1=ANY(pg_blocking_pids($2))", blocker, waiter).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case err = <-done:
					t.Fatalf("concurrent operation did not wait: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(5 * time.Millisecond):
				}
			}
			want := "23503"
			if insertFirst {
				err = writer.Commit(ctx)
				want = "55000"
			} else {
				err = cleaner.Commit(ctx)
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != want {
				t.Fatalf("fresh post-wait snapshot expected %s: %v", want, err)
			}
			// Release the failed transaction before assertions or fixture teardown.
			_ = writer.Rollback(ctx)
			_ = cleaner.Rollback(ctx)
			var intents, versions int
			if err = p.s.f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2", p.s.f.actor, operation).Scan(&intents); err != nil {
				t.Fatal(err)
			}
			if err = p.s.f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2", p.s.f.app, p.flow).Scan(&versions); err != nil {
				t.Fatal(err)
			}
			expected := 0
			if insertFirst {
				expected = 1
			}
			if intents != expected || versions != expected {
				t.Fatalf("orphaned intent or unsafe cleanup: intents=%d versions=%d", intents, versions)
			}
		})
	}
}

// Rehearse the actual migration in a private old-shape schema; only qualified
// schema names are redirected. No shared migrated database is rolled back.
func publicationMigrationFixture(t *testing.T, p rootPublicationFixture, populated bool) (*pgx.Conn, string, string, string) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	schema := pgx.Identifier{"publication_migration_" + strings.ReplaceAll(uuid(t, p.s.f.owner), "-", "")}.Sanitize()
	if _, err = c.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, e := c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	})
	for _, table := range []string{"apps", "workflow_definitions", "workflow_versions", "workflow_instances", "workflow_commands", "workflow_publications", "workflow_engine_receipts", "workflow_deployments"} {
		if _, err = c.Exec(ctx, "CREATE TABLE "+schema+"."+table+" (LIKE applications."+table+" INCLUDING ALL)"); err != nil {
			t.Fatal(err)
		}
		if !populated && (table == "workflow_publications" || table == "workflow_engine_receipts" || table == "workflow_deployments") {
			continue
		}
		where := "app_id=$1"
		if table == "apps" {
			where = "id=$1"
		}
		if table == "workflow_commands" {
			where = "command_json->>'AppID'=$1"
		}
		if _, err = c.Exec(ctx, "INSERT INTO "+schema+"."+table+" SELECT * FROM applications."+table+" WHERE "+where, p.s.f.app); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		"ALTER TABLE %s.workflow_publications ADD CONSTRAINT workflow_publications_app_id_flow_id_view_id_fkey FOREIGN KEY(app_id,flow_id,view_id) REFERENCES %s.workflow_definitions(app_id,id,view_id) ON DELETE RESTRICT",
		"ALTER TABLE %s.workflow_publications ADD CONSTRAINT workflow_publications_app_id_flow_id_version_version_id_fkey FOREIGN KEY(app_id,flow_id,version,version_id) REFERENCES %s.workflow_versions(app_id,flow_id,version,version_id) ON DELETE RESTRICT",
		"ALTER TABLE %s.workflow_engine_receipts ADD CONSTRAINT workflow_engine_receipts_app_id_flow_id_version_version_id_fkey FOREIGN KEY(app_id,flow_id,version,version_id) REFERENCES %s.workflow_versions(app_id,flow_id,version,version_id) ON DELETE RESTRICT",
		"ALTER TABLE %s.workflow_deployments ADD CONSTRAINT workflow_deployments_app_id_flow_id_version_fkey FOREIGN KEY(app_id,flow_id,version) REFERENCES %s.workflow_versions(app_id,flow_id,version) ON DELETE RESTRICT",
	} {
		if _, err = c.Exec(ctx, strings.ReplaceAll(q, "%s", schema)); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile("../../../../db/migrations/00028_workflow_publication_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("exact Up/Down required")
	}
	return c, schema, strings.ReplaceAll(parts[0], "applications.", schema+"."), strings.ReplaceAll(parts[1], "applications.", schema+".")
}
func publicationMigrationApply(c *pgx.Conn, sql string) error {
	ctx := context.Background()
	tx, e := c.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, sql); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func publicationCloneRows(t *testing.T, c *pgx.Conn, schema string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"workflow_publications", "workflow_engine_receipts", "workflow_deployments"} {
		var raw string
		if e := c.QueryRow(context.Background(), "SELECT coalesce(jsonb_agg(to_jsonb(h) ORDER BY to_jsonb(h)::text),'[]'::jsonb)::text FROM "+schema+"."+table+" h").Scan(&raw); e != nil {
			t.Fatal(e)
		}
		out[table] = raw
	}
	return out
}
func TestRootPublicationHistoryMigrationPreservesBytesAndRefusesNonemptyDown(t *testing.T) {
	p := rootPublicationSetup(t)
	p.publish(t, uuid(t, p.s.f.owner), 1)
	p.dispatch(t)
	c, schema, up, down := publicationMigrationFixture(t, p, true)
	before := publicationCloneRows(t, c, schema)
	if err := publicationMigrationApply(c, up); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, publicationCloneRows(t, c, schema)) {
		t.Fatal("migration rewrote original history")
	}
	err := publicationMigrationApply(c, down)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "55000" {
		t.Fatalf("nonempty Down not refused: %v", err)
	}
	if !reflect.DeepEqual(before, publicationCloneRows(t, c, schema)) {
		t.Fatal("refused Down changed history")
	}
}
func TestRootPublicationHistoryMigrationEmptyDownAndReexpand(t *testing.T) {
	p := rootPublicationSetup(t)
	c, schema, up, down := publicationMigrationFixture(t, p, false)
	if err := publicationMigrationApply(c, up); err != nil {
		t.Fatal(err)
	}
	if err := publicationMigrationApply(c, down); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := c.QueryRow(context.Background(), "SELECT count(*) FROM pg_constraint WHERE connamespace=$1::regnamespace AND contype='f'", strings.Trim(schema, "\"")).Scan(&n); err != nil || n != 4 {
		t.Fatal("original four FKs not restored", n, err)
	}
	if err := publicationMigrationApply(c, up); err != nil {
		t.Fatal("reexpand", err)
	}
}
func TestRootPublicationHistoryMigrationRejectsBadOriginalBindingAtomically(t *testing.T) {
	p := rootPublicationSetup(t)
	p.publish(t, uuid(t, p.s.f.owner), 1)
	p.dispatch(t)
	c, schema, up, _ := publicationMigrationFixture(t, p, true)
	ctx := context.Background()
	if _, err := c.Exec(ctx, "ALTER TABLE "+schema+".workflow_engine_receipts DROP CONSTRAINT workflow_engine_receipts_app_id_flow_id_version_version_id_fkey"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, "UPDATE "+schema+".workflow_engine_receipts SET version_id=$1", uuid(t, p.s.f.owner)); err != nil {
		t.Fatal(err)
	}
	before := publicationCloneRows(t, c, schema)
	err := publicationMigrationApply(c, up)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23503" {
		t.Fatalf("bad history not rejected: %v", err)
	}
	if !reflect.DeepEqual(before, publicationCloneRows(t, c, schema)) {
		t.Fatal("bad history repaired or deleted")
	}
	var exists bool
	if err = c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_proc WHERE pronamespace=$1::regnamespace AND proname='bind_publication_history_insert')", strings.Trim(schema, "\"")).Scan(&exists); err != nil || exists {
		t.Fatal("failed migration left trigger function", err)
	}
}
