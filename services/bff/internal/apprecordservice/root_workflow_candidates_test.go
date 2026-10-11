package apprecordservice

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

// V075 P5 / V043 permission intersection / V015 14.2. These tests use the
// actual restricted runtime connection and Redis, never a mock candidate port.
func rootCandidateSetup(t *testing.T, editable bool) rootTaskFixture {
	return rootCandidateKindSetup(t, editable, "member")
}
func rootCandidateKindSetup(t *testing.T, editable bool, kind string) rootTaskFixture {
	t.Helper()
	base := rootCaptureSetup(t)
	f := base.recordFixture
	if kind == "department" {
		defs := make([]appfields.Field, len(base.fields))
		for i, field := range base.fields {
			defs[i] = field.Definition
			if defs[i].ID == f.reference {
				defs[i].Kind = "department"
			}
		}
		rootCaptureDefinitions(t, base, defs)
		rootSaveExec(t, rootTaskFixture{rootProjection: rootProjection{recordFixture: f}}, "UPDATE "+rootCaptureRelation(base)+" SET "+rootCaptureColumn(f.reference)+"=NULL")
	}
	rootTaskKeepDispatchPrivate(t, f)
	graph := rootCatalogGraph(t, f, false)
	if editable {
		graph.Nodes[1].Approval.EditableFieldIDs = []string{f.reference}
	}
	h := rootCatalogReady(t, f, graph)
	instance := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var version string
	if err := f.owner.QueryRow(f.ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	p := rootProjection{recordFixture: f, head: h, instance: instance, version: version, first: graph.Nodes[1].ID, second: graph.Nodes[1].ID}
	c, payload := rootRecoveryAccept(t, p)
	task := p.task(t, p.first, p.actor, 1)
	receipt, body := p.receipt(t, c, "active", "", task)
	p.apply(t, c, payload, receipt, body, true)
	return rootTaskFixture{rootProjection: p, task: task}
}
func rootCandidateRequest(f rootTaskFixture) WorkflowCandidateRequest {
	return WorkflowCandidateRequest{WorkflowTaskRequest: f.request(), FieldID: f.reference, PageSize: 1}
}
func TestRootWorkflowCandidatesMemberPaginationMinimalAndReadOnly(t *testing.T) {
	f := rootCandidateSetup(t, true)
	req := rootCandidateRequest(f)
	first, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
	if err != nil {
		t.Fatalf("authorized active task must list member candidates: %v", err)
	}
	if len(first.Items) != 1 || !first.Pagination.HasMore || first.Pagination.NextPageToken == nil {
		t.Fatal("bounded candidate page or next cursor missing")
	}
	raw, err := json.Marshal(first.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if json.Unmarshal(raw, &item) != nil || len(item) != 3 || item["status"] != "active" || item["id"] == nil || item["label"] == nil {
		t.Fatalf("candidate is not the minimal active DTO: %s", raw)
	}
	if strings.Contains(string(raw), "password") {
		t.Fatal("private account data in candidates")
	}
	req.PageToken = *first.Pagination.NextPageToken
	next, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
	if err != nil || len(next.Items) != 1 {
		t.Fatalf("second bounded page: %v", err)
	}
	other, _ := json.Marshal(next.Items[0])
	if string(other) == string(raw) {
		t.Fatal("cursor repeated the first item")
	}
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowCandidatesRejectReadonlyNodeAndRevokedField(t *testing.T) {
	for _, kind := range []string{"node", "field"} {
		t.Run(kind, func(t *testing.T) {
			f := rootCandidateSetup(t, kind != "node")
			req := rootCandidateRequest(f)
			if kind == "field" {
				rootSaveExec(t, f, "DELETE FROM applications.grant_fields gf USING applications.grants g WHERE gf.grant_id=g.id AND g.app_id=$1 AND g.action='data.edit'", f.app)
			}
			result, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
			if !errors.Is(err, applications.ErrDenied) || len(result.Items) != 0 {
				t.Fatalf("must deny without candidates: %v", err)
			}
		})
	}
}
func TestRootWorkflowCandidatesRequireCurrentTaskActorAndResource(t *testing.T) {
	for _, kind := range []string{"actor", "record", "instance", "task"} {
		t.Run(kind, func(t *testing.T) {
			f := rootCandidateSetup(t, true)
			req := rootCandidateRequest(f)
			p := f.principal
			want := applications.ErrMissing
			switch kind {
			case "actor":
				p.UserID = f.other
				want = applications.ErrDenied
			case "record":
				req.RecordID = f.otherRecord
			case "instance":
				req.InstanceID = recordOperationID(t, f.recordFixture)
			case "task":
				req.TaskID = recordOperationID(t, f.recordFixture)
			}
			result, err := f.service.WorkflowTaskCandidates(f.ctx, p, req)
			if !errors.Is(err, want) || len(result.Items) != 0 {
				t.Fatalf("wrong %s accepted or wrong boundary: %v", kind, err)
			}
		})
	}
}
func TestRootWorkflowCandidatesClosingFlowStillAllowsItsActiveTask(t *testing.T) {
	f := rootCandidateSetup(t, true)
	rootCatalogTx(t, f.recordFixture, func(tx pgx.Tx) error {
		h, e := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, f.head.FlowID, f.head.Revision)
		if e == nil && h.State != "closing" {
			t.Fatal("fixture did not enter closing")
		}
		return e
	})
	result, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, rootCandidateRequest(f))
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("closing active task candidates: %v", err)
	}
}
func TestRootWorkflowCandidatesCursorCannotCrossQueryOrRecordRevision(t *testing.T) {
	for _, kind := range []string{"query", "recordVersion"} {
		t.Run(kind, func(t *testing.T) {
			f := rootCandidateSetup(t, true)
			req := rootCandidateRequest(f)
			first, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
			if err != nil || first.Pagination.NextPageToken == nil {
				t.Fatalf("initial authorized page: %v", err)
			}
			req.PageToken = *first.Pagination.NextPageToken
			if kind == "query" {
				req.Query = "changed"
			} else {
				rootSaveExec(t, f, "UPDATE "+rootCaptureRelation(rootEvidenceStoreFixture{recordFixture: f.recordFixture})+" SET record_version=record_version+1 WHERE id=$1", f.ownRecord)
			}
			result, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
			if err == nil || len(result.Items) != 0 {
				t.Fatal("out-of-scope cursor returned candidates")
			}
		})
	}
}

func TestRootWorkflowCandidatesOnlyActivePrefixMatches(t *testing.T) {
	f := rootCandidateSetup(t, true)
	req := rootCandidateRequest(f)
	prefix := "v075-" + f.app + "-"
	req.Query = " " + prefix + " "
	req.PageSize = 50
	var active string
	if err := f.owner.QueryRow(f.ctx, "INSERT INTO auth.users(account) VALUES($1) RETURNING id::text", prefix+"active").Scan(&active); err != nil {
		t.Fatal(err)
	}
	rootSaveExec(t, f, "INSERT INTO auth.users(account,status) VALUES($1,'disabled')", prefix+"disabled")
	result, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
	if err != nil || len(result.Items) != 1 || result.Pagination.HasMore || result.Pagination.NextPageToken != nil {
		t.Fatalf("active prefix-only result: %v", err)
	}
	raw, _ := json.Marshal(result.Items[0])
	var item map[string]any
	_ = json.Unmarshal(raw, &item)
	if item["id"] != active || item["label"] != prefix+"active" || item["status"] != "active" {
		t.Fatalf("wrong active member: %s", raw)
	}
}
func TestRootWorkflowCandidatesDepartmentUsesMinimalParentDisplay(t *testing.T) {
	f := rootCandidateKindSetup(t, true, "department")
	req := rootCandidateRequest(f)
	var parent, child string
	if err := f.owner.QueryRow(f.ctx, "SELECT id::text FROM personnel.departments WHERE is_root LIMIT 1").Scan(&parent); err != nil {
		t.Fatal(err)
	}
	name := "v075-" + f.app
	if err := f.owner.QueryRow(f.ctx, "INSERT INTO personnel.departments(name,parent_id) VALUES($1,$2) RETURNING id::text", name, parent).Scan(&child); err != nil {
		t.Fatal(err)
	}
	req.Query = name
	result, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("department candidates: %v", err)
	}
	raw, _ := json.Marshal(result.Items[0])
	var item map[string]any
	_ = json.Unmarshal(raw, &item)
	if len(item) != 4 || item["id"] != child || item["label"] != name || item["parentId"] != parent || item["status"] != "active" {
		t.Fatalf("not minimal department DTO: %s", raw)
	}
	rootSaveCounts(t, f, 0)
}
func TestRootWorkflowCandidatesCursorBindsCurrentSession(t *testing.T) {
	f := rootCandidateSetup(t, true)
	req := rootCandidateRequest(f)
	first, err := f.service.WorkflowTaskCandidates(f.ctx, f.principal, req)
	if err != nil || first.Pagination.NextPageToken == nil {
		t.Fatalf("first page: %v", err)
	}
	req.PageToken = *first.Pagination.NextPageToken
	p := f.principal
	p.SessionRef = recordOperationID(t, f.recordFixture)
	result, err := f.service.WorkflowTaskCandidates(f.ctx, p, req)
	if err == nil || len(result.Items) != 0 {
		t.Fatal("cursor replayed in a different session")
	}
}
