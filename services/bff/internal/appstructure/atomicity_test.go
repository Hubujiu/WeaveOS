package appstructure

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
)

func failTrigger(t *testing.T, f *fixture, table, event, condition string) {
	t.Helper()
	suffix := strings.ReplaceAll(uuid(t, f.owner), "-", "")
	function := pgx.Identifier{"public", "v013_fail_" + suffix}.Sanitize()
	trigger := pgx.Identifier{"v013_fail_" + suffix}.Sanitize()
	sql := fmt.Sprintf("CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF %s THEN RAISE EXCEPTION 'isolated injected failure'; END IF; RETURN NEW;END$$;CREATE TRIGGER %s BEFORE %s ON %s FOR EACH ROW EXECUTE FUNCTION %s()", function, condition, trigger, event, table, function)
	if _, e := f.owner.Exec(context.Background(), sql); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		f.owner.Exec(context.Background(), "DROP TRIGGER "+trigger+" ON "+table)
		f.owner.Exec(context.Background(), "DROP FUNCTION "+function+"()")
	})
}
func TestSaveMetadataLayoutAuditAndResultFaultsRollbackOneRealTransaction(t *testing.T) {
	for _, phase := range []string{"metadata", "layout", "audit", "result"} {
		t.Run(phase, func(t *testing.T) {
			f := setup(t)
			view, table := newForm(t, f)
			target, event, condition := "applications.logical_tables", "UPDATE", "NEW.id='"+table+"'::uuid"
			switch phase {
			case "layout":
				target = "applications.form_views"
				condition = "NEW.id='" + view + "'::uuid"
			case "audit":
				target, event = "auth.authentication_events", "INSERT"
				condition = "NEW.actor_user_id='" + f.actor + "'::uuid AND NEW.event_type='application_structure_changed'"
			case "result":
				target = "applications.operations"
				condition = "NEW.app_id='" + f.app + "'::uuid AND NEW.operation_kind='definition.save'"
			}
			failTrigger(t, f, target, event, condition)
			text := field(t, f, "text", "literal", map[string]any{"maxLength": nil})
			in := input(t, f, 0, 0, text)
			expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 503, "COMMON_SERVICE_UNAVAILABLE")
			var exists, ready bool
			var schema, layout, fields, ledger, audits int
			e := f.owner.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL,t.schema_ready,t.schema_version,v.view_version,(SELECT count(*) FROM applications.fields WHERE table_id=t.id),(SELECT count(*) FROM applications.operations WHERE operation_id=$3),(SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$3::text) FROM applications.logical_tables t JOIN applications.form_views v ON v.table_id=t.id WHERE t.id=$2", "appdata.t_"+strings.ReplaceAll(table, "-", ""), table, in["operationId"]).Scan(&exists, &ready, &schema, &layout, &fields, &ledger, &audits)
			if e != nil || exists || ready || schema != 0 || layout != 0 || fields != 0 || ledger != 0 || audits != 0 {
				t.Fatalf("fault rollback phase %s %v %v %d %d %d %d %d %v", phase, exists, ready, schema, layout, fields, ledger, audits, e)
			}
		})
	}
}
func TestAtomicNewFormFailureLeavesNoTableOrView(t *testing.T) {
	f := setup(t)
	failTrigger(t, f, "applications.form_views", "INSERT", "NEW.app_id='"+f.app+"'::uuid")
	in := map[string]any{"operationId": uuid(t, f.owner), "name": "failed", "source": map[string]any{"kind": "new_table"}, "directoryId": nil, "position": 0, "expectedStructureVersion": 0}
	expectError(t, f, "POST", "/forms", in, 503, "COMMON_SERVICE_UNAVAILABLE")
	var tables, views, ops, version int
	e := f.owner.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM applications.logical_tables WHERE app_id=$1),(SELECT count(*) FROM applications.form_views WHERE app_id=$1),(SELECT count(*) FROM applications.operations WHERE app_id=$1),structure_version FROM applications.apps WHERE id=$1", f.app).Scan(&tables, &views, &ops, &version)
	if e != nil || tables+views+ops+version != 0 {
		t.Fatal("orphan after atomic create failure", tables, views, ops, version, e)
	}
}
