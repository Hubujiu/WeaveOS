package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"reflect"
	"sort"
	"testing"
)

func startedRead(t *testing.T, f rootTaskFixture, q WorkflowStartedRequest) WorkflowStartedResult {
	t.Helper()
	r, e := f.service.SearchWorkflowStarted(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestRootPersonalStartedUsesInitiatorNotRecordCreatorOrOwner(t *testing.T) {
	f := rootTaskSetup(t, true)
	r := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if r.Total != 1 || len(r.Items) != 1 || r.Page != 1 || r.PageSize != 20 || r.QueryVersion == "" {
		t.Fatal("incomplete personal started page", r)
	}
	row := r.Items[0]
	v := row.Instance
	if row.AppID != f.app || row.ViewID != f.view || row.RecordID != f.ownRecord || v.ID != f.instance.ID || v.InitiatorID != f.actor || v.State != "active" || v.Sequence != 1 || v.DefinitionVersion != 1 || v.Name != "Expense" {
		t.Fatal("incorrect independent identity", row)
	}
	raw, _ := json.Marshal(row)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	keys := []string{}
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"appId", "instance", "recordId", "viewId"}) {
		t.Fatal("extra summary data", keys)
	}
	var instance map[string]json.RawMessage
	json.Unmarshal(fields["instance"], &instance)
	if len(instance) != 9 {
		t.Fatal("instance is not V046 minimal DTO", instance)
	}
	p := f.principal
	p.UserID = f.other
	p.SessionRef = f.other
	other, e := f.service.SearchWorkflowStarted(f.ctx, p, WorkflowStartedRequest{Page: 1})
	if e != nil || other.Total != 0 || other.Items == nil {
		t.Fatal("record creator/application owner acquired another initiator's instances", other, e)
	}
}
func TestRootPersonalStartedPagesIncludeAcceptedStartingWithoutCallingItCompleted(t *testing.T) {
	f := rootTaskSetup(t, false)
	head := rootCatalogReady(t, f.recordFixture, rootCatalogGraph(t, f.recordFixture, false))
	pending := rootCatalogReserve(t, f.recordFixture, rootCatalogReserveInput(t, f.recordFixture, head))
	q := WorkflowStartedRequest{Page: 1, PageSize: 1}
	a := startedRead(t, f, q)
	q.Page = 2
	q.QueryVersion = a.QueryVersion
	b := startedRead(t, f, q)
	if a.Total != 2 || b.Total != 2 || len(a.Items) != 1 || len(b.Items) != 1 || a.Items[0].Instance.ID == b.Items[0].Instance.ID {
		t.Fatal("lost or duplicated another flow's round", a, b)
	}
	got := map[string]string{a.Items[0].Instance.ID: a.Items[0].Instance.State, b.Items[0].Instance.ID: b.Items[0].Instance.State}
	if got[pending.ID] != "starting" || got[f.instance.ID] != "active" {
		t.Fatal("accepted intent misrepresented as business completion", got)
	}
}
func TestRootPersonalStartedCurrentReadRevocationFiltersBeforeCount(t *testing.T) {
	f := rootTaskSetup(t, true)
	first := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if first.Total != 1 {
		t.Fatal("missing instance")
	}
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all')", f.app)
	eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all'", f.app)
	got, e := f.service.SearchWorkflowStarted(f.ctx, f.principal, WorkflowStartedRequest{Page: 1, QueryVersion: first.QueryVersion})
	if !errors.Is(e, querycontext.ErrChanged) || len(got.Items) != 0 {
		t.Fatal("revoked foreign record remained visible", got, e)
	}
	fresh := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if fresh.Total != 0 || fresh.Items == nil || len(fresh.Items) != 0 {
		t.Fatal("unauthorized count leaked", fresh)
	}
}
func TestRootPersonalStartedVisibleStateChangeRequiresExplicitRefresh(t *testing.T) {
	f := rootTaskSetup(t, false)
	first := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	eventConfirmed(t, f, "agree")
	got, e := f.service.SearchWorkflowStarted(f.ctx, f.principal, WorkflowStartedRequest{Page: 1, QueryVersion: first.QueryVersion})
	if !errors.Is(e, querycontext.ErrChanged) || len(got.Items) != 0 {
		t.Fatal("stale active status survived", got, e)
	}
	fresh := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if fresh.Total != 1 || len(fresh.Items) != 1 || fresh.Items[0].Instance.State != "completed" || fresh.Items[0].Instance.Sequence != 2 {
		t.Fatal("fresh status not confirmed projection", fresh)
	}
}
func TestRootPersonalStartedPurposeSessionAndIrrelevantEditChanges(t *testing.T) {
	f := rootTaskSetup(t, false)
	inbox := inboxSearch(t, f, inboxRequest())
	first := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	_, e := f.service.SearchWorkflowStarted(f.ctx, f.principal, WorkflowStartedRequest{Page: 1, QueryVersion: inbox.QueryVersion})
	if !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("inbox token accepted for initiated list", e)
	}
	p := f.principal
	p.SessionRef = f.other
	_, e = f.service.SearchWorkflowStarted(f.ctx, p, WorkflowStartedRequest{Page: 1, QueryVersion: first.QueryVersion})
	if !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("another Session reused initiated query", e)
	}
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.edit')", f.app)
	eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.edit'", f.app)
	next := startedRead(t, f, WorkflowStartedRequest{Page: 1, QueryVersion: first.QueryVersion})
	if !reflect.DeepEqual(next, first) {
		t.Fatal("unrelated edit authorization changed read projection", next, first)
	}
}
