package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"reflect"
	"sort"
	"strconv"
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

// Construct a second independently registered application before publishing
// its graph. No immutable version or task roster is rewritten to fake access.
func inboxForActor(t *testing.T, actor string) rootTaskFixture {
	t.Helper()
	base := rootCaptureSetup(t)
	f := base.recordFixture
	if actor != "" {
		if _, e := f.owner.Exec(f.ctx, "UPDATE applications.group_members SET user_id=$2 WHERE app_id=$1 AND user_id=$3", f.app, actor, f.actor); e != nil {
			t.Fatal(e)
		}
		f.actor = actor
		f.principal.UserID = actor
	}
	rootTaskKeepDispatchPrivate(t, f)
	graph := rootCatalogGraph(t, f, false)
	h := rootCatalogReady(t, f, graph)
	i := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var version string
	if e := f.owner.QueryRow(f.ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version); e != nil {
		t.Fatal(e)
	}
	p := rootProjection{recordFixture: f, head: h, instance: i, version: version, first: graph.Nodes[1].ID, second: graph.Nodes[1].ID}
	c, payload := rootRecoveryAccept(t, p)
	task := p.task(t, p.first, p.actor, 1)
	receipt, body := p.receipt(t, c, "active", "", task)
	p.apply(t, c, payload, receipt, body, true)
	return rootTaskFixture{rootProjection: p, task: task}
}
func TestRootWorkflowInboxCrossApplicationStableBatchAndHiddenCount(t *testing.T) {
	f := inboxForActor(t, "")
	visible := []string{f.task.ID}
	inboxSQL(t, f, "UPDATE applications.workflow_tasks SET created_at='2026-01-01T00:00:00Z' WHERE id=$1", f.task.ID)
	for i := 0; i < 66; i++ {
		t.Run("prepare-"+strconv.Itoa(i), func(t *testing.T) {
			g := inboxForActor(t, f.actor)
			inboxSQL(t, g, "UPDATE applications.workflow_tasks SET created_at='2026-01-01T00:00:00Z' WHERE id=$1", g.task.ID)
			if i%3 == 0 {
				inboxSQL(t, g, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", g.app)
			} else {
				visible = append(visible, g.task.ID)
			}
		})
	}
	sort.Sort(sort.Reverse(sort.StringSlice(visible)))
	if len(visible) != 45 {
		t.Fatal("fixture count")
	}
	q := WorkflowInboxRequest{Page: 1, PageSize: 20}
	seen := []string{}
	for page := 1; page <= 4; page++ {
		q.Page = page
		r := inboxSearch(t, f, q)
		if r.Total != 45 || r.Items == nil {
			t.Fatal("hidden candidates affected total")
		}
		q.QueryVersion = r.QueryVersion
		for _, v := range r.Items {
			seen = append(seen, v.ID)
		}
		if page == 4 && len(r.Items) != 0 {
			t.Fatal("beyond-total clamped")
		}
	}
	if !reflect.DeepEqual(seen, visible) {
		t.Fatalf("lost, duplicated, sparse or unstable authorized pages: got %v want %v", seen, visible)
	}
}
func TestRootWorkflowInboxPersonalReadOneSnapshot(t *testing.T) {
	f := rootTaskSetup(t, false)
	r, e := (&applications.Application{Pool: f.runtime}).BeginPersonalRead(f.ctx, f.principal)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Rollback(context.Background())
	var iso, ro string
	if e = r.QueryRow(f.ctx, "SHOW transaction_isolation").Scan(&iso); e != nil {
		t.Fatal(e)
	}
	if e = r.QueryRow(f.ctx, "SHOW transaction_read_only").Scan(&ro); e != nil {
		t.Fatal(e)
	}
	if iso != "repeatable read" || ro != "on" {
		t.Fatal("not one read-only RR", iso, ro)
	}
	before, e := r.RecordContext(f.ctx, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	inboxSQL(t, f, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app)
	after, e := r.RecordContext(f.ctx, f.app, f.view)
	if e != nil || !reflect.DeepEqual(before.Grants, after.Grants) || after.Actor.ID != f.actor {
		t.Fatal("transaction mixed snapshots or actor", e)
	}
	if e = r.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	fresh := inboxSearch(t, f, inboxRequest())
	if fresh.Total != 0 {
		t.Fatal("next request ignored live revocation")
	}
}
func TestRootWorkflowInboxClosingKeepsActiveTaskAndPendingNotSuccess(t *testing.T) {
	f := rootTaskSetup(t, false)
	inboxSQL(t, f, "UPDATE applications.workflow_definitions SET state='closing' WHERE app_id=$1 AND id=$2", f.app, f.head.FlowID)
	r := inboxSearch(t, f, inboxRequest())
	if r.Total != 1 {
		t.Fatal("closing removed active work")
	}
	f.accept(t, "agree", f.task.ID, "", 1, 1)
	r = inboxSearch(t, f, inboxRequest())
	if r.Total != 1 || r.Items[0].Sequence != 1 {
		t.Fatal("accepted but unconfirmed command prematurely removed task")
	}
}
func TestRootWorkflowInboxUnavailableDoesNotPublishPartialContext(t *testing.T) {
	f := rootTaskSetup(t, false)
	g := inboxForActor(t, f.actor)
	// A nonexistent physical relation is a real storage failure, never an empty page.
	inboxSQL(t, g, "ALTER TABLE "+rootCaptureRelation(rootEvidenceStoreFixture{recordFixture: g.recordFixture})+" RENAME TO inbox_missing_"+strings.ReplaceAll(g.table, "-", ""))
	r, e := f.service.SearchWorkflowInbox(f.ctx, f.principal, inboxRequest())
	if e == nil || r.QueryVersion != "" || len(r.Items) != 0 || r.Total != 0 {
		t.Fatal("partial success on storage failure", e)
	}
}

func TestRootWorkflowInboxUsesFixedVersionAndRejectsCorruptRoster(t *testing.T) {
	f := rootTaskSetup(t, false)
	graph := rootCatalogGraph(t, f.recordFixture, false)
	graph.Nodes[1].Approval.AssigneeIDs = []string{f.other}
	head := rootCatalogPut(t, f.recordFixture, f.head.FlowID, f.head.Revision, graph)
	head = rootCatalogDeploy(t, f.recordFixture, head)
	rootCatalogEnable(t, f.recordFixture, head)
	r := inboxSearch(t, f, inboxRequest())
	if r.Total != 1 || r.Items[0].DefinitionVersion != 1 || r.Items[0].NodeID != f.first {
		t.Fatal("new publication replaced in-flight roster")
	}
	// Only isolated owner injects a corrupt immutable version; public/runtime
	// writers cannot do this. A damaged accepted roster must fail closed.
	inboxSQL(t, f, "UPDATE applications.workflow_versions SET graph_json='{}'::jsonb WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, f.head.FlowID)
	got, e := f.service.SearchWorkflowInbox(f.ctx, f.principal, inboxRequest())
	if !errors.Is(e, ErrUnavailable) || len(got.Items) != 0 || got.QueryVersion != "" {
		t.Fatal("corrupt fixed version returned a partial queue", e)
	}
}
func TestRootWorkflowInboxUnrelatedOtherUserAndBusinessValues(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := inboxRequest()
	a := inboxSearch(t, f, q)
	q.QueryVersion = a.QueryVersion
	other := rootTaskSetup(t, false)
	inboxSQL(t, other, "UPDATE applications.workflow_definitions SET name='Unrelated' WHERE app_id=$1", other.app)
	relation := rootCaptureRelation(rootEvidenceStoreFixture{recordFixture: f.recordFixture})
	column := rootCaptureColumn(f.public)
	inboxSQL(t, f, "UPDATE "+relation+" SET "+column+"='unrelated business value' WHERE id=$1", f.ownRecord)
	b := inboxSearch(t, f, q)
	if b.QueryVersion != a.QueryVersion || !reflect.DeepEqual(b.Items, a.Items) || b.Total != a.Total {
		t.Fatal("unrelated invisible projection forced refresh")
	}
}
