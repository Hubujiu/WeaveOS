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

func removeStartedRuntimeProjection(t *testing.T, f rootTaskFixture) {
	t.Helper()
	eventSQL(t, f, "DELETE FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", f.app, f.instance.ID)
	tag, e := f.owner.Exec(f.ctx, "DELETE FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 AND state IN ('completed','rejected','withdrawn','no_effect')", f.app, f.instance.ID)
	if e != nil || tag.RowsAffected() != 1 {
		t.Fatal("terminal-only fixture removal failed", tag, e)
	}
	var retained bool
	if e = f.owner.QueryRow(f.ctx, "SELECT EXISTS(SELECT 1 FROM applications.workflow_rounds WHERE instance_id=$1)", f.instance.ID).Scan(&retained); e != nil || !retained {
		t.Fatal("independent round identity was lost", e)
	}
}
func assertArchivedStartedItem(t *testing.T, item WorkflowStartedItem) {
	t.Helper()
	raw, _ := json.Marshal(item)
	var m map[string]json.RawMessage
	json.Unmarshal(raw, &m)
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"appId", "archived", "flowNameSource", "instance", "recordId", "viewId"}) || string(m["archived"]) != "true" || string(m["flowNameSource"]) != `"captured"` {
		t.Fatal("retained round source is not explicit closed metadata", string(raw))
	}
}
func TestRootPersonalStartedRetainsArchivedRoundAndOriginalConfirmedName(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	eventConfirmed(t, f, "reject")
	before := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	removeStartedRuntimeProjection(t, f)
	eventSQL(t, f, "UPDATE applications.workflow_definitions SET name='Changed current catalog' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	eventSQL(t, f, "UPDATE applications.workflow_versions SET graph_json='{}',bpmn_xml='<removed-test-fixture/>' WHERE app_id=$1 AND flow_id=$2", f.app, f.head.FlowID)
	if _, e := f.service.SearchWorkflowStarted(f.ctx, f.principal, WorkflowStartedRequest{Page: 1, QueryVersion: before.QueryVersion}); !errors.Is(e, querycontext.ErrChanged) {
		t.Fatal("archival transition did not invalidate old visible projection", e)
	}
	after := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if after.Total != 1 || len(after.Items) != 1 {
		t.Fatal("retained initiated round disappeared with runtime projection", after)
	}
	v := after.Items[0].Instance
	if v.ID != f.instance.ID || v.InitiatorID != f.actor || v.State != "rejected" || v.Sequence != 2 || v.Name != "Expense" || v.CreatedAt != before.Items[0].Instance.CreatedAt {
		t.Fatal("archive invented current name or state", v)
	}
	assertArchivedStartedItem(t, after.Items[0])
	again := startedRead(t, f, WorkflowStartedRequest{Page: 1, QueryVersion: after.QueryVersion})
	if !reflect.DeepEqual(after, again) {
		t.Fatal("unchanged archive was not stable")
	}
	beyond := startedRead(t, f, WorkflowStartedRequest{Page: 2, PageSize: 1, QueryVersion: after.QueryVersion})
	if beyond.Total != 1 || beyond.Items == nil || len(beyond.Items) != 0 {
		t.Fatal("archive paging not authoritative", beyond)
	}
}
func TestRootPersonalStartedArchiveNeedsCurrentReadAndHistoryBeforeCount(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	eventConfirmed(t, f, "agree")
	removeStartedRuntimeProjection(t, f)
	first := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if first.Total != 1 {
		t.Fatal("readable archive missing", first)
	}
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.history')", f.app)
	eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.history'", f.app)
	if _, e := f.service.SearchWorkflowStarted(f.ctx, f.principal, WorkflowStartedRequest{Page: 1, QueryVersion: first.QueryVersion}); !errors.Is(e, querycontext.ErrChanged) {
		t.Fatal("revoked history remained in archived page", e)
	}
	next := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if next.Total != 0 || len(next.Items) != 0 {
		t.Fatal("initiator status bypassed current history authorization", next)
	}
}
func TestRootPersonalStartedArchiveRejectsCorruptConfirmedResult(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	event := eventConfirmed(t, f, "agree")
	removeStartedRuntimeProjection(t, f)
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT result_bytes FROM applications.workflow_execution_events WHERE command_id=$1", event.EventID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	raw[len(raw)-1] ^= 1
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET result_bytes=$2 WHERE command_id=$1", event.EventID, raw)
	got, e := f.service.SearchWorkflowStarted(f.ctx, f.principal, WorkflowStartedRequest{Page: 1})
	if !errors.Is(e, ErrUnavailable) || len(got.Items) != 0 {
		t.Fatal("damaged archived result silently disappeared or exposed a state", got, e)
	}
}
func TestRootPersonalStartedArchiveNoEffectCannotOverwriteConfirmedTerminal(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	eventConfirmed(t, f, "reject")
	c, p := f.rootProjection.accept(t, "agree", f.task.ID, "", 1, 2)
	r, b := f.receipt(t, c, "unchanged", "task_inactive")
	f.apply(t, c, p, r, b, true)
	removeStartedRuntimeProjection(t, f)
	got := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Instance.State != "rejected" || got.Items[0].Instance.Sequence != 2 {
		t.Fatal("later no-effect replaced confirmed business state", got)
	}
	assertArchivedStartedItem(t, got.Items[0])
}
func TestRootPersonalStartedArchiveCancelledInitialStartRemainsNoEffect(t *testing.T) {
	p := rootProjectionFixture(t)
	c, payload := p.accept(t, "start", "", "", 0, 0)
	r, b := p.receipt(t, c, "unchanged", "cancelled")
	p.apply(t, c, payload, r, b, true)
	f := rootTaskFixture{rootProjection: p}
	eventGrant(t, f, "all", f.public)
	removeStartedRuntimeProjection(t, f)
	got := startedRead(t, f, WorkflowStartedRequest{Page: 1})
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Instance.State != "no_effect" || got.Items[0].Instance.Sequence != 0 {
		t.Fatal("cancelled reservation invented a successful workflow", got)
	}
	assertArchivedStartedItem(t, got.Items[0])
}
