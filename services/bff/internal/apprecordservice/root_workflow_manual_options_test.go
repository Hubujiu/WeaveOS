package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
	"testing"
)

func rootManualOptionsRequest(f recordFixture) WorkflowManualOptionsRequest {
	return WorkflowManualOptionsRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, Page: 1}
}
func rootManualOptions(t *testing.T, f recordFixture, q WorkflowManualOptionsRequest) WorkflowManualOptionsResult {
	t.Helper()
	r, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestRootManualOptionsCreatorCanDiscoverAndStartWithoutManagement(t *testing.T) {
	f, req := rootManualSetup(t)
	q := rootManualOptionsRequest(f)
	got := rootManualOptions(t, f, q)
	if got.Total != 1 || len(got.Items) != 1 || got.Page != 1 || got.PageSize != 20 || got.QueryVersion == "" {
		t.Fatalf("missing safe candidate page %+v", got)
	}
	item := got.Items[0]
	if item.FlowID != req.FlowID || item.Name == "" || item.WorkflowRevision != req.ExpectedWorkflowRevision || item.DefinitionVersion != 1 || item.SchemaVersion != 1 || item.RecordVersion != 1 {
		t.Fatalf("wrong candidate %+v", item)
	}
	raw, _ := json.Marshal(item)
	var keys map[string]any
	_ = json.Unmarshal(raw, &keys)
	if len(keys) != 6 {
		t.Fatalf("extra definition details leaked %s", raw)
	}
	rootTriggeredCount(t, f, req.RecordID, 0)
	rootManualNoReceipt(t, f, req.OperationID)
	req.FlowID = item.FlowID
	req.ExpectedWorkflowRevision = item.WorkflowRevision
	req.ExpectedSchemaVersion = item.SchemaVersion
	req.ExpectedRecordVersion = item.RecordVersion
	rootManualStart(t, f, req)
	q.QueryVersion = got.QueryVersion
	if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); !errors.Is(e, querycontext.ErrChanged) {
		t.Fatalf("accepted history did not invalidate options %v", e)
	}
	q.QueryVersion = ""
	if next := rootManualOptions(t, f, q); next.Total != 0 || len(next.Items) != 0 || next.Items == nil {
		t.Fatalf("history remained candidate %+v", next)
	}
}
func TestRootManualOptionsNonCreatorSeesEmptyEvenAsOwner(t *testing.T) {
	f, _ := rootManualSetup(t)
	q := rootManualOptionsRequest(f)
	p := f.principal
	p.UserID = f.other
	p.SessionRef = recordOperationID(t, f)
	got, e := f.service.SearchManualWorkflowOptions(f.ctx, p, q)
	if e != nil || got.Total != 0 || got.Items == nil || len(got.Items) != 0 {
		t.Fatalf("owner bypassed creator options %+v %v", got, e)
	}
	q.RecordID = f.otherRecord
	got = rootManualOptions(t, f, q)
	if got.Total != 0 || got.Items == nil || len(got.Items) != 0 {
		t.Fatalf("reader discovered foreign options %+v", got)
	}
}
func TestRootManualOptionsLiveRevocationWinsInvalidToken(t *testing.T) {
	for _, action := range []string{"menu.enter", "data.read"} {
		t.Run(action, func(t *testing.T) {
			f, _ := rootManualSetup(t)
			q := rootManualOptionsRequest(f)
			first := rootManualOptions(t, f, q)
			if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action=$2)", f.app, action); e != nil {
				t.Fatal(e)
			}
			if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action=$2", f.app, action); e != nil {
				t.Fatal(e)
			}
			for _, token := range []string{first.QueryVersion, "invalid-token"} {
				q.QueryVersion = token
				got, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q)
				if !errors.Is(e, applications.ErrDenied) || len(got.Items) != 0 {
					t.Fatalf("revocation did not win %+v %v", got, e)
				}
			}
		})
	}
}
func TestRootManualOptionsOnlyPublishedMatchingEnabledManual(t *testing.T) {
	f, r := rootManualSetup(t)
	rootConfiguredTrigger(t, f, "record.updated", nil)
	rootConfiguredTrigger(t, f, "manual", []byte(strings.Replace(string(rootTriggerCondition(f.public)), "alpha", "never-match", 1)))
	old := rootConfiguredTrigger(t, f, "record.created", nil)
	cfg := []workflowcatalog.Trigger{{Event: "manual"}}
	in := rootCatalogInput(f, old.FlowID, old.Revision, rootCatalogGraph(t, f, false))
	in.Triggers = &cfg
	rootCatalogTx(t, f, func(tx pgx.Tx) error { _, e := (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in); return e })
	closing := rootConfiguredTrigger(t, f, "manual", nil)
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		_, e := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, closing.FlowID, closing.Revision)
		return e
	})
	got := rootManualOptions(t, f, rootManualOptionsRequest(f))
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].FlowID != r.FlowID {
		t.Fatalf("ineligible configuration exposed %+v", got)
	}
}
func TestRootManualOptionsAllHistoryExcludedButOtherFlowsIndependent(t *testing.T) {
	for _, state := range []string{"starting", "active", "completed", "rejected", "withdrawn", "no_effect"} {
		t.Run(state, func(t *testing.T) {
			f, r := rootManualSetup(t)
			a := rootManualStart(t, f, r)
			if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_instances SET state=$2 WHERE id=$1", a.InstanceID, state); e != nil {
				t.Fatal(e)
			}
			b := rootConfiguredTrigger(t, f, "manual", nil)
			got := rootManualOptions(t, f, rootManualOptionsRequest(f))
			if got.Total != 1 || len(got.Items) != 1 || got.Items[0].FlowID != b.FlowID {
				t.Fatalf("wrong history isolation %+v", got)
			}
		})
	}
}
func TestRootManualOptionsStablePagesAndUnrelatedHistory(t *testing.T) {
	f, r := rootManualSetup(t)
	b := rootConfiguredTrigger(t, f, "manual", nil)
	c := rootConfiguredTrigger(t, f, "manual", nil)
	ids := []string{r.FlowID, b.FlowID, c.FlowID}
	sort.Strings(ids)
	q := rootManualOptionsRequest(f)
	q.PageSize = 1
	first := rootManualOptions(t, f, q)
	q.QueryVersion = first.QueryVersion
	for i, id := range ids {
		q.Page = i + 1
		got := rootManualOptions(t, f, q)
		if got.Total != 3 || len(got.Items) != 1 || got.Items[0].FlowID != id || got.QueryVersion != first.QueryVersion {
			t.Fatalf("page%d %+v want %s", q.Page, got, id)
		}
	}
	// A reservation for a different row must not invalidate these options.
	in := rootCatalogReserveInput(t, f, b)
	in.RecordID = f.otherRecord
	rootCatalogReserve(t, f, in)
	q.Page = 4
	got := rootManualOptions(t, f, q)
	if got.Total != 3 || len(got.Items) != 0 || got.Items == nil {
		t.Fatalf("unrelated history or beyond-last page %+v", got)
	}
}
func TestRootManualOptionsRecordAndDefinitionChangeRequireRefresh(t *testing.T) {
	for _, kind := range []string{"record", "name", "new_flow", "close"} {
		t.Run(kind, func(t *testing.T) {
			f, r := rootManualSetup(t)
			q := rootManualOptionsRequest(f)
			first := rootManualOptions(t, f, q)
			switch kind {
			case "record":
				_, e := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "changed"}}, applications.Metadata{RequestID: "manual-options-edit"})
				if e != nil {
					t.Fatal(e)
				}
			case "name":
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_definitions SET name='new label',revision=revision+1 WHERE id=$1", r.FlowID); e != nil {
					t.Fatal(e)
				}
			case "new_flow":
				rootConfiguredTrigger(t, f, "manual", nil)
			case "close":
				rootCatalogTx(t, f, func(tx pgx.Tx) error {
					_, e := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, r.FlowID, r.ExpectedWorkflowRevision)
					return e
				})
			}
			q.QueryVersion = first.QueryVersion
			if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); !errors.Is(e, querycontext.ErrChanged) {
				t.Fatalf("%s not changed %v", kind, e)
			}
			q.QueryVersion = ""
			got := rootManualOptions(t, f, q)
			if kind == "record" && got.Items[0].RecordVersion != 2 {
				t.Fatal("stale version on explicit refresh")
			}
		})
	}
}
func TestRootManualOptionsRejectForeignTokenBindingsAndInvalidPages(t *testing.T) {
	f, _ := rootManualSetup(t)
	q := rootManualOptionsRequest(f)
	first := rootManualOptions(t, f, q)
	q.QueryVersion = first.QueryVersion
	p := f.principal
	p.SessionRef = recordOperationID(t, f)
	if _, e := f.service.SearchManualWorkflowOptions(f.ctx, p, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatalf("foreign session %v", e)
	}
	q.RecordID = f.otherRecord
	if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatalf("foreign record %v", e)
	}
	q = rootManualOptionsRequest(f)
	q.QueryVersion = "invalid-token"
	if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); e == nil {
		t.Fatal("invalid token silently reset")
	}
	for _, v := range []WorkflowManualOptionsRequest{{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, Page: 0}, {AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, Page: 1, PageSize: 101}} {
		if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, v); e == nil {
			t.Fatal("invalid page accepted")
		}
	}
}

func TestRootManualOptionsOtherViewAndTokenIsolation(t *testing.T) {
	f, r := rootManualSetup(t)
	q := rootManualOptionsRequest(f)
	first := rootManualOptions(t, f, q)
	view := recordOperationID(t, f)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.form_views(id,app_id,table_id,name,position,view_version) VALUES($1,$2,$3,'other options view',1,1)", view, f.app, f.table); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.menu_resources VALUES($1,'form',$2)", f.app, view); e != nil {
		t.Fatal(e)
	}
	var group string
	if e := f.owner.QueryRow(f.ctx, "SELECT group_id::text FROM applications.group_members WHERE app_id=$1 AND user_id=$2 LIMIT 1", f.app, f.actor).Scan(&group); e != nil {
		t.Fatal(e)
	}
	for _, action := range []string{"menu.enter", "data.read"} {
		var grant string
		if e := f.owner.QueryRow(f.ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,$4,'all') RETURNING id::text", f.app, group, view, action).Scan(&grant); e != nil {
			t.Fatal(e)
		}
		if action == "data.read" {
			if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, f.public, f.table); e != nil {
				t.Fatal(e)
			}
		}
	}
	q.ViewID = view
	got := rootManualOptions(t, f, q)
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].FlowID != r.FlowID {
		t.Fatalf("same-table other view lost flow %+v", got)
	}
	q.QueryVersion = first.QueryVersion
	if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatalf("cross-view token allowed %v", e)
	}
	history, e := f.service.SearchRecordWorkflows(f.ctx, f.principal, WorkflowInstanceSearchRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, Page: 1})
	if e != nil {
		t.Fatal(e)
	}
	q = rootManualOptionsRequest(f)
	q.QueryVersion = history.QueryVersion
	if _, e = f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatalf("history namespace token allowed %v", e)
	}
}
func TestRootManualOptionsOwnScopeAndFieldMaskAreLive(t *testing.T) {
	for _, kind := range []string{"own", "empty_fields"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := rootManualSetup(t)
			q := rootManualOptionsRequest(f)
			if kind == "own" {
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all')", f.app); e != nil {
					t.Fatal(e)
				}
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all'", f.app); e != nil {
					t.Fatal(e)
				}
				q.RecordID = f.otherRecord
				if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); !errors.Is(e, applications.ErrMissing) {
					t.Fatalf("own read leaked other record %v", e)
				}
			} else {
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app); e != nil {
					t.Fatal(e)
				}
				if _, e := f.service.SearchManualWorkflowOptions(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
					t.Fatalf("no readable fields still got options %v", e)
				}
			}
		})
	}
}
func TestRootManualOptionsReadHasNoBusinessSideEffects(t *testing.T) {
	f, _ := rootManualSetup(t)
	counts := func() [4]int64 {
		t.Helper()
		var c [4]int64
		for i, table := range []string{"operations", "record_change_events", "workflow_instances", "workflow_commands"} {
			where := "app_id=$1"
			if table == "workflow_commands" {
				where = "command_json->>'AppID'=$1"
			}
			if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE "+where, f.app).Scan(&c[i]); e != nil {
				t.Fatal(e)
			}
		}
		return c
	}
	before := counts()
	q := rootManualOptionsRequest(f)
	first := rootManualOptions(t, f, q)
	q.QueryVersion = first.QueryVersion
	rootManualOptions(t, f, q)
	if got := counts(); got != before {
		t.Fatalf("read mutated business state before%v after%v", before, got)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	var v int64
	if e := f.owner.QueryRow(f.ctx, "SELECT record_version FROM "+relation+" WHERE id=$1", f.ownRecord).Scan(&v); e != nil || v != 1 {
		t.Fatalf("read changed record %d %v", v, e)
	}
}
