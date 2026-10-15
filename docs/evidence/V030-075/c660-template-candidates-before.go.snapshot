package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

const templateMembers = "/api/v1/application-templates/member-candidates"
const templateDepartments = "/api/v1/application-templates/department-candidates"

func rootTemplateCandidatePage(t *testing.T, f *rootTaskHTTPFixture, path string, edit func(*http.Request)) ([]map[string]any, string) {
	t.Helper()
	w := f.call(t, "GET", path, "", edit)
	data := rootHTTPData(t, w, 200)
	if len(data) != 1 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Request-Id") == "" {
		t.Fatal("candidate envelope is not minimal and private")
	}
	var items []map[string]any
	if err := json.Unmarshal(data["items"], &items); err != nil || items == nil {
		t.Fatal("candidate items are not an explicit array", err)
	}
	var wire struct {
		Meta map[string]json.RawMessage `json:"meta"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Meta) != 2 || wire.Meta["requestId"] == nil {
		t.Fatal("candidate metadata changed")
	}
	var page map[string]any
	if err := json.Unmarshal(wire.Meta["pagination"], &page); err != nil || len(page) != 2 {
		t.Fatal("wrong cursor page", err)
	}
	token, _ := page["nextPageToken"].(string)
	if page["hasMore"] != (token != "") || token == "" && page["nextPageToken"] != nil {
		t.Fatal("contradictory cursor page")
	}
	return items, token
}
func rootTemplateCandidateMembers(t *testing.T, f *rootTaskHTTPFixture) string {
	t.Helper()
	prefix := "tmpl" + strings.ReplaceAll(f.id(t), "-", "")
	for i, user := range []string{f.actor, f.other} {
		name := prefix + []string{"a", "b"}[i]
		if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET account=$1 WHERE id=$2", name, user); e != nil {
			t.Fatal(e)
		}
	}
	return prefix
}
func TestRootTemplateCandidatesCurrentCreatorMinimalKeysetNoBusinessWrites(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	rootTemplateGrantCreate(t, f, f.actor)
	prefix := rootTemplateCandidateMembers(t, f)
	before := rootTemplateReadCounts(t, f)
	first, token := rootTemplateCandidatePage(t, f, templateMembers+"?q="+prefix+"&pageSize=1", nil)
	if len(first) != 1 || len(first[0]) != 3 || first[0]["id"] != f.actor || first[0]["label"] != prefix+"a" || first[0]["status"] != "active" || token == "" {
		t.Fatal("wrong first minimal candidate page", first)
	}
	second, next := rootTemplateCandidatePage(t, f, templateMembers+"?q="+prefix+"&pageSize=1&pageToken="+url.QueryEscape(token), nil)
	if len(second) != 1 || second[0]["id"] != f.other || next != "" {
		t.Fatal("unstable or repeated next page", second)
	}
	if !reflect.DeepEqual(before, rootTemplateReadCounts(t, f)) {
		t.Fatal("candidate discovery wrote business state")
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.other); e != nil {
		t.Fatal(e)
	}
	items, _ := rootTemplateCandidatePage(t, f, templateMembers+"?q="+prefix+"b", nil)
	if len(items) != 0 {
		t.Fatal("inactive mapping target exposed")
	}
}
func TestRootTemplateDepartmentCandidatesCurrentParentAndPaging(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	rootTemplateGrantCreate(t, f, f.actor)
	a, b := f.id(t), f.id(t)
	var parent string
	if e := f.owner.QueryRow(f.ctx, "SELECT id::text FROM personnel.departments WHERE is_root").Scan(&parent); e != nil {
		t.Fatal(e)
	}
	for i, dept := range []string{a, b} {
		if _, e := f.owner.Exec(f.ctx, "INSERT INTO personnel.departments(id,parent_id,name) VALUES($1,$2,$3)", dept, parent, []string{"Tmpl Department A", "Tmpl Department B"}[i]); e != nil {
			t.Fatal(e)
		}
	}
	first, token := rootTemplateCandidatePage(t, f, templateDepartments+"?q=Tmpl%20Department&pageSize=1", nil)
	if len(first) != 1 || len(first[0]) != 4 || first[0]["id"] != a || first[0]["parentId"] != parent || first[0]["status"] != "active" || token == "" {
		t.Fatal("department projection changed", first)
	}
	second, next := rootTemplateCandidatePage(t, f, templateDepartments+"?q=Tmpl%20Department&pageSize=1&pageToken="+url.QueryEscape(token), nil)
	if len(second) != 1 || second[0]["id"] != b || next != "" {
		t.Fatal("department next page changed", second)
	}
}
func TestRootTemplateCandidatesAlwaysRequireLiveCreationAuthority(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	rootTemplateHTTPError(t, f.call(t, "GET", templateMembers, "", nil), 403)
	identity := rootTemplateGrantCreate(t, f, f.actor)
	rootTemplateCandidatePage(t, f, templateMembers, nil)
	if _, e := f.owner.Exec(f.ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity); e != nil {
		t.Fatal(e)
	}
	rootTemplateHTTPError(t, f.call(t, "GET", templateMembers, "", nil), 403)
	rootTemplateCurrentBootstrap(t, f, f.actor)
	rootTemplateCandidatePage(t, f, templateMembers, nil)
	if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor); e != nil {
		t.Fatal(e)
	}
	rootTemplateHTTPError(t, f.call(t, "GET", templateMembers, "", nil), 401)
}
func TestRootTemplateCandidateCursorCannotCrossKindQueryActorOrPurpose(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	rootTemplateGrantCreate(t, f, f.actor)
	prefix := rootTemplateCandidateMembers(t, f)
	_, token := rootTemplateCandidatePage(t, f, templateMembers+"?q="+prefix+"&pageSize=1", nil)
	if token == "" {
		t.Fatal("fixture did not produce pagination")
	}
	suffix := "?q=" + prefix + "&pageSize=1&pageToken=" + url.QueryEscape(token)
	for _, path := range []string{templateDepartments + suffix, templateMembers + strings.Replace(suffix, "q="+prefix, "q=changed", 1), "/api/v1/applications/" + f.app + "/member-candidates" + suffix} {
		rootTemplateHTTPError(t, f.call(t, "GET", path, "", nil), 400)
	}
	rootTemplateGrantCreate(t, f, f.other)
	sid, csrf, e := f.store.Create(f.ctx, session.Record{UserID: f.other, SessionRef: f.id(t), AuthVersion: "1"})
	if e != nil {
		t.Fatal(e)
	}
	defer f.store.Revoke(f.ctx, sid)
	rootTemplateHTTPError(t, f.call(t, "GET", templateMembers+suffix, "", func(r *http.Request) {
		r.Header.Del("Cookie")
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
		r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
	}), 400)
	_, normal := rootTemplateCandidatePage(t, f, "/api/v1/applications/"+f.app+"/member-candidates?q="+prefix+"&pageSize=1", nil)
	rootTemplateHTTPError(t, f.call(t, "GET", templateMembers+"?q="+prefix+"&pageSize=1&pageToken="+url.QueryEscape(normal), "", nil), 400)
}
func TestRootTemplateCandidateHTTPBoundaries(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	rootTemplateGrantCreate(t, f, f.actor)
	for _, q := range []string{"?q=a&q=b", "?pageSize=0", "?pageSize=51", "?pageSize=01", "?pageSize=", "?unknown=true", "?q=%ff", "?q=%00", "?q=" + strings.Repeat("a", 101), "?pageToken=forged"} {
		rootTemplateHTTPError(t, f.call(t, "GET", templateMembers+q, "", nil), 400)
	}
	rootTemplateHTTPError(t, f.call(t, "GET", templateMembers, "", func(r *http.Request) { r.Header.Del("Cookie") }), 401)
	rootTemplateHTTPError(t, f.call(t, "GET", templateMembers, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }), 409)
	for _, path := range []string{templateMembers, templateDepartments} {
		w := f.call(t, "HEAD", path, "", nil)
		if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("HEAD must succeed without a body", w.Code)
		}
		w = f.call(t, "POST", path, "{}", nil)
		rootTemplateHTTPError(t, w, 405)
		if w.Header().Get("Allow") != "GET, HEAD" {
			t.Fatal("wrong method policy")
		}
	}
}
