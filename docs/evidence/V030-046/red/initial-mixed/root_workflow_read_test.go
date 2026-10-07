package apprecordservice

import (
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"testing"
)

func rootReadRequest(f rootTaskFixture) WorkflowInstanceSearchRequest {
	return WorkflowInstanceSearchRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, Page: 1, PageSize: 20}
}
func TestRootWorkflowReadCurrentRecordSummary(t *testing.T) {
	f := rootTaskSetup(t, false)
	r, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, rootReadRequest(f))
	if e != nil {
		t.Fatal(e)
	}
	if r.Total != 1 || len(r.Items) != 1 || r.QueryVersion == "" || r.Page != 1 || r.PageSize != 20 {
		t.Fatalf("bad page %+v", r)
	}
	v := r.Items[0]
	if v.ID != f.instance.ID || v.FlowID != f.head.FlowID || v.State != "active" || v.Sequence != 1 || v.DefinitionVersion != 1 || v.CreatedAt == "" || v.Name == "" {
		t.Fatalf("bad summary %+v", v)
	}
}
func TestRootWorkflowReadOwnScopeRejectsForeignRecord(t *testing.T) {
	f := rootTaskSetup(t, false)
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.grants SET row_scope='own' WHERE app_id=$1 AND action='data.read'", f.app); e != nil {
		t.Fatal(e)
	}
	q := rootReadRequest(f)
	q.RecordID = f.otherRecord
	r, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if !errors.Is(e, applications.ErrMissing) || len(r.Items) != 0 {
		t.Fatalf("foreign row visible: %+v %v", r, e)
	}
}
func TestRootWorkflowReadRevocationIsLive(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootReadRequest(f)
	first, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	q.QueryVersion = first.QueryVersion
	r, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e == nil || len(r.Items) != 0 {
		t.Fatalf("revoked read returned data %+v %v", r, e)
	}
}
func TestRootWorkflowReadTwoFlowsPageWithoutDuplicates(t *testing.T) {
	f := rootTaskSetup(t, false)
	head := rootCatalogReady(t, f.recordFixture, rootCatalogGraph(t, f.recordFixture, false))
	rootCatalogReserve(t, f.recordFixture, rootCatalogReserveInput(t, f.recordFixture, head))
	q := rootReadRequest(f)
	q.PageSize = 1
	a, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	q.Page = 2
	q.QueryVersion = a.QueryVersion
	b, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	if a.Total != 2 || b.Total != 2 || len(a.Items) != 1 || len(b.Items) != 1 || a.Items[0].ID == b.Items[0].ID {
		t.Fatalf("wrong pagination %+v %+v", a, b)
	}
}
func TestRootWorkflowReadRelatedChangeRequiresRefresh(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootReadRequest(f)
	a, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET sequence=sequence+1,updated_at=clock_timestamp() WHERE id=$1", f.instance.ID); e != nil {
		t.Fatal(e)
	}
	q.QueryVersion = a.QueryVersion
	_, e = f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if !errors.Is(e, querycontext.ErrChanged) {
		t.Fatalf("related update not rejected: %v", e)
	}
}
func TestRootWorkflowReadUnrelatedRecordDoesNotExpire(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootReadRequest(f)
	a, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	head := rootCatalogReady(t, f.recordFixture, rootCatalogGraph(t, f.recordFixture, false))
	in := rootCatalogReserveInput(t, f.recordFixture, head)
	in.RecordID = f.otherRecord
	rootCatalogReserve(t, f.recordFixture, in)
	q.QueryVersion = a.QueryVersion
	b, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e != nil || b.Total != a.Total || len(b.Items) != 1 || b.Items[0].ID != a.Items[0].ID {
		t.Fatalf("unrelated instance invalidated current record: %+v %v", b, e)
	}
}
func TestRootWorkflowReadRejectsInvalidPageAndForeignSession(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootReadRequest(f)
	for _, n := range []int{0, -1} {
		q.Page = n
		_, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
		if e == nil {
			t.Fatalf("accepted page%d", n)
		}
	}
	q = rootReadRequest(f)
	q.PageSize = 101
	if _, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q); e == nil {
		t.Fatal("unbounded page")
	}
	q = rootReadRequest(f)
	r, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	q.QueryVersion = r.QueryVersion
	p := f.principal
	p.SessionRef = recordOperationID(t, f.recordFixture)
	if _, e = f.service.SearchRecordWorkflows(f.ctx, p, q); e == nil {
		t.Fatal("foreign session reused query")
	}
}
