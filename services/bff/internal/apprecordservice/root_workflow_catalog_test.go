package apprecordservice

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

func rootCatalogGraph(t *testing.T, f recordFixture, editable bool) flowgraph.Graph {
	t.Helper()
	start, approve, end := recordOperationID(t, f), recordOperationID(t, f), recordOperationID(t, f)
	fields := []string{}
	if editable {
		fields = append(fields, f.public)
	}
	return flowgraph.Graph{Version: 1, Nodes: []flowgraph.Node{
		{ID: start, Kind: "start"},
		{ID: approve, Kind: "approval", Approval: &flowgraph.Approval{Mode: "all", AssigneeIDs: []string{f.actor}, EditableFieldIDs: fields}},
		{ID: end, Kind: "end"},
	}, Edges: []flowgraph.Edge{{From: start, To: approve}, {From: approve, To: end}}}
}
func rootCatalogCondition(t *testing.T, f recordFixture) flowgraph.Graph {
	g := rootCatalogGraph(t, f, true)
	cond, otherEnd := recordOperationID(t, f), recordOperationID(t, f)
	raw, _ := json.Marshal(map[string]any{"operator": "and", "children": []any{map[string]any{"fieldId": f.public, "operator": "eq", "value": "alpha"}}})
	g.Nodes = append(g.Nodes, flowgraph.Node{ID: cond, Kind: "condition", Condition: raw}, flowgraph.Node{ID: otherEnd, Kind: "end"})
	g.Edges[1].To = cond
	g.Edges = append(g.Edges, flowgraph.Edge{From: cond, To: g.Nodes[2].ID, Branch: "true"}, flowgraph.Edge{From: cond, To: otherEnd, Branch: "false"})
	return g
}
func rootCatalogTx(t *testing.T, f recordFixture, fn func(pgx.Tx) error) {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if e = fn(tx); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
}
func rootCatalogInput(f recordFixture, id string, rev int64, g flowgraph.Graph) workflowcatalog.VersionInput {
	return workflowcatalog.VersionInput{AppID: f.app, TableID: f.table, ViewID: f.view, FlowID: id, ActorID: f.other, Name: "Expense", ExpectedRevision: rev, ExpectedSchemaVersion: 1, Graph: g, AllowWithdraw: true}
}
func rootCatalogPut(t *testing.T, f recordFixture, id string, rev int64, g flowgraph.Graph) workflowcatalog.Head {
	t.Helper()
	var h workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		h, e = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, rootCatalogInput(f, id, rev, g))
		return e
	})
	return h
}
func rootCatalogDeploy(t *testing.T, f recordFixture, h workflowcatalog.Head) workflowcatalog.Head {
	t.Helper()
	var out workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		out, e = (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, h.CandidateVersion, "engine-"+recordOperationID(t, f))
		return e
	})
	return out
}
func rootCatalogEnable(t *testing.T, f recordFixture, h workflowcatalog.Head) workflowcatalog.Head {
	t.Helper()
	var out workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		out, e = (workflowcatalog.Catalog{}).EnableInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
		return e
	})
	return out
}
func rootCatalogReady(t *testing.T, f recordFixture, g flowgraph.Graph) workflowcatalog.Head {
	t.Helper()
	return rootCatalogEnable(t, f, rootCatalogDeploy(t, f, rootCatalogPut(t, f, recordOperationID(t, f), 0, g)))
}
func rootCatalogReserveInput(t *testing.T, f recordFixture, h workflowcatalog.Head) workflowcatalog.ReserveInput {
	t.Helper()
	return workflowcatalog.ReserveInput{AppID: f.app, FlowID: h.FlowID, InstanceID: recordOperationID(t, f), RecordID: f.ownRecord, ActorID: f.actor, ExpectedRevision: h.Revision, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1}
}
func rootCatalogReserve(t *testing.T, f recordFixture, in workflowcatalog.ReserveInput) workflowcatalog.Instance {
	t.Helper()
	var out workflowcatalog.Instance
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		out, e = (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, in)
		return e
	})
	return out
}
func rootCatalogWantError(t *testing.T, f recordFixture, want error, fn func(pgx.Tx) error) {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if e = fn(tx); !errors.Is(e, want) {
		t.Fatalf("want %v got %v", want, e)
	}
}
func TestRootCatalogVersionIsDurableAndCallerRollback(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, true)
	id := recordOperationID(t, f)
	h := rootCatalogPut(t, f, id, 0, g)
	if h.FlowID != id || h.Revision != 1 || h.CurrentVersion != 0 || h.CandidateVersion != 1 || h.State != "disabled" {
		t.Fatalf("new head %+v", h)
	}
	var raw []byte
	var bpmn string
	var allowed bool
	if e := f.runtime.QueryRow(f.ctx, "SELECT graph_json,bpmn_xml,allow_withdraw FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, id).Scan(&raw, &bpmn, &allowed); e != nil {
		t.Fatal(e)
	}
	var saved flowgraph.Graph
	if json.Unmarshal(raw, &saved) != nil || len(saved.Nodes) != 3 || saved.Nodes[1].ID != g.Nodes[1].ID || bpmn == "" || !allowed {
		t.Fatalf("missing immutable version %s", raw)
	}
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	rollbackID := recordOperationID(t, f)
	if _, e = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, rootCatalogInput(f, rollbackID, 0, g)); e != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	rootCatalogWantError(t, f, workflowcatalog.ErrMissing, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).GetInTx(f.ctx, tx, f.app, rollbackID)
		return e
	})
}
func TestRootCatalogUndeployedCannotEnableOrStart(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogPut(t, f, recordOperationID(t, f), 0, rootCatalogGraph(t, f, true))
	rootCatalogWantError(t, f, workflowcatalog.ErrNotReady, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).EnableInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
		return e
	})
	in := rootCatalogReserveInput(t, f, h)
	rootCatalogWantError(t, f, workflowcatalog.ErrNotReady, func(tx pgx.Tx) error { _, e := (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, in); return e })
}
func TestRootCatalogDeploymentReceiptIdempotentAndBound(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogPut(t, f, recordOperationID(t, f), 0, rootCatalogGraph(t, f, false))
	var a, b workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		a, e = (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, 1, "deployment-one")
		return e
	})
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		b, e = (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, 1, "deployment-one")
		return e
	})
	if a != b || a.CurrentVersion != 1 || a.Revision != 2 {
		t.Fatalf("receipt replay changed state %+v %+v", a, b)
	}
	rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, 1, "different")
		return e
	})
}
func TestRootCatalogOldDeploymentCannotReplaceNewCandidate(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, false)
	h := rootCatalogPut(t, f, recordOperationID(t, f), 0, g)
	h = rootCatalogPut(t, f, h.FlowID, h.Revision, g)
	var old workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		old, e = (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, 1, "old")
		return e
	})
	if old.CurrentVersion != 0 || old.CandidateVersion != 2 {
		t.Fatalf("late deployment activated wrong version %+v", old)
	}
	newer := rootCatalogDeploy(t, f, old)
	if newer.CurrentVersion != 2 {
		t.Fatalf("new candidate not activated %+v", newer)
	}
}
func TestRootCatalogInstancesKeepStructureWhileNextUsesConfirmedVersion(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, true)
	h := rootCatalogReady(t, f, g)
	in := rootCatalogReserveInput(t, f, h)
	first := rootCatalogReserve(t, f, in)
	if first.DefinitionVersion != 1 || first.State != "starting" || first.RecordID != f.ownRecord || first.Sequence != 0 {
		t.Fatalf("first %+v", first)
	}
	next := rootCatalogPut(t, f, h.FlowID, h.Revision, rootCatalogGraph(t, f, false))
	during := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, next))
	if during.DefinitionVersion != 1 {
		t.Fatalf("unconfirmed candidate exposed %+v", during)
	}
	next = rootCatalogDeploy(t, f, next)
	second := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, next))
	if second.DefinitionVersion != 2 {
		t.Fatalf("new instance did not bind latest confirmed structure %+v", second)
	}
	replay := rootCatalogReserve(t, f, in)
	if replay != first {
		t.Fatalf("instance replay changed binding %+v %+v", first, replay)
	}
	var count int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE id=$1 AND definition_version=1", first.ID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("old binding count=%d err=%v", count, e)
	}
}
func TestRootCatalogCloseWaitsForAcceptedStartAndRejectsNewStart(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	inst := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var closing workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		closing, e = (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
		return e
	})
	if closing.State != "closing" {
		t.Fatalf("starting intent omitted from drain %+v", closing)
	}
	rootCatalogWantError(t, f, workflowcatalog.ErrClosing, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, rootCatalogReserveInput(t, f, closing))
		return e
	})
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		got, e := (workflowcatalog.Catalog{}).FinalizeCloseInTx(f.ctx, tx, f.app, h.FlowID, closing.Revision)
		if e == nil && got != closing {
			t.Fatalf("premature close %+v", got)
		}
		return e
	})
	// Owner simulates a CONFIRMED application projection, not a network timeout.
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state='completed',sequence=1 WHERE id=$1", inst.ID); e != nil {
		t.Fatal(e)
	}
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		got, e := (workflowcatalog.Catalog{}).FinalizeCloseInTx(f.ctx, tx, f.app, h.FlowID, closing.Revision)
		if e == nil && (got.State != "disabled" || got.Revision != closing.Revision+1) {
			t.Fatalf("not drained %+v", got)
		}
		return e
	})
}
func TestRootCatalogCloseWithoutInstancesAndLateCloseCAS(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	var closed workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		closed, e = (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
		return e
	})
	if closed.State != "disabled" {
		t.Fatalf("empty close %+v", closed)
	}
	h = rootCatalogEnable(t, f, closed)
	rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var closing workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		closing, e = (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
		return e
	})
	opened := rootCatalogEnable(t, f, closing)
	if opened.State != "enabled" || opened.Revision <= closing.Revision {
		t.Fatal("reopen must supersede close revision")
	}
	rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).FinalizeCloseInTx(f.ctx, tx, f.app, h.FlowID, closing.Revision)
		return e
	})
}
func TestRootCatalogIndependentFlowsSameRecord(t *testing.T) {
	f := newRecordFixture(t)
	a := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	b := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	ia := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, a))
	ib := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, b))
	if ia.ID == ib.ID || ia.FlowID == ib.FlowID || ia.RecordID != ib.RecordID {
		t.Fatal("parallel flows not independent")
	}
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, a.FlowID, a.Revision)
		return e
	})
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		got, e := (workflowcatalog.Catalog{}).GetInTx(f.ctx, tx, f.app, b.FlowID)
		if e == nil && got.State != "enabled" {
			t.Fatal("A close changed B")
		}
		return e
	})
}
func TestRootCatalogCASAndResourceScope(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, false)
	h := rootCatalogReady(t, f, g)
	for _, change := range []func(*workflowcatalog.VersionInput){
		func(in *workflowcatalog.VersionInput) { in.ExpectedRevision-- },
		func(in *workflowcatalog.VersionInput) { in.ExpectedSchemaVersion = 2 },
	} {
		in := rootCatalogInput(f, h.FlowID, h.Revision, g)
		change(&in)
		rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error { _, e := (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in); return e })
	}
	foreign := newRecordFixture(t)
	bad := rootCatalogInput(f, recordOperationID(t, f), 0, g)
	bad.ViewID = foreign.view
	rootCatalogWantError(t, f, workflowcatalog.ErrMissing, func(tx pgx.Tx) error { _, e := (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, bad); return e })
	rootCatalogWantError(t, f, workflowcatalog.ErrMissing, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).GetInTx(f.ctx, tx, foreign.app, h.FlowID)
		return e
	})
	for _, change := range []func(*workflowcatalog.ReserveInput){
		func(in *workflowcatalog.ReserveInput) { in.ExpectedRecordVersion = 2 },
		func(in *workflowcatalog.ReserveInput) { in.ExpectedSchemaVersion = 2 },
		func(in *workflowcatalog.ReserveInput) { in.ExpectedRevision-- },
	} {
		in := rootCatalogReserveInput(t, f, h)
		change(&in)
		rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error { _, e := (workflowcatalog.Catalog{}).ReserveInTx(f.ctx, tx, in); return e })
	}
}
func TestRootCatalogInvalidGraphHasNoPersistentVersion(t *testing.T) {
	f := newRecordFixture(t)
	id := recordOperationID(t, f)
	g := rootCatalogGraph(t, f, false)
	g.Edges = nil
	rootCatalogWantError(t, f, workflowcatalog.ErrInvalid, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, rootCatalogInput(f, id, 0, g))
		return e
	})
	rootCatalogWantError(t, f, workflowcatalog.ErrMissing, func(tx pgx.Tx) error { _, e := (workflowcatalog.Catalog{}).GetInTx(f.ctx, tx, f.app, id); return e })
}
func rootCatalogFields(f recordFixture, kind appquery.FieldKind) []appquery.Field {
	return []appquery.Field{{ID: f.public, Kind: kind}, {ID: f.secret, Kind: appquery.Text}, {ID: f.reference, Kind: appquery.Member}}
}
func rootCatalogConflicts(t *testing.T, f recordFixture, fields []appquery.Field) []workflowcatalog.Conflict {
	t.Helper()
	var out []workflowcatalog.Conflict
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var e error
		out, e = (workflowcatalog.Catalog{}).CheckCompatibilityInTx(f.ctx, tx, f.app, f.table, fields)
		return e
	})
	return out
}
func TestRootCatalogCompatibilityIsPrecise(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogCondition(t, f)
	h := rootCatalogReady(t, f, g)
	if got := rootCatalogConflicts(t, f, rootCatalogFields(f, appquery.Multiline)); len(got) != 0 {
		t.Fatalf("compatible text equality blocked %+v", got)
	}
	conflicts := rootCatalogConflicts(t, f, rootCatalogFields(f, appquery.Boolean))
	found := false
	for _, c := range conflicts {
		if c.FlowID == h.FlowID && c.Version == 1 && c.NodeID == g.Nodes[3].ID && c.FieldID == f.public && c.Reason != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing precise incompatible condition %+v", conflicts)
	}
	removed := rootCatalogFields(f, appquery.Text)[1:]
	conflicts = rootCatalogConflicts(t, f, removed)
	seen := map[string]bool{}
	for _, c := range conflicts {
		if c.FlowID == h.FlowID && c.FieldID == f.public {
			seen[c.NodeID] = true
		}
	}
	if !seen[g.Nodes[1].ID] || !seen[g.Nodes[3].ID] {
		t.Fatalf("missing editable/condition dependencies %+v", conflicts)
	}
}
func TestRootCatalogOldInFlightVersionBlocksUntilTerminal(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, true))
	i := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	h = rootCatalogPut(t, f, h.FlowID, h.Revision, rootCatalogGraph(t, f, false))
	h = rootCatalogDeploy(t, f, h)
	fields := rootCatalogFields(f, appquery.Text)[1:]
	if got := rootCatalogConflicts(t, f, fields); len(got) == 0 || got[0].Version != 1 {
		t.Fatalf("lost in-flight old dependency %+v", got)
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state='completed',sequence=1 WHERE id=$1", i.ID); e != nil {
		t.Fatal(e)
	}
	if got := rootCatalogConflicts(t, f, fields); len(got) != 0 {
		t.Fatalf("terminal history blocks forever %+v", got)
	}
}
func TestRootCatalogDisabledHistoryDoesNotBlockSchema(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, true))
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
		return e
	})
	if got := rootCatalogConflicts(t, f, rootCatalogFields(f, appquery.Text)[1:]); len(got) != 0 {
		t.Fatalf("unused disabled history blocks %+v", got)
	}
}
func TestRootCatalogDeploymentRechecksChangedSchema(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogPut(t, f, recordOperationID(t, f), 0, rootCatalogCondition(t, f))
	// A schema Save can occur while a candidate has not been enabled or deployed.
	var fields []map[string]any
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT fields_json FROM applications.logical_tables WHERE id=$1", f.table).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(raw, &fields); e != nil {
		t.Fatal(e)
	}
	fields = fields[1:]
	raw, _ = json.Marshal(fields)
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=$2,schema_version=2 WHERE id=$1", f.table, raw); e != nil {
		t.Fatal(e)
	}
	rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, 1, "too-late")
		return e
	})
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		got, e := (workflowcatalog.Catalog{}).GetInTx(f.ctx, tx, f.app, h.FlowID)
		if e == nil && got.CurrentVersion != 0 {
			t.Fatal("incompatible deployment became current")
		}
		return e
	})
}
func TestRootCatalogConcurrentVersionCAS(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, false)
	h := rootCatalogPut(t, f, recordOperationID(t, f), 0, g)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	results := make(chan error, 2)
	for j := 0; j < 2; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				results <- e
				return
			}
			defer tx.Rollback(f.ctx)
			_, e = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, rootCatalogInput(f, h.FlowID, h.Revision, g))
			if e == nil {
				e = tx.Commit(f.ctx)
			}
			results <- e
		}()
	}
	close(gate)
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for e := range results {
		if e == nil {
			ok++
		} else if errors.Is(e, workflowcatalog.ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("concurrent publishers success=%d conflict=%d", ok, conflict)
	}
}
func TestRootCatalogImmutableVersionsAndMinimumRoles(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	for _, q := range []string{
		"UPDATE applications.workflow_versions SET graph_json=graph_json WHERE flow_id=$1",
		"UPDATE applications.workflow_versions SET bpmn_xml=bpmn_xml WHERE flow_id=$1",
		"DELETE FROM applications.workflow_versions WHERE flow_id=$1",
		"DELETE FROM applications.workflow_deployments WHERE flow_id=$1",
		"UPDATE applications.workflow_deployments SET deployment_id=deployment_id WHERE flow_id=$1",
		"UPDATE applications.workflow_definitions SET table_id=table_id WHERE id=$1",
	} {
		_, e := f.runtime.Exec(f.ctx, q, h.FlowID)
		rootSQLState(t, e, "42501")
	}
	for _, name := range []string{"workflow_definitions", "workflow_versions", "workflow_deployments", "workflow_instances"} {
		var backup, reader, maint, trunc bool
		e := f.owner.QueryRow(f.ctx, "SELECT has_table_privilege('auth_backup',$1,'SELECT'),has_table_privilege('auth_reader',$1,'SELECT'),has_table_privilege('auth_maintenance',$1,'SELECT'),has_table_privilege('auth_app',$1,'TRUNCATE')", "applications."+name).Scan(&backup, &reader, &maint, &trunc)
		if e != nil || !backup || reader || maint || trunc {
			t.Fatalf("%s least privilege %v/%v/%v/%v err=%v", name, backup, reader, maint, trunc, e)
		}
	}
}
