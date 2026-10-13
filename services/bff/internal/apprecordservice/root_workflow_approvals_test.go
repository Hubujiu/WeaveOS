package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"reflect"
	"sort"
	"testing"
)

func approvedRead(t *testing.T, f rootTaskFixture, q WorkflowApprovalsRequest) WorkflowApprovalsResult {
	t.Helper()
	r, e := f.service.SearchWorkflowApprovals(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestRootPersonalApprovedUsesOnlyOwnSuccessfulActionsAndMinimalMetadata(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public)
	before := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1})
	if before.Total != 0 || before.Items == nil {
		t.Fatal("assigned/start event counted as approved", before)
	}
	historyNoEffect(t, f)
	empty := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1})
	if empty.Total != 0 {
		t.Fatal("no_effect counted as approved")
	}
	q := eventConfirmed(t, f, "reject")
	r := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1})
	if r.Total != 1 || len(r.Items) != 1 || r.Page != 1 || r.PageSize != 20 || r.QueryVersion == "" {
		t.Fatal("incomplete approval page", r)
	}
	item := r.Items[0]
	e := item.Event
	if item.AppID != f.app || item.ViewID != f.view || item.RecordID != f.ownRecord || e.ID != q.EventID || e.ActorID != f.actor || e.Action != "reject" || e.Outcome != "success" || e.NodeID == nil || *e.NodeID != f.first || e.FlowName != "Expense" || e.FlowNameSource != "captured" {
		t.Fatalf("wrong independent approval summary %+v", item)
	}
	raw, _ := json.Marshal(item)
	var m map[string]json.RawMessage
	json.Unmarshal(raw, &m)
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"appId", "event", "recordId", "viewId"}) {
		t.Fatal("public summary leaked fields", keys)
	}
	var event map[string]json.RawMessage
	json.Unmarshal(m["event"], &event)
	if len(event) != 14 {
		t.Fatal("event is not original minimal journal DTO", event)
	}
	other := f.principal
	other.UserID = f.other
	other.SessionRef = f.other
	own, e2 := f.service.SearchWorkflowApprovals(f.ctx, other, WorkflowApprovalsRequest{Page: 1})
	if e2 != nil || own.Total != 0 || own.Items == nil {
		t.Fatal("owner saw another person's approved events", own, e2)
	}
}
func TestRootPersonalApprovedCurrentReadHistoryAndMenuBeforeCount(t *testing.T) {
	for _, action := range []string{"menu.enter", "data.read", "data.history"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, true)
			eventGrant(t, f, "all", f.public)
			eventConfirmed(t, f, "agree")
			first := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1})
			if first.Total != 1 {
				t.Fatal("eligible event absent")
			}
			eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action=$2)", f.app, action)
			eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action=$2", f.app, action)
			got, e := f.service.SearchWorkflowApprovals(f.ctx, f.principal, WorkflowApprovalsRequest{Page: 1, QueryVersion: first.QueryVersion})
			if !errors.Is(e, querycontext.ErrChanged) || len(got.Items) != 0 {
				t.Fatal("revoked event survived old query", got, e)
			}
			next := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1})
			if next.Total != 0 || len(next.Items) != 0 {
				t.Fatal("unauthorized event counted", next)
			}
		})
	}
}
func TestRootPersonalApprovedSurvivesTerminalProjectionRemovalAndRename(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public)
	eventConfirmed(t, f, "reject")
	q := WorkflowApprovalsRequest{Page: 1}
	first := approvedRead(t, f, q)
	if first.Total != 1 {
		t.Fatal("missing event")
	}
	q.QueryVersion = first.QueryVersion
	eventSQL(t, f, "DELETE FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", f.app, f.instance.ID)
	eventSQL(t, f, "DELETE FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 AND state='rejected'", f.app, f.instance.ID)
	eventSQL(t, f, "UPDATE applications.workflow_versions SET graph_json='{}',bpmn_xml='<removed-test-fixture/>' WHERE app_id=$1 AND flow_id=$2", f.app, f.head.FlowID)
	eventSQL(t, f, "UPDATE applications.workflow_definitions SET name='Changed after approval' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	next := approvedRead(t, f, q)
	if !reflect.DeepEqual(next, first) {
		t.Fatal("original history depended on removed runtime/current name", first, next)
	}
}
func TestRootPersonalApprovedPurposeSessionAndPaginationRemainIndependent(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	inbox := inboxSearch(t, f, inboxRequest())
	eventConfirmed(t, f, "agree")
	first := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1, PageSize: 1})
	next := approvedRead(t, f, WorkflowApprovalsRequest{Page: 2, PageSize: 1, QueryVersion: first.QueryVersion})
	if next.Total != 1 || next.Items == nil || len(next.Items) != 0 {
		t.Fatal("beyond total page is not exact", next)
	}
	_, e := f.service.SearchWorkflowApprovals(f.ctx, f.principal, WorkflowApprovalsRequest{Page: 1, QueryVersion: inbox.QueryVersion})
	if !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("inbox token reused for approved history", e)
	}
	p := f.principal
	p.SessionRef = f.other
	_, e = f.service.SearchWorkflowApprovals(f.ctx, p, WorkflowApprovalsRequest{Page: 1, QueryVersion: first.QueryVersion})
	if !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("another Session reused history token", e)
	}
	eventSQL(t, f, "UPDATE applications.apps SET policy_revision=policy_revision+1 WHERE id=$1", f.app)
	same := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1, PageSize: 1, QueryVersion: first.QueryVersion})
	if same.QueryVersion != first.QueryVersion {
		t.Fatal("irrelevant policy revision forced a refresh")
	}
}
func TestRootPersonalApprovedBrokenVisibleJournalFailsClosed(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	q := eventConfirmed(t, f, "agree")
	first := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1})
	if first.Total != 1 {
		t.Fatal("missing event")
	}
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET flow_id=$2 WHERE command_id=$1", q.EventID, recordOperationID(t, f.recordFixture))
	got, e := f.service.SearchWorkflowApprovals(f.ctx, f.principal, WorkflowApprovalsRequest{Page: 1})
	if !errors.Is(e, ErrUnavailable) || len(got.Items) != 0 {
		t.Fatal("damaged binding yielded a partial history", got, e)
	}
}
