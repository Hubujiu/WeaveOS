package apprecordservice

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Independent oracle: V064 PRD J01/J05 and its explicit data contract require
// captured confirmation-time name, original source task and stable record scope.
// No expected value is derived from the journal's implementation.
func TestRootWorkflowJournalStoresIndependentOriginalScope(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventSQL(t, f, "UPDATE applications.workflow_definitions SET name='Original approval name' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	q := eventConfirmed(t, f, "reject")
	var table, view, record, flow, version, task, node, name, source string
	var definition, epoch int64
	var target *string
	err := f.runtime.QueryRow(f.ctx, `SELECT table_id::text,view_id::text,record_id::text,flow_id::text,version_id::text,
 definition_version,task_id::text,task_epoch,node_id::text,target_node_id::text,flow_name,flow_name_source
 FROM applications.workflow_execution_events WHERE command_id=$1`, q.EventID).Scan(&table, &view, &record, &flow, &version, &definition, &task, &epoch, &node, &target, &name, &source)
	if err != nil {
		t.Fatalf("independent durable journal scope missing: %v", err)
	}
	if table != f.table || view != f.view || record != f.ownRecord || flow != f.head.FlowID || version != f.version || definition != 1 || task != f.task.ID || epoch != 1 || node != f.first || target != nil || name != "Original approval name" || source != "captured" {
		t.Fatalf("wrong original journal identity/name: %s %s %s %s %s %d %s %d %s %v %s %s", table, view, record, flow, version, definition, task, epoch, node, target, name, source)
	}
}

func TestRootWorkflowJournalNameIsConfirmationSnapshot(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.reference)
	eventSQL(t, f, "UPDATE applications.workflow_definitions SET name='Captured before rename' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	q := eventConfirmed(t, f, "reject")
	eventSQL(t, f, "UPDATE applications.workflow_definitions SET name='Renamed after confirmation' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	got := eventRead(t, f, f.principal, q)
	raw, err := json.Marshal(got.Event)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 14 || string(fields["flowName"]) != `"Captured before rename"` || string(fields["flowNameSource"]) != `"captured"` {
		t.Fatalf("journal must expose exact 14-field original-name DTO: %s", raw)
	}
}

func TestRootWorkflowJournalSurvivesTerminalProjectionRemoval(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.reference)
	q := eventConfirmed(t, f, "reject")
	before := eventRead(t, f, f.principal, q)
	// A separate workflow is intentionally left active, proving fixture cleanup
	// cannot hide a broad or cross-flow deletion behind a passing target read.
	other := rootCatalogReady(t, f.recordFixture, rootCatalogGraph(t, f.recordFixture, false))
	otherInstance := rootCatalogReserve(t, f.recordFixture, rootCatalogReserveInput(t, f.recordFixture, other))
	var commandsBefore int
	if err := f.owner.QueryRow(f.ctx, `SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1`, f.app).Scan(&commandsBefore); err != nil {
		t.Fatal(err)
	}
	tx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, "DELETE FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", f.app, f.instance.ID); err != nil {
		t.Fatal(err)
	}
	tag, err := tx.Exec(f.ctx, "DELETE FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 AND state='rejected'", f.app, f.instance.ID)
	if err != nil {
		t.Fatalf("terminal query projection still owns durable event lifecycle: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatal("target terminal fixture was not removed")
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	// The original graph is deliberately no longer a usable executable graph.
	// This is isolated owner fixture data, never an application mutation path.
	eventSQL(t, f, "UPDATE applications.workflow_versions SET graph_json='{}',bpmn_xml='<removed-test-fixture/>' WHERE app_id=$1 AND flow_id=$2", f.app, f.head.FlowID)
	got := eventRead(t, f, f.principal, q)
	if !reflect.DeepEqual(got, before) {
		t.Fatal("independent history changed after query projection removal")
	}
	list, err := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, WorkflowEventHistoryRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range list.Items {
		if event.ID == q.EventID {
			found = true
			if !reflect.DeepEqual(event, before.Event) {
				t.Fatal("list/detail disagree after projection removal")
			}
		}
	}
	if !found {
		t.Fatal("confirmed target disappeared from independent history list")
	}
	var remaining, commandsAfter int
	if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1 AND id=$2", f.app, otherInstance.ID).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal("unrelated instance removed", err)
	}
	if err = f.owner.QueryRow(f.ctx, `SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1`, f.app).Scan(&commandsAfter); err != nil || commandsAfter != commandsBefore {
		t.Fatal("original command ledger changed", err)
	}
}
