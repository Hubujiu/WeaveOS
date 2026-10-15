package apprecordservice

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

func filterCandidateRequest(f recordFixture) FilterCandidatesRequest {
	return FilterCandidatesRequest{AppID: f.app, ViewID: f.view, FieldID: f.reference, PageSize: 20}
}
func filterExec(t *testing.T, f recordFixture, sql string, args ...any) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, sql, args...); e != nil {
		t.Fatal(e)
	}
}
func filterReference(t *testing.T, f recordFixture, record, value string) {
	t.Helper()
	rel := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	col := pgx.Identifier{"f_" + strings.ReplaceAll(f.reference, "-", "")}.Sanitize()
	filterExec(t, f, "UPDATE "+rel+" SET "+col+"=$1 WHERE id=$2", value, record)
}
func filterReadOnly(t *testing.T, f recordFixture) {
	t.Helper()
	filterExec(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action IN ('data.create','data.edit'))", f.app)
	filterExec(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action IN ('data.create','data.edit')", f.app)
}
func filterTwo(t *testing.T, f recordFixture) FilterCandidatesRequest {
	t.Helper()
	filterReference(t, f, f.ownRecord, f.actor)
	filterExec(t, f, "UPDATE applications.member_sources SET label=CASE WHEN id=$1 THEN 'A' ELSE 'B' END WHERE id IN ($1,$2)", f.actor, f.other)
	req := filterCandidateRequest(f)
	req.PageSize = 1
	return req
}
func TestRootFilterCandidatesReadOnlyDeduplicatesActualValuesWithoutBusinessWrites(t *testing.T) {
	f := newRecordFixture(t)
	filterReadOnly(t, f)
	var label string
	if e := f.owner.QueryRow(f.ctx, "SELECT label FROM applications.member_sources WHERE id=$1", f.other).Scan(&label); e != nil {
		t.Fatal(e)
	}
	var before string
	snapshot := `SELECT json_build_array((SELECT count(*) FROM applications.operations WHERE app_id=$1),(SELECT count(*) FROM applications.record_write_audit WHERE app_id=$1),(SELECT data_revision FROM applications.logical_tables WHERE id=$2))::text`
	if e := f.owner.QueryRow(f.ctx, snapshot, f.app, f.table).Scan(&before); e != nil {
		t.Fatal(e)
	}
	got, e := f.service.FilterCandidates(f.ctx, f.principal, filterCandidateRequest(f))
	if e != nil || !reflect.DeepEqual(got.Items, []ReferenceDisplay{{ID: f.other, Label: label, Deleted: false}}) || got.HasMore || got.NextPageToken != "" {
		t.Fatalf("read-only actual distinct values: %+v %v", got, e)
	}
	raw, _ := json.Marshal(got)
	var object map[string]json.RawMessage
	_ = json.Unmarshal(raw, &object)
	if len(object) != 1 || object["items"] == nil {
		t.Fatalf("unexpected result projection %s", raw)
	}
	var after string
	if e = f.owner.QueryRow(f.ctx, snapshot, f.app, f.table).Scan(&after); e != nil || after != before {
		t.Fatalf("candidate read changed business state %s/%s %v", before, after, e)
	}
}
func TestRootFilterCandidatesOwnScopeExcludesOtherRowsAndRequiresWholeVisibleFieldCoverage(t *testing.T) {
	f := newRecordFixture(t)
	filterReference(t, f, f.ownRecord, f.actor)
	filterExec(t, f, "UPDATE applications.grants SET row_scope='own' WHERE app_id=$1 AND action='data.read'", f.app)
	got, e := f.service.FilterCandidates(f.ctx, f.principal, filterCandidateRequest(f))
	if e != nil || len(got.Items) != 1 || got.Items[0].ID != f.actor {
		t.Fatalf("own candidate scope: %+v %v", got, e)
	}
	// Add all-row visibility using a different field, without widening reference read.
	var grant string
	if e = f.owner.QueryRow(f.ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) SELECT app_id,id,'form',$2,'data.read','all' FROM applications.permission_groups WHERE app_id=$1 RETURNING id::text", f.app, f.view).Scan(&grant); e != nil {
		t.Fatal(e)
	}
	filterExec(t, f, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, f.public, f.table)
	if _, e = f.service.FilterCandidates(f.ctx, f.principal, filterCandidateRequest(f)); !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("partial-field scope must reject, not narrow rows: %v", e)
	}
}
func TestRootFilterCandidatesTombstoneDisabledAndDeletedRecords(t *testing.T) {
	f := newRecordFixture(t)
	filterReference(t, f, f.ownRecord, f.actor)
	filterExec(t, f, "UPDATE applications.member_sources SET status='deleted',label='原成员' WHERE id=$1", f.actor)
	filterExec(t, f, "UPDATE applications.member_sources SET status='disabled',label='停用成员' WHERE id=$1", f.other)
	got, e := f.service.FilterCandidates(f.ctx, f.principal, filterCandidateRequest(f))
	if e != nil || len(got.Items) != 2 {
		t.Fatalf("visible historical values %+v %v", got, e)
	}
	seen := map[string]ReferenceDisplay{}
	for _, v := range got.Items {
		seen[v.ID] = v
	}
	if seen[f.actor] != (ReferenceDisplay{ID: f.actor, Label: "原成员", Deleted: true}) || seen[f.other].Deleted {
		t.Fatalf("registry display semantics %+v", seen)
	}
	if _, e = f.service.ChangeRecordLifecycle(f.ctx, lifecycleManager(f), lifecycleRequest(t, f, 1, true)); e != nil {
		t.Fatal(e)
	}
	got, e = f.service.FilterCandidates(f.ctx, f.principal, filterCandidateRequest(f))
	if e != nil || len(got.Items) != 1 || got.Items[0].ID != f.other {
		t.Fatalf("logically deleted record supplied candidate %+v %v", got, e)
	}
}
func TestRootFilterCandidatesMissingRegistryIsFailureEvenWhenSearchWouldHideIt(t *testing.T) {
	f := newRecordFixture(t)
	filterExec(t, f, "DELETE FROM applications.member_sources WHERE id=$1", f.other)
	req := filterCandidateRequest(f)
	req.Q = "not-a-matching-label"
	if _, e := f.service.FilterCandidates(f.ctx, f.principal, req); !errors.Is(e, ErrUnavailable) {
		t.Fatalf("missing registry must not silently drop values: %v", e)
	}
}
func TestRootFilterCandidatesOpaqueCursorScopeAndLiveRevocation(t *testing.T) {
	f := newRecordFixture(t)
	req := filterTwo(t, f)
	first, e := f.service.FilterCandidates(f.ctx, f.principal, req)
	if e != nil || len(first.Items) != 1 || first.Items[0].ID != f.actor || !first.HasMore || len(first.NextPageToken) != 43 {
		t.Fatalf("stable first page %+v %v", first, e)
	}
	req.PageToken = first.NextPageToken
	second, e := f.service.FilterCandidates(f.ctx, f.principal, req)
	if e != nil || len(second.Items) != 1 || second.Items[0].ID != f.other || second.HasMore || second.NextPageToken != "" {
		t.Fatalf("stable next page %+v %v", second, e)
	}
	for _, change := range []func(*FilterCandidatesRequest, *session.Principal){func(r *FilterCandidatesRequest, p *session.Principal) { r.Q = "B" }, func(r *FilterCandidatesRequest, p *session.Principal) { p.SessionRef = f.otherRecord }, func(r *FilterCandidatesRequest, p *session.Principal) { p.UserID = f.other }} {
		r, p := req, f.principal
		change(&r, &p)
		if result, e := f.service.FilterCandidates(f.ctx, p, r); e == nil || len(result.Items) > 0 {
			t.Fatalf("cursor crossed original scope %+v %v", result, e)
		}
	}
	filterExec(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app)
	filterExec(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read'", f.app)
	req.PageToken = "not-a-valid-token"
	if _, e = f.service.FilterCandidates(f.ctx, f.principal, req); !errors.Is(e, applications.ErrDenied) {
		t.Fatalf("authorization must precede cursor lookup: %v", e)
	}
}
func TestRootFilterCandidatesVersionChangeRequiresExplicitRestart(t *testing.T) {
	f := newRecordFixture(t)
	req := filterTwo(t, f)
	first, e := f.service.FilterCandidates(f.ctx, f.principal, req)
	if e != nil || !first.HasMore {
		t.Fatalf("first page %+v %v", first, e)
	}
	filterExec(t, f, "UPDATE applications.logical_tables SET data_revision=data_revision+1 WHERE id=$1", f.table)
	req.PageToken = first.NextPageToken
	if _, e = f.service.FilterCandidates(f.ctx, f.principal, req); !errors.Is(e, querycontext.ErrChanged) {
		t.Fatalf("mixed old/new candidate pages: %v", e)
	}
	req.PageToken = ""
	if _, e = f.service.FilterCandidates(f.ctx, f.principal, req); e != nil {
		t.Fatalf("explicit restart rejected: %v", e)
	}
}

func TestRootFilterCandidatesLiteralPrefixNullAndDepartmentValues(t *testing.T) {
	f := newRecordFixture(t)
	filterReference(t, f, f.ownRecord, f.actor)
	filterExec(t, f, "UPDATE applications.member_sources SET label=CASE WHEN id=$1 THEN 'A%_原文' ELSE 'AX其他' END WHERE id IN ($1,$2)", f.actor, f.other)
	req := filterCandidateRequest(f)
	req.Q = "\u2003A%_\u2003"
	got, e := f.service.FilterCandidates(f.ctx, f.principal, req)
	if e != nil || len(got.Items) != 1 || got.Items[0].ID != f.actor {
		t.Fatalf("literal Unicode-trimmed prefix %+v %v", got, e)
	}
	// The same real UUID column is now a department field in this isolated fixture.
	// Register explicit department sources; do not use organisation traversal.
	filterExec(t, f, "UPDATE applications.fields SET definition=jsonb_set(definition,'{kind}','\"department\"'::jsonb) WHERE id=$1", f.reference)
	filterExec(t, f, "INSERT INTO applications.department_sources(id,label,status) VALUES($1,'部门一','deleted'),($2,'部门二','active')", f.actor, f.other)
	rel := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	col := pgx.Identifier{"f_" + strings.ReplaceAll(f.reference, "-", "")}.Sanitize()
	filterExec(t, f, "UPDATE "+rel+" SET "+col+"=NULL WHERE id=$1", f.otherRecord)
	req.Q = ""
	got, e = f.service.FilterCandidates(f.ctx, f.principal, req)
	if e != nil || !reflect.DeepEqual(got.Items, []ReferenceDisplay{{ID: f.actor, Label: "部门一", Deleted: true}}) {
		t.Fatalf("department projection/null exclusion %+v %v", got, e)
	}
}
func TestRootFilterCandidatesRejectsInvalidFieldAndInput(t *testing.T) {
	f := newRecordFixture(t)
	for _, mutate := range []func(*FilterCandidatesRequest){func(r *FilterCandidatesRequest) { r.FieldID = f.public }, func(r *FilterCandidatesRequest) { r.FieldID = f.ownRecord }, func(r *FilterCandidatesRequest) { r.PageSize = 51 }, func(r *FilterCandidatesRequest) { r.PageSize = -1 }, func(r *FilterCandidatesRequest) { r.Q = strings.Repeat("😀", 101) }, func(r *FilterCandidatesRequest) { r.Q = "a\x00b" }, func(r *FilterCandidatesRequest) { r.Q = string([]byte{255}) }} {
		req := filterCandidateRequest(f)
		mutate(&req)
		if _, e := f.service.FilterCandidates(f.ctx, f.principal, req); !errors.Is(e, applications.ErrInvalid) {
			t.Fatalf("invalid input not refused: %+v %v", req, e)
		}
	}
}
