package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"reflect"
	"sort"
	"testing"
)

func historyRequest(f rootTaskFixture, size int) WorkflowEventHistoryRequest {
	return WorkflowEventHistoryRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, PageSize: size}
}
func historyNoEffect(t *testing.T, f rootTaskFixture) string {
	t.Helper()
	c, p := f.rootProjection.accept(t, "agree", f.task.ID, "", 1, 1)
	r, b := f.receipt(t, c, "unchanged", "task_inactive")
	f.apply(t, c, p, r, b, true)
	return c.CommandID
}
func historyRead(t *testing.T, f rootTaskFixture, p session.Principal, q WorkflowEventHistoryRequest) WorkflowEventHistoryResult {
	t.Helper()
	v, e := f.service.ReadWorkflowEventHistory(f.ctx, p, q)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func historyStartID(t *testing.T, f rootTaskFixture) string {
	t.Helper()
	var id string
	if e := f.owner.QueryRow(f.ctx, "SELECT command_id::text FROM applications.workflow_execution_events WHERE instance_id=$1 AND action='start'", f.instance.ID).Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}
func TestRootEventHistoryDeterministicPagesAndSafeSummary(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	ids := []string{historyStartID(t, f)}
	for i := 0; i < 4; i++ {
		ids = append(ids, historyNoEffect(t, f))
	}
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET created_at='2026-01-01T00:00:00Z' WHERE app_id=$1", f.app)
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	q := historyRequest(f, 2)
	seen := []string{}
	for page := 0; page < 3; page++ {
		r := historyRead(t, f, f.principal, q)
		want := 2
		if page == 2 {
			want = 1
		}
		if len(r.Items) != want || r.HasMore != (page < 2) || (r.NextPageToken != nil) != (page < 2) {
			t.Fatalf("wrong page %d %+v", page, r)
		}
		for _, v := range r.Items {
			seen = append(seen, v.ID)
			detail := eventRead(t, f, f.principal, WorkflowEventRequest{f.app, f.view, f.ownRecord, v.ID})
			if !reflect.DeepEqual(v, detail.Event) {
				t.Fatal("summary differs from confirmed detail")
			}
			raw, _ := json.Marshal(v)
			var obj map[string]any
			json.Unmarshal(raw, &obj)
			if len(obj) != 12 {
				t.Fatal("unsafe DTO", obj)
			}
		}
		if r.NextPageToken != nil {
			q.PageToken = *r.NextPageToken
		}
	}
	if !reflect.DeepEqual(seen, ids) {
		t.Fatalf("same-time lost/duplicated events got %v want %v", seen, ids)
	}
}
func TestRootEventHistoryNewEventsDoNotJumpBack(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	old := historyStartID(t, f)
	middle := historyNoEffect(t, f)
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET created_at='2026-01-01T00:00:00Z' WHERE command_id=$1", old)
	eventSQL(t, f, "UPDATE applications.workflow_execution_events SET created_at='2026-01-02T00:00:00Z' WHERE command_id=$1", middle)
	q := historyRequest(f, 1)
	first := historyRead(t, f, f.principal, q)
	if len(first.Items) != 1 || first.Items[0].ID != middle || first.NextPageToken == nil {
		t.Fatal("wrong first cursor")
	}
	latest := historyNoEffect(t, f)
	q.PageToken = *first.NextPageToken
	r := historyRead(t, f, f.principal, q)
	if len(r.Items) != 1 || r.Items[0].ID != old || r.HasMore {
		t.Fatal("new event shifted old page chain", r)
	}
	q.PageToken = ""
	r = historyRead(t, f, f.principal, q)
	if r.Items[0].ID != latest {
		t.Fatal("fresh first page missed event")
	}
}
func TestRootEventHistoryAuthorityAndResourceIsolation(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := historyRequest(f, 20)
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("history-free actor saw events", e)
	}
	eventGrant(t, f, "all", f.public)
	r := historyRead(t, f, f.principal, q)
	if len(r.Items) != 1 {
		t.Fatal("start missing")
	}
	q.RecordID = f.otherRecord
	r = historyRead(t, f, f.principal, q)
	if r.Items == nil || len(r.Items) != 0 || r.HasMore || r.NextPageToken != nil {
		t.Fatal("other record inherited events", r)
	}
	q.RecordID = f.ownRecord
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app)
	eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read'", f.app)
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, q); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("revoked actor read events", e)
	}
}
func TestRootEventHistoryCursorCannotCrossBinding(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	historyNoEffect(t, f)
	q := historyRequest(f, 1)
	first := historyRead(t, f, f.principal, q)
	if first.NextPageToken == nil {
		t.Fatal("no continuation")
	}
	q.PageToken = *first.NextPageToken
	p := f.principal
	p.SessionRef = recordOperationID(t, f.recordFixture)
	p.Record.SessionRef = p.SessionRef
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, p, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("session splice accepted", e)
	}
	bad := q
	bad.PageSize = 2
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("page size splice", e)
	}
	bad = q
	bad.RecordID = f.otherRecord
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("record splice", e)
	}
	bad = q
	bad.PageToken = "bad"
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, bad); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("bad token", e)
	}
	eventSQL(t, f, "UPDATE applications.apps SET policy_revision=policy_revision+1 WHERE id=$1", f.app)
	if _, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatal("policy revision stale cursor", e)
	}
}
func TestRootEventHistoryCorruptLedgerFailsNoPartialPage(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	id := historyNoEffect(t, f)
	eventSQL(t, f, "UPDATE applications.workflow_commands SET command_hash=decode(repeat('00',32),'hex') WHERE command_id=$1", id)
	r, e := f.service.ReadWorkflowEventHistory(f.ctx, f.principal, historyRequest(f, 20))
	if !errors.Is(e, ErrUnavailable) || len(r.Items) != 0 || r.NextPageToken != nil {
		t.Fatal("corrupt ledger partial page", r, e)
	}
}
