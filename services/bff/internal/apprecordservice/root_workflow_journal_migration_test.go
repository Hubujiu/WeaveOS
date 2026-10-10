package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const journalNewColumns = "table_id,view_id,record_id,flow_id,version_id,definition_version,task_id,task_epoch,node_id,target_node_id,flow_name,flow_name_source"

// Actual migration text, with only schema qualification redirected to a private
// old-shape clone. The shared migrated database is never rolled backwards.
func journalMigrationFixture(t *testing.T, f rootTaskFixture, withEvents bool) (*pgx.Conn, string, string, string) {
	t.Helper()
	c, err := pgx.Connect(f.ctx, os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	schema := pgx.Identifier{"journal_migration_" + strings.ReplaceAll(recordOperationID(t, f.recordFixture), "-", "")}.Sanitize()
	if _, err = c.Exec(f.ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	for _, table := range []string{"logical_tables", "workflow_definitions", "workflow_versions", "workflow_instances", "workflow_commands", "workflow_tasks", "workflow_execution_events"} {
		if _, err = c.Exec(f.ctx, "CREATE TABLE "+schema+"."+table+" (LIKE applications."+table+" INCLUDING ALL)"); err != nil {
			t.Fatal(err)
		}
		if table == "workflow_execution_events" && !withEvents {
			continue
		}
		where := "app_id=$1"
		if table == "workflow_commands" {
			where = "command_json->>'AppID'=$1"
		}
		if _, err = c.Exec(f.ctx, "INSERT INTO "+schema+"."+table+" SELECT * FROM applications."+table+" WHERE "+where, f.app); err != nil {
			t.Fatal(err)
		}
	}
	for _, column := range strings.Split(journalNewColumns, ",") {
		if _, err = c.Exec(f.ctx, "ALTER TABLE "+schema+".workflow_execution_events DROP COLUMN "+column); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = c.Exec(f.ctx, "ALTER TABLE "+schema+".workflow_execution_events ADD CONSTRAINT workflow_execution_events_app_id_instance_id_fkey FOREIGN KEY(app_id,instance_id) REFERENCES "+schema+".workflow_instances(app_id,id) ON DELETE RESTRICT"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../../db/migrations/00027_workflow_independent_journal.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("expected exact Up and Down")
	}
	return c, schema, strings.ReplaceAll(parts[0], "applications.", schema+"."), strings.ReplaceAll(parts[1], "applications.", schema+".")
}
func journalRows(t *testing.T, ctx context.Context, c *pgx.Conn, schema string, newColumns bool) []byte {
	t.Helper()
	subtract := ""
	if newColumns {
		subtract = " - ARRAY['" + strings.ReplaceAll(journalNewColumns, ",", "','") + "']"
	}
	var raw []byte
	if err := c.QueryRow(ctx, "SELECT coalesce(jsonb_agg(to_jsonb(e)"+subtract+" ORDER BY command_id),'[]'::jsonb) FROM "+schema+".workflow_execution_events e").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestRootWorkflowJournalMigrationBackfillPreservesOriginalBytes(t *testing.T) {
	f := rootTaskSetup(t, true)
	q := eventConfirmed(t, f, "reject")
	eventSQL(t, f, "UPDATE applications.workflow_definitions SET name='Last known at migration' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	c, schema, up, down := journalMigrationFixture(t, f, true)
	before := journalRows(t, f.ctx, c, schema, false)
	if err := rootLifecycleMigrationApply(f.ctx, c, up); err != nil {
		t.Fatal(err)
	}
	after := journalRows(t, f.ctx, c, schema, true)
	var a, b any
	if json.Unmarshal(before, &a) != nil || json.Unmarshal(after, &b) != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("migration changed original protocol bytes/identity/time")
	}
	var name, source, node, version string
	if err := c.QueryRow(f.ctx, "SELECT flow_name,flow_name_source,node_id::text,version_id::text FROM "+schema+".workflow_execution_events WHERE command_id=$1", q.EventID).Scan(&name, &source, &node, &version); err != nil {
		t.Fatal(err)
	}
	if name != "Last known at migration" || source != "legacy_last_known" || node != f.first || version != f.version {
		t.Fatal("backfill invented historical name or lost original binding")
	}
	err := rootLifecycleMigrationApply(f.ctx, c, down)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "55000" {
		t.Fatalf("nonempty Down must fail safely: %v", err)
	}
	if !reflect.DeepEqual(after, journalRows(t, f.ctx, c, schema, true)) {
		t.Fatal("failed Down changed original events")
	}
}
func TestRootWorkflowJournalMigrationWrongBindingRollsBackWholeExpansion(t *testing.T) {
	f := rootTaskSetup(t, true)
	q := eventConfirmed(t, f, "reject")
	c, schema, up, _ := journalMigrationFixture(t, f, true)
	if _, err := c.Exec(f.ctx, "UPDATE "+schema+".workflow_commands SET command_json=jsonb_set(command_json,'{ActorID}',to_jsonb($2::text)) WHERE command_id=$1", q.EventID, f.other); err != nil {
		t.Fatal(err)
	}
	before := journalRows(t, f.ctx, c, schema, false)
	err := rootLifecycleMigrationApply(f.ctx, c, up)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23514" {
		t.Fatalf("mismatched old binding must reject whole migration: %v", err)
	}
	var exists bool
	if err = c.QueryRow(f.ctx, "SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=($1||'.workflow_execution_events')::regclass AND attname='table_id' AND NOT attisdropped)", schema).Scan(&exists); err != nil || exists {
		t.Fatal("failed migration left partial new columns", err)
	}
	if !reflect.DeepEqual(before, journalRows(t, f.ctx, c, schema, false)) {
		t.Fatal("bad old event was dropped or altered")
	}
}
func TestRootWorkflowJournalMigrationEmptyDownRestoresInstanceFK(t *testing.T) {
	f := rootTaskSetup(t, true)
	c, schema, up, down := journalMigrationFixture(t, f, false)
	if err := rootLifecycleMigrationApply(f.ctx, c, up); err != nil {
		t.Fatal(err)
	}
	if err := rootLifecycleMigrationApply(f.ctx, c, down); err != nil {
		t.Fatal(err)
	}
	var fk bool
	if err := c.QueryRow(f.ctx, "SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=($1||'.workflow_execution_events')::regclass AND conname='workflow_execution_events_app_id_instance_id_fkey' AND contype='f')", schema).Scan(&fk); err != nil || !fk {
		t.Fatal("empty Down did not restore old scope FK", err)
	}
	if err := rootLifecycleMigrationApply(f.ctx, c, up); err != nil {
		t.Fatal("re-expansion failed", err)
	}
}
