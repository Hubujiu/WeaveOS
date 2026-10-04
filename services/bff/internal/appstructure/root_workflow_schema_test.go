package appstructure

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

type rootWorkflowSchemaFixture struct {
	f                  *fixture
	table, view, field string
}

func rootWorkflowSchemaSetup(t *testing.T) rootWorkflowSchemaFixture {
	t.Helper()
	f := setup(t)
	out := data(t, f.call(t, "POST", "/forms", map[string]any{"operationId": uuid(t, f.owner), "name": "Approval form", "source": map[string]any{"kind": "new_table"}, "directoryId": nil, "position": 0, "expectedStructureVersion": 0}), 201)
	s := rootWorkflowSchemaFixture{f: f, table: out["table"].(map[string]any)["id"].(string), view: out["form"].(map[string]any)["id"].(string), field: uuid(t, f.owner)}
	body := s.body(t, []any{s.fieldValue("text", "Original")})
	body["expectedSchemaVersion"] = 0
	body["expectedViewVersion"] = 0
	data(t, f.call(t, "PUT", "/forms/"+s.view+"/definition", body), 200)
	return s
}
func (s rootWorkflowSchemaFixture) fieldValue(kind, name string) map[string]any {
	return map[string]any{"id": s.field, "name": name, "kind": kind, "required": false, "default": nil, "config": map[string]any{"maxLength": nil}, "presentation": map[string]any{"helpText": nil, "displayTimeZone": nil}}
}
func (s rootWorkflowSchemaFixture) body(t *testing.T, fields []any) map[string]any {
	return map[string]any{"operationId": uuid(t, s.f.owner), "expectedSchemaVersion": 1, "expectedViewVersion": 1, "fields": fields, "layout": []any{}, "optionMappings": []any{}, "confirmationToken": nil}
}
func (s rootWorkflowSchemaFixture) graph(t *testing.T, dependent bool) flowgraph.Graph {
	start, node, end := uuid(t, s.f.owner), uuid(t, s.f.owner), uuid(t, s.f.owner)
	fields := []string{}
	if dependent {
		fields = append(fields, s.field)
	}
	return flowgraph.Graph{Version: 1, Nodes: []flowgraph.Node{{ID: start, Kind: "start"}, {ID: node, Kind: "approval", Approval: &flowgraph.Approval{Mode: "all", AssigneeIDs: []string{s.f.actor}, EditableFieldIDs: fields}}, {ID: end, Kind: "end"}}, Edges: []flowgraph.Edge{{From: start, To: node}, {From: node, To: end}}}
}
func (s rootWorkflowSchemaFixture) tx(t *testing.T, fn func(pgx.Tx) error) {
	t.Helper()
	c := context.Background()
	tx, e := s.f.runtime.Begin(c)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(c)
	if e = fn(tx); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(c); e != nil {
		t.Fatal(e)
	}
}
func (s rootWorkflowSchemaFixture) version(t *testing.T, id string, rev int64, dependent bool) workflowcatalog.Head {
	t.Helper()
	g := s.graph(t, dependent)
	var h workflowcatalog.Head
	c := context.Background()
	s.tx(t, func(tx pgx.Tx) error {
		var e error
		h, e = (workflowcatalog.Catalog{}).PutVersionInTx(c, tx, workflowcatalog.VersionInput{AppID: s.f.app, TableID: s.table, ViewID: s.view, FlowID: id, ActorID: s.f.actor, Name: "Review", ExpectedRevision: rev, ExpectedSchemaVersion: 1, Graph: g})
		return e
	})
	s.tx(t, func(tx pgx.Tx) error {
		var e error
		h, e = (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(c, tx, s.f.app, h.FlowID, h.CandidateVersion, "deployment-"+uuid(t, s.f.owner))
		return e
	})
	return h
}
func (s rootWorkflowSchemaFixture) enable(t *testing.T, h workflowcatalog.Head) workflowcatalog.Head {
	t.Helper()
	s.tx(t, func(tx pgx.Tx) error {
		var e error
		h, e = (workflowcatalog.Catalog{}).EnableInTx(context.Background(), tx, s.f.app, h.FlowID, h.Revision)
		return e
	})
	return h
}
func (s rootWorkflowSchemaFixture) flow(t *testing.T) workflowcatalog.Head {
	return s.enable(t, s.version(t, uuid(t, s.f.owner), 0, true))
}
func rootWorkflowConflict(t *testing.T, w *httptest.ResponseRecorder, flow, field string) {
	t.Helper()
	var env struct {
		Code string
		Data struct {
			Conflicts []struct {
				FlowID                  string
				Version                 int64
				NodeID, FieldID, Reason string
			}
		}
	}
	if w.Code != 409 || json.Unmarshal(w.Body.Bytes(), &env) != nil || env.Code != "APPLICATION_SCHEMA_WORKFLOW_INCOMPATIBLE" {
		t.Fatalf("wanted workflow compatibility conflict: %d %s", w.Code, w.Body.String())
	}
	found := false
	for _, c := range env.Data.Conflicts {
		if c.FlowID == flow && c.FieldID == field && c.Version == 1 && c.NodeID != "" && c.Reason != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing scoped diagnostic %s", w.Body.String())
	}
}
func TestRootWorkflowSchemaSaveRejectsAndRollsBackAllChanges(t *testing.T) {
	s := rootWorkflowSchemaSetup(t)
	h := s.flow(t)
	newField := s.fieldValue("text", "Other")
	newID := uuid(t, s.f.owner)
	newField["id"] = newID
	body := s.body(t, []any{newField})
	rootWorkflowConflict(t, s.f.call(t, "PUT", "/forms/"+s.view+"/definition", body), h.FlowID, s.field)
	c := context.Background()
	var revision int64
	var original, added bool
	var operations int
	if e := s.f.owner.QueryRow(c, "SELECT schema_version FROM applications.logical_tables WHERE id=$1", s.table).Scan(&revision); e != nil {
		t.Fatal(e)
	}
	q := "SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=to_regclass($1) AND attname=$2 AND NOT attisdropped)"
	physical := "appdata.t_" + strings.ReplaceAll(s.table, "-", "")
	if e := s.f.owner.QueryRow(c, q, physical, "f_"+strings.ReplaceAll(s.field, "-", "")).Scan(&original); e != nil {
		t.Fatal(e)
	}
	if e := s.f.owner.QueryRow(c, q, physical, "f_"+strings.ReplaceAll(newID, "-", "")).Scan(&added); e != nil {
		t.Fatal(e)
	}
	if e := s.f.owner.QueryRow(c, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", s.f.actor, body["operationId"]).Scan(&operations); e != nil {
		t.Fatal(e)
	}
	if revision != 1 || !original || added || operations != 0 {
		t.Fatalf("partial save rev=%d old=%v new=%v operation=%d", revision, original, added, operations)
	}
}
func TestRootWorkflowSchemaPreflightReportsConflictWithoutConfirmation(t *testing.T) {
	s := rootWorkflowSchemaSetup(t)
	s.flow(t)
	body := s.body(t, []any{})
	delete(body, "operationId")
	delete(body, "confirmationToken")
	out := data(t, s.f.call(t, "POST", "/forms/"+s.view+"/definition/preflight", body), 200)
	if out["saveAllowed"] != false || out["confirmation"] != nil {
		t.Fatalf("preflight must refuse save/token %+v", out)
	}
	conflicts, ok := out["workflowConflicts"].([]any)
	if !ok || len(conflicts) == 0 {
		t.Fatalf("missing workflow diagnostics %+v", out)
	}
}
func TestRootWorkflowSchemaAllowsCompatibleRenameAndTextKind(t *testing.T) {
	for _, kind := range []string{"text", "multiline"} {
		t.Run(kind, func(t *testing.T) {
			s := rootWorkflowSchemaSetup(t)
			s.flow(t)
			out := data(t, s.f.call(t, "PUT", "/forms/"+s.view+"/definition", s.body(t, []any{s.fieldValue(kind, "Renamed")})), 200)
			if out["definition"].(map[string]any)["table"].(map[string]any)["schemaVersion"] != float64(2) {
				t.Fatalf("compatible change not committed %+v", out)
			}
		})
	}
}
func TestRootWorkflowSchemaRechecksAfterPreviouslyAllowedPreflight(t *testing.T) {
	s := rootWorkflowSchemaSetup(t)
	h := s.version(t, uuid(t, s.f.owner), 0, true)
	body := s.body(t, []any{})
	pre := map[string]any{}
	for k, v := range body {
		if k != "operationId" && k != "confirmationToken" {
			pre[k] = v
		}
	}
	out := data(t, s.f.call(t, "POST", "/forms/"+s.view+"/definition/preflight", pre), 200)
	if out["saveAllowed"] != true {
		t.Fatalf("disabled unused definition blocked %+v", out)
	}
	h = s.enable(t, h)
	rootWorkflowConflict(t, s.f.call(t, "PUT", "/forms/"+s.view+"/definition", body), h.FlowID, s.field)
}
func TestRootWorkflowSchemaClosedUnusedHistoryAllowsRemoval(t *testing.T) {
	s := rootWorkflowSchemaSetup(t)
	h := s.flow(t)
	s.tx(t, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).RequestCloseInTx(context.Background(), tx, s.f.app, h.FlowID, h.Revision)
		return e
	})
	data(t, s.f.call(t, "PUT", "/forms/"+s.view+"/definition", s.body(t, []any{})), 200)
}
func TestRootWorkflowSchemaInFlightOldDefinitionStillProtectsField(t *testing.T) {
	s := rootWorkflowSchemaSetup(t)
	h := s.flow(t)
	c := context.Background()
	record := uuid(t, s.f.owner)
	physical := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(s.table, "-", "")}.Sanitize()
	if _, e := s.f.owner.Exec(c, "INSERT INTO "+physical+"(id,created_by) VALUES($1,$2)", record, s.f.actor); e != nil {
		t.Fatal(e)
	}
	s.tx(t, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).ReserveInTx(c, tx, workflowcatalog.ReserveInput{AppID: s.f.app, FlowID: h.FlowID, InstanceID: uuid(t, s.f.owner), RecordID: record, ActorID: s.f.actor, ExpectedRevision: h.Revision, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1})
		return e
	})
	s.version(t, h.FlowID, h.Revision, false)
	rootWorkflowConflict(t, s.f.call(t, "PUT", "/forms/"+s.view+"/definition", s.body(t, []any{})), h.FlowID, s.field)
}
func TestRootWorkflowSchemaSaveSerializesWithEnable(t *testing.T) {
	s := rootWorkflowSchemaSetup(t)
	h := s.version(t, uuid(t, s.f.owner), 0, true)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, e := s.f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); e != nil {
		t.Fatal(e)
	}
	if _, e = (workflowcatalog.Catalog{}).EnableInTx(ctx, tx, s.f.app, h.FlowID, h.Revision); e != nil {
		t.Fatal(e)
	}
	body := s.body(t, []any{})
	reply := make(chan *httptest.ResponseRecorder, 1)
	go func() { reply <- s.f.call(t, "PUT", "/forms/"+s.view+"/definition", body) }()
	blocked := false
	for !blocked {
		select {
		case w := <-reply:
			t.Fatalf("schema Save escaped uncommitted enable: %d %s", w.Code, w.Body.String())
		case <-ctx.Done():
			t.Fatal("Save did not reach real app gate lock")
		default:
		}
		if e = s.f.owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity a WHERE $1=ANY(pg_blocking_pids(a.pid)))", pid).Scan(&blocked); e != nil {
			t.Fatal(e)
		}
		if !blocked {
			time.Sleep(5 * time.Millisecond)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w := <-reply:
		rootWorkflowConflict(t, w, h.FlowID, s.field)
	case <-ctx.Done():
		t.Fatal("Save failed to resume after enable commit")
	}
}
