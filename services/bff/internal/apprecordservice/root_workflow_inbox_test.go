package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func inboxRequest() WorkflowInboxRequest { return WorkflowInboxRequest{Page: 1} }
func inboxSearch(t *testing.T, f rootTaskFixture, q WorkflowInboxRequest) WorkflowInboxResult {
	t.Helper()
	r, e := f.service.SearchWorkflowInbox(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func inboxSQL(t *testing.T, f rootTaskFixture, q string, args ...any) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, q, args...); e != nil {
		t.Fatal(e)
	}
}
func TestRootWorkflowInboxExactPersonalProjection(t *testing.T) {
	f := rootTaskSetup(t, true)
	r := inboxSearch(t, f, inboxRequest())
	if r.Total != 1 || len(r.Items) != 1 || r.Page != 1 || r.PageSize != 20 || r.QueryVersion == "" {
		t.Fatalf("bad personal page: %+v", r)
	}
	v := r.Items[0]
	if v.ID != f.task.ID || v.AppID != f.app || v.ViewID != f.view || v.RecordID != f.ownRecord || v.InstanceID != f.instance.ID || v.FlowID != f.head.FlowID || v.NodeID != f.first || v.FlowName != "Expense" || v.ActivationEpoch != 1 || v.DefinitionVersion != 1 || v.Sequence != 1 {
		t.Fatalf("wrong independent identity: %+v", v)
	}
	if _, e := time.Parse(time.RFC3339Nano, v.CreatedAt); e != nil || !strings.HasSuffix(v.CreatedAt, "Z") {
		t.Fatal("invalid UTC time", v.CreatedAt, e)
	}
	raw, _ := json.Marshal(v)
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	keys := []string{}
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"activationEpoch", "appId", "createdAt", "definitionVersion", "flowId", "flowName", "id", "instanceId", "nodeId", "recordId", "sequence", "viewId"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatal("projection leaked or omitted fields", keys)
	}
	rootTaskNoEvidenceWrites(t, f)
}
func TestRootWorkflowInboxOwnerCannotReadOthersQueue(t *testing.T) {
	f := rootTaskSetup(t, false)
	f.principal.UserID = f.other
	f.principal.SessionRef = f.other
	r := inboxSearch(t, f, inboxRequest())
	if r.Total != 0 || r.Items == nil || len(r.Items) != 0 {
		t.Fatal("owner saw other's assigned task")
	}
}
func TestRootWorkflowInboxCurrentAccessBeforeCount(t *testing.T) {
	for _, change := range []string{"menu", "groups", "own", "fields"} {
		t.Run(change, func(t *testing.T) {
			f := rootTaskSetup(t, true)
			switch change {
			case "menu":
				inboxSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='menu.enter'", f.app)
			case "groups":
				inboxSQL(t, f, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app)
			case "own":
				inboxSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all')", f.app)
				inboxSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all'", f.app)
			case "fields":
				inboxSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app)
			}
			r := inboxSearch(t, f, inboxRequest())
			if r.Total != 0 || r.Items == nil || len(r.Items) != 0 {
				t.Fatal("invisible record counted or exposed")
			}
		})
	}
}
func TestRootWorkflowInboxRevocationRequiresExplicitRefresh(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := inboxRequest()
	a := inboxSearch(t, f, q)
	inboxSQL(t, f, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app)
	q.QueryVersion = a.QueryVersion
	r, e := f.service.SearchWorkflowInbox(f.ctx, f.principal, q)
	if !errors.Is(e, querycontext.ErrChanged) || len(r.Items) != 0 {
		t.Fatal("stale authorized page survived", e)
	}
	q.QueryVersion = ""
	fresh := inboxSearch(t, f, q)
	if fresh.Total != 0 {
		t.Fatal("fresh revoked task leaked")
	}
}
func TestRootWorkflowInboxRelatedAndUnrelatedChanges(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := inboxRequest()
	a := inboxSearch(t, f, q)
	q.QueryVersion = a.QueryVersion
	inboxSQL(t, f, "UPDATE applications.apps SET policy_revision=policy_revision+1 WHERE id=$1", f.app)
	b := inboxSearch(t, f, q)
	if b.QueryVersion != a.QueryVersion || b.Total != 1 {
		t.Fatal("unrelated policy revision refreshed")
	}
	inboxSQL(t, f, "UPDATE applications.workflow_definitions SET name='Renamed' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	r, e := f.service.SearchWorkflowInbox(f.ctx, f.principal, q)
	if !errors.Is(e, querycontext.ErrChanged) || len(r.Items) != 0 {
		t.Fatal("changed visible flow name was not rejected", e)
	}
}
func TestRootWorkflowInboxClosedTaskRemoved(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := inboxRequest()
	a := inboxSearch(t, f, q)
	c, p := f.accept(t, "agree", f.task.ID, "", 1, 1)
	r, b := f.receipt(t, c, "completed", "")
	f.apply(t, c, p, r, b, true)
	q.QueryVersion = a.QueryVersion
	_, e := f.service.SearchWorkflowInbox(f.ctx, f.principal, q)
	if !errors.Is(e, querycontext.ErrChanged) {
		t.Fatal("completed task not invalidated", e)
	}
	q.QueryVersion = ""
	fresh := inboxSearch(t, f, q)
	if fresh.Total != 0 || len(fresh.Items) != 0 {
		t.Fatal("closed task still visible")
	}
}
func TestRootWorkflowInboxSessionAndInput(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := inboxRequest()
	a := inboxSearch(t, f, q)
	p := f.principal
	p.SessionRef = recordOperationID(t, f.recordFixture)
	q.QueryVersion = a.QueryVersion
	if _, e := f.service.SearchWorkflowInbox(f.ctx, p, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("cross-session token accepted", e)
	}
	p = f.principal
	p.Record.AuthVersion = "9999"
	if _, e := f.service.SearchWorkflowInbox(f.ctx, p, inboxRequest()); !errors.Is(e, session.ErrUnauthorized) {
		t.Fatal("revoked auth accepted", e)
	}
	for _, bad := range []WorkflowInboxRequest{{Page: 0}, {Page: -1}, {Page: 1, PageSize: 101}, {Page: 1, PageSize: -1}, {Page: 9007199254740991, PageSize: 100}} {
		if _, e := f.service.SearchWorkflowInbox(f.ctx, f.principal, bad); e == nil {
			t.Fatal("invalid page accepted", bad)
		}
	}
	q = inboxRequest()
	q.Page = 2
	q.PageSize = 1
	b := inboxSearch(t, f, q)
	if b.Total != 1 || len(b.Items) != 0 || b.Items == nil {
		t.Fatal("beyond-total page incorrectly clamped")
	}
}
