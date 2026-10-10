package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowprojection"
	"github.com/jackc/pgx/v5/pgconn"
	"reflect"
	"strings"
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

func TestRootWorkflowJournalListDoesNotLoseRemovedProjection(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.reference)
	q := eventConfirmed(t, f, "reject")
	eventSQL(t, f, "DELETE FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", f.app, f.instance.ID)
	eventSQL(t, f, "DELETE FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 AND state='rejected'", f.app, f.instance.ID)
	got, err := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, WorkflowEventHistoryRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range got.Items {
		if v.ID == q.EventID {
			return
		}
	}
	t.Fatalf("confirmed event silently lost after projection removal: %+v", got)
}

func TestRootWorkflowJournalConfirmedReplayWithoutProjection(t *testing.T) {
	f := rootTaskSetup(t, true)
	op := rootAction(t, f, rootActionRequest(t, f, "reject"))
	c, p := rootActionRead(t, f, op)
	r, body := f.receipt(t, c, "rejected", "")
	f.apply(t, c, p, r, body, true)
	eventSQL(t, f, "DELETE FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", f.app, f.instance.ID)
	eventSQL(t, f, "DELETE FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 AND state='rejected'", f.app, f.instance.ID)
	eventSQL(t, f, "UPDATE applications.workflow_versions SET graph_json='{}',bpmn_xml='<removed-test-fixture/>' WHERE app_id=$1 AND flow_id=$2", f.app, f.head.FlowID)
	got := f.apply(t, c, p, r, body, true)
	if !got.Duplicate || got.Entry.Command != c || got.Entry.Receipt == nil || *got.Entry.Receipt != r {
		t.Fatal("did not return original confirmed receipt")
	}
	var events, instances int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events WHERE command_id=$1", c.CommandID).Scan(&events); err != nil || events != 1 {
		t.Fatal("replay created/replaced event", err)
	}
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1 AND id=$2", f.app, f.instance.ID).Scan(&instances); err != nil || instances != 0 {
		t.Fatal("replay recreated execution instance", err)
	}
}

func TestRootWorkflowJournalDirectRecordAccessPath(t *testing.T) {
	f := rootTaskSetup(t, false)
	for i := 0; i < 200; i++ {
		historyNoEffect(t, f)
	}
	noise := rootTaskSetup(t, false)
	for i := 0; i < 400; i++ {
		historyNoEffect(t, noise)
	}
	eventSQL(t, f, "ANALYZE applications.workflow_execution_events")
	var plan []byte
	if err := f.owner.QueryRow(f.ctx, `EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON)
 SELECT command_id FROM applications.workflow_execution_events
 WHERE app_id=$1 AND table_id=$2 AND record_id=$3
 ORDER BY created_at DESC,command_id DESC LIMIT 21`, f.app, f.table, f.ownRecord).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	t.Logf("V064_DIRECT_HISTORY_PLAN targetConfirmed=201 otherConfirmed=401 bounded21 localFixture=%s", plan)
	var definition string
	var valid, ready bool
	if err := f.owner.QueryRow(f.ctx, `SELECT pg_get_indexdef(indexrelid),indisvalid,indisready FROM pg_index
 WHERE indexrelid=to_regclass('applications.ix_workflow_events_record_history')`).Scan(&definition, &valid, &ready); err != nil {
		t.Fatalf("independent record-keyset access path missing: %v", err)
	}
	if !valid || !ready || !strings.Contains(definition, "(app_id, table_id, record_id, created_at DESC, command_id DESC)") {
		t.Fatal("wrong independent history access path", definition)
	}
}

func TestRootWorkflowJournalReplayPreservesNewFenceAndRejectsTampering(t *testing.T) {
	f := rootTaskSetup(t, true)
	op := rootAction(t, f, rootActionRequest(t, f, "reject"))
	c, p := rootActionRead(t, f, op)
	r, body := f.receipt(t, c, "rejected", "")
	f.apply(t, c, p, r, body, true)
	next := f.rootProjection
	next.instance = rootCatalogReserve(t, f.recordFixture, rootCatalogReserveInput(t, f.recordFixture, f.head))
	pending, _ := next.accept(t, "start", "", "", 0, 0)
	eventSQL(t, f, "DELETE FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", f.app, f.instance.ID)
	eventSQL(t, f, "DELETE FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 AND state='rejected'", f.app, f.instance.ID)
	if !f.apply(t, c, p, r, body, true).Duplicate {
		t.Fatal("original replay lost duplicate identity")
	}
	checkFence := func() {
		t.Helper()
		var id, state string
		var epoch int64
		if err := f.owner.QueryRow(f.ctx, "SELECT command_id::text,state,fence_epoch FROM applications.record_command_fences WHERE app_id=$1 AND table_id=$2 AND record_id=$3", f.app, f.table, f.ownRecord).Scan(&id, &state, &epoch); err != nil || id != pending.CommandID || state != "pending" || epoch != pending.FenceEpoch {
			t.Fatal("old replay changed new command fence", id, state, epoch, err)
		}
	}
	checkFence()
	changed := r
	changed.ProofID = recordOperationID(t, f.recordFixture)
	tx, err := f.runtime.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	got, err := (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, changed, body)
	if err == nil || !reflect.DeepEqual(got, workflowprojection.Applied{}) {
		t.Fatal("changed receipt passed terminal shortcut", err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	checkFence()
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET record_id=$2 WHERE command_id=$1", c.CommandID, recordOperationID(t, f.recordFixture))
	tx, err = f.runtime.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	got, err = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, body)
	if err == nil || !reflect.DeepEqual(got, workflowprojection.Applied{}) {
		t.Fatal("wrong durable record binding passed terminal shortcut", err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	checkFence()
}

func TestRootWorkflowJournalAppendOnlyRolesAndCallerMetadata(t *testing.T) {
	f := rootTaskSetup(t, true)
	q := eventConfirmed(t, f, "reject")
	for _, sql := range []string{"UPDATE applications.workflow_execution_events SET flow_name='forged' WHERE command_id=$1", "DELETE FROM applications.workflow_execution_events WHERE command_id=$1"} {
		_, err := f.runtime.Exec(f.ctx, sql, q.EventID)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "42501" {
			t.Fatalf("runtime must not change permanent journal: %v", err)
		}
	}
	_, err := f.runtime.Exec(f.ctx, `INSERT INTO applications.workflow_execution_events
 (command_id,app_id,instance_id,actor_id,action,outcome,sequence,schema_version,record_version,evidence_hash,payload_bytes,result_bytes,proof_id,flow_name)
 SELECT command_id,app_id,instance_id,actor_id,action,outcome,sequence,schema_version,record_version,evidence_hash,payload_bytes,result_bytes,proof_id,'forged'
 FROM applications.workflow_execution_events WHERE command_id=$1`, q.EventID)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23514" {
		t.Fatalf("conflicting caller snapshot must reject before insertion: %v", err)
	}
	tx, err := f.owner.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, "SET LOCAL ROLE auth_backup"); err != nil {
		t.Fatal(err)
	}
	var record, name, source string
	if err = tx.QueryRow(f.ctx, "SELECT record_id::text,flow_name,flow_name_source FROM applications.workflow_execution_events WHERE command_id=$1", q.EventID).Scan(&record, &name, &source); err != nil || record != f.ownRecord || name == "" || name == "forged" || source != "captured" {
		t.Fatal("backup cannot read unchanged new journal metadata", err)
	}
}

func TestRootWorkflowJournalNoEffectUnknownTaskKeepsNullSource(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	missing := recordOperationID(t, f.recordFixture)
	c, p := f.accept(t, "agree", missing, "", 1, 1)
	r, body := f.receipt(t, c, "unchanged", "task_inactive")
	f.apply(t, c, p, r, body, true)
	q := WorkflowEventRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, EventID: c.CommandID}
	got := eventRead(t, f, f.principal, q)
	if got.Event.NodeID != nil || got.Event.Outcome != "no_effect" || got.Event.FlowNameSource != "captured" {
		t.Fatal("unknown original task was fabricated", got.Event)
	}
	var task string
	var node *string
	if err := f.runtime.QueryRow(f.ctx, "SELECT task_id::text,node_id::text FROM applications.workflow_execution_events WHERE command_id=$1", c.CommandID).Scan(&task, &node); err != nil || task != missing || node != nil {
		t.Fatal("lost original stable task identity", err)
	}
}

func TestRootWorkflowJournalWrongExistingTaskRollsBackWholeProjection(t *testing.T) {
	f := rootTaskSetup(t, false)
	foreign := rootTaskSetup(t, false)
	c, p := f.accept(t, "agree", foreign.task.ID, "", 1, 1)
	r, body := f.receipt(t, c, "unchanged", "task_inactive")
	tx, err := f.runtime.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	got, err := (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, body)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23514" || !reflect.DeepEqual(got, workflowprojection.Applied{}) {
		t.Fatalf("existing wrong-scope task must reject journal atomically: %v", err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	var state, owner string
	var events int
	if err = f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_commands WHERE command_id=$1", c.CommandID).Scan(&state); err != nil || state != "pending" {
		t.Fatal("failed projection confirmed ledger", err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events WHERE command_id=$1", c.CommandID).Scan(&events); err != nil || events != 0 {
		t.Fatal("failed projection retained partial event", err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT command_id::text FROM applications.record_command_fences WHERE app_id=$1 AND table_id=$2 AND record_id=$3 AND state='pending'", f.app, f.table, f.ownRecord).Scan(&owner); err != nil || owner != c.CommandID {
		t.Fatal("failed projection released original fence", err)
	}
}
