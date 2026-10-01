package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"testing"
	"time"
)

func baselineVersion(t *testing.T, f *webFixture, search string) string {
	t.Helper()
	return memberQueryResponse(t, f.request("POST", "/api/v1/personnel/members/search", queryBody(t, map[string]any{"search": search}), true, true)).QueryVersion
}
func TestQ36HTTPRequiredWriteQueryVersion(t *testing.T) {
	for _, kind := range []string{"member-identities", "member-groups", "department-create", "department-update", "department-delete"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := queryWeb(t)
			target := newTarget(t, f.fixture)
			ctx := context.Background()
			parent := rootDepartment(t, f.fixture)
			d, err := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "query-required", ParentID: parent}, RequestMetadata{RequestID: "q36-required-setup"})
			if err != nil {
				t.Fatal(err)
			}
			cleanDepartment(t, f.fixture, d.ID)
			t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM personnel.departments WHERE name='must-not-create'") })
			method, path, body := "PUT", "/api/v1/personnel/members/"+target+"/identities", map[string]any{"identityIds": []string{f.i2}, "version": 0}
			switch kind {
			case "member-groups":
				method = "POST"
				path = "/api/v1/personnel/members/" + target + "/groups"
				body = map[string]any{"operation": "add", "departmentId": d.ID, "version": 0}
			case "department-create":
				method = "POST"
				path = "/api/v1/personnel/departments"
				body = map[string]any{"name": "must-not-create", "parentId": parent}
			case "department-update":
				path = "/api/v1/personnel/departments/" + d.ID
				body = map[string]any{"name": "must-not-change", "version": 1}
			case "department-delete":
				method = "DELETE"
				path = "/api/v1/personnel/departments/" + d.ID + "?version=1"
				body = nil
			}
			before := queryRevisionsForTest(t, f.owner)
			w := f.request(method, path, queryBody(t, body), true, true)
			queryError(t, w, 400, "COMMON_INVALID_ARGUMENT")
			if queryRevisionsForTest(t, f.owner) != before {
				t.Fatal("missing queryVersion changed business/revisions")
			}
		})
	}
}
func TestQ36HTTPWriteQueryRelatedUnrelatedAndObjectVersion(t *testing.T) {
	f, _ := queryWeb(t)
	ctx := context.Background()
	target := newTarget(t, f.fixture)
	prefix := fmt.Sprintf("q36-guard-%d", time.Now().UnixNano())
	if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", target, prefix); err != nil {
		t.Fatal(err)
	}
	qv := baselineVersion(t, f, prefix)
	if _, err := f.owner.Exec(ctx, "UPDATE personnel.identities SET description='unrelated' WHERE id=$1", f.i2); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"identityIds": []string{f.i2}, "version": 0, "queryVersion": qv}
	path := "/api/v1/personnel/members/" + target + "/identities"
	w := f.request("PUT", path, queryBody(t, body), true, true)
	if w.Code != 200 {
		t.Fatalf("unrelated revision must allow write: %d %s", w.Code, w.Body)
	}
	body["identityIds"] = []string{}
	body["version"] = 1
	before := queryRevisionsForTest(t, f.owner)
	queryError(t, f.request("PUT", path, queryBody(t, body), true, true), 409, "COMMON_QUERY_CHANGED")
	if queryRevisionsForTest(t, f.owner) != before {
		t.Fatal("related query rejection partially wrote")
	}
	body["queryVersion"] = baselineVersion(t, f, prefix)
	body["version"] = 0
	queryError(t, f.request("PUT", path, queryBody(t, body), true, true), 409, "PERSONNEL_CONFLICT")
}
func TestQ36WriteGuardRevalidatesBeforeAnyMutation(t *testing.T) {
	for _, mode := range []string{"unrelated-once", "related", "busy", "revoked", "auth-version"} {
		t.Run(mode, func(t *testing.T) {
			f, trace := queryWeb(t)
			ctx := context.Background()
			target := newTarget(t, f.fixture)
			var account string
			if err := f.owner.QueryRow(ctx, "SELECT account FROM auth.users WHERE id=$1", target).Scan(&account); err != nil {
				t.Fatal(err)
			}
			version := baselineVersion(t, f, account)
			p := f.actor
			p.SessionRef = "11111111-1111-4111-8111-111111111111"
			calls := 0
			trace.mu.Lock()
			trace.beforeRevisionLock = func() {
				calls++
				if calls > 1 && mode != "busy" {
					return
				}
				var err error
				switch mode {
				case "unrelated-once", "busy":
					_, err = f.owner.Exec(ctx, "UPDATE personnel.identities SET description=$2 WHERE id=$1", f.i2, fmt.Sprint(calls))
				case "related":
					_, err = f.owner.Exec(ctx, "UPDATE auth.users SET account=account||'-changed' WHERE id=$1", target)
				case "revoked":
					_, err = f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template)
				case "auth-version":
					_, err = f.owner.Exec(ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor.UserID)
				}
				if err != nil {
					t.Error(err)
				}
			}
			trace.mu.Unlock()
			tx, err := f.app.BeginQueryWrite(ctx, p, version)
			if tx != nil {
				_ = tx.Rollback(ctx)
			}
			switch mode {
			case "unrelated-once":
				if err != nil || calls != 2 {
					t.Fatalf("unrelated race must revalidate once: calls=%d err=%v", calls, err)
				}
			case "related":
				if !errors.Is(err, ErrQueryChanged) {
					t.Fatalf("related race: %v", err)
				}
			case "busy":
				if !errors.Is(err, ErrQueryBusy) || calls != 3 {
					t.Fatalf("three prevalidations then busy: calls=%d err=%v", calls, err)
				}
			case "revoked":
				if !errors.Is(err, ErrDenied) {
					t.Fatalf("recheck current qualification: %v", err)
				}
			case "auth-version":
				if !errors.Is(err, session.ErrUnauthorized) {
					t.Fatalf("original auth_version must be rechecked: %v", err)
				}
			}
		})
	}
}
func TestQ36HTTPBusinessDraftCleanup(t *testing.T) {
	for _, mode := range []string{"exact", "newer", "failed-business", "new-definition"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := queryWeb(t)
			ctx := context.Background()
			target := newTarget(t, f.fixture)
			var account string
			_ = f.owner.QueryRow(ctx, "SELECT account FROM auth.users WHERE id=$1", target).Scan(&account)
			p := f.actor
			p.SessionRef = "11111111-1111-4111-8111-111111111111"
			zero := int64(0)
			in := DraftCreateInput{Kind: DraftMemberIdentities, TargetID: &target, BaseVersion: &zero, Payload: []byte(`{"identityIds":["` + f.i2 + `"]}`)}
			if mode == "new-definition" {
				in = DraftCreateInput{Kind: DraftIdentity, Payload: []byte(`{"name":"from explicit draft","description":"","templateIds":[],"permissionCodes":[]}`)}
			}
			d, err := f.app.CreateDraft(ctx, p, in)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM personnel.drafts WHERE owner_user_id=$1", p.UserID) })
			if mode == "newer" {
				if _, err = f.app.UpdateDraft(ctx, p, d.ID, DraftUpdateInput{Version: d.Version, Payload: []byte(`{"identityIds":[]}`)}); err != nil {
					t.Fatal(err)
				}
			}
			body := map[string]any{"identityIds": []string{f.i2}, "version": 0, "queryVersion": baselineVersion(t, f, account), "draftRef": map[string]any{"id": d.ID, "version": d.Version}}
			method, path := "PUT", "/api/v1/personnel/members/"+target+"/identities"
			want := 200
			if mode == "failed-business" {
				body["version"] = 9
				want = 409
			}
			if mode == "new-definition" {
				method = "POST"
				path = "/api/v1/personnel/identities"
				body = map[string]any{"name": "from explicit draft", "description": "", "permissionCodes": []string{}, "templateIds": []string{}, "draftRef": map[string]any{"id": d.ID, "version": d.Version}}
				want = 201
			}
			w := f.request(method, path, queryBody(t, body), true, true)
			if w.Code != want {
				t.Fatalf("business response %d want %d %s", w.Code, want, w.Body)
			}
			if mode == "new-definition" {
				var e struct{ Data Definition }
				_ = json.Unmarshal(w.Body.Bytes(), &e)
				t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM personnel.identities WHERE id=$1", e.Data.ID) })
			}
			var exists bool
			if err = f.owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM personnel.drafts WHERE id=$1)", d.ID).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists != (mode == "newer" || mode == "failed-business") {
				t.Fatalf("exact committed draft cleanup mismatch; exists=%v", exists)
			}
		})
	}
}

func TestQ36HTTPAuxiliaryReadsValidateOriginalQuery(t *testing.T) {
	for _, resource := range []string{"departments", "identities", "templates", "permissions"} {
		t.Run(resource, func(t *testing.T) {
			f, trace := queryWeb(t)
			ctx := context.Background()
			target := newTarget(t, f.fixture)
			var account string
			if err := f.owner.QueryRow(ctx, "SELECT account FROM auth.users WHERE id=$1", target).Scan(&account); err != nil {
				t.Fatal(err)
			}
			qv := baselineVersion(t, f, account)
			if _, err := f.owner.Exec(ctx, "UPDATE personnel.identities SET description='unrelated auxiliary option' WHERE id=$1", f.i2); err != nil {
				t.Fatal(err)
			}
			path := "/api/v1/personnel/" + resource + "?queryVersion=" + qv
			if resource == "identities" || resource == "templates" {
				path += "&search=unrelated&pageSize=1"
			}
			if w := f.request("GET", path, "", true, false); w.Code != 200 {
				t.Fatalf("unrelated options: %d %s", w.Code, w.Body)
			}
			// Mutation after revision SELECT must not split old-query validation and options across snapshots.
			trace.mu.Lock()
			trace.afterRevision = func() {
				if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET account=account||'-changed' WHERE id=$1", target); err != nil {
					t.Error(err)
				}
			}
			trace.mu.Unlock()
			if w := f.request("GET", path, "", true, false); w.Code != 200 {
				t.Fatalf("same RR read: %d %s", w.Code, w.Body)
			}
			queryError(t, f.request("GET", path, "", true, false), 409, "COMMON_QUERY_CHANGED")
			if w := f.request("GET", "/api/v1/personnel/"+resource, "", true, false); w.Code != 200 {
				t.Fatal("independent collection remains available")
			}
		})
	}
}
func TestQ36HTTPWriteBusyLeavesBusinessAndDraftUntouched(t *testing.T) {
	f, trace := queryWeb(t)
	ctx := context.Background()
	target := newTarget(t, f.fixture)
	qv := baselineVersion(t, f, "")
	zero := int64(0)
	d, err := f.app.CreateDraft(ctx, f.actor, DraftCreateInput{Kind: DraftMemberIdentities, TargetID: &target, BaseVersion: &zero, Payload: []byte(`{"identityIds":[]}`)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM personnel.drafts WHERE id=$1", d.ID) })
	calls := 0
	trace.mu.Lock()
	trace.beforeRevisionLock = func() {
		calls++
		if _, err := f.owner.Exec(ctx, "UPDATE personnel.identities SET description=$2 WHERE id=$1", f.i2, fmt.Sprint(calls)); err != nil {
			t.Error(err)
		}
	}
	trace.mu.Unlock()
	w := f.request("PUT", "/api/v1/personnel/members/"+target+"/identities", queryBody(t, map[string]any{"identityIds": []string{f.i2}, "version": 0, "queryVersion": qv, "draftRef": DraftReference{ID: d.ID, Version: 1}}), true, true)
	queryError(t, w, 503, "COMMON_SERVICE_UNAVAILABLE")
	var envelope struct{ Meta struct{ Reason string } }
	_ = json.Unmarshal(w.Body.Bytes(), &envelope)
	if calls != 3 || envelope.Meta.Reason != "QUERY_BUSY" {
		t.Fatalf("bounded busy response: calls=%d reason=%s", calls, envelope.Meta.Reason)
	}
	var members, audits, drafts int
	if err := f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM personnel.member_identities WHERE user_id=$1),(SELECT count(*) FROM auth.authentication_events WHERE object_id=$1),(SELECT count(*) FROM personnel.drafts WHERE id=$2)`, target, d.ID).Scan(&members, &audits, &drafts); err != nil {
		t.Fatal(err)
	}
	if members != 0 || audits != 0 || drafts != 1 {
		t.Fatal("busy touched business/audit/draft")
	}
}

func TestQ36HTTPDraftReferenceRejectsDuplicateFields(t *testing.T) {
	f, _ := queryWeb(t)
	target := newTarget(t, f.fixture)
	qv := baselineVersion(t, f, "")
	body := fmt.Sprintf(`{"identityIds":[],"version":0,"queryVersion":%q,"draftRef":{"id":%q,"version":1,"version":2}}`, qv, target)
	queryError(t, f.request("PUT", "/api/v1/personnel/members/"+target+"/identities", body, true, true), 400, "COMMON_INVALID_ARGUMENT")
}
