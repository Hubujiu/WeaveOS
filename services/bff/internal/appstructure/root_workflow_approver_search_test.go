package appstructure

// P16 source: V020 FLOW-06, V075 P16, Figma483:3500. Real current grants, PG, Redis and Session.
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appworkflows"
	"github.com/redis/go-redis/v9"
)

func rootApproverHandler(t *testing.T, s rootWorkflowManagementFixture) http.Handler {
	t.Helper()
	options, e := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(options)
	gen := "root-approvers-" + s.f.actor
	if e = client.Ping(context.Background()).Err(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		defer client.Close()
		var cursor uint64
		for {
			keys, next, e := client.Scan(context.Background(), cursor, "ems:workflow-approvers:query:"+gen+":*", 100).Result()
			if e != nil {
				t.Error(e)
				return
			}
			if len(keys) > 0 {
				if e = client.Del(context.Background(), keys...).Err(); e != nil {
					t.Error(e)
					return
				}
			}
			cursor = next
			if cursor == 0 {
				return
			}
		}
	})
	return &appworkflows.Service{Application: &appworkflows.Application{Pool: s.f.runtime, Limits: s.f.service.Application.Limits, ApproverQueries: appworkflows.NewApproverQueryStore(client, gen)}, Authenticator: s.f.service.Authenticator}
}
func rootApproverPath(s rootWorkflowManagementFixture) string {
	return "/api/v1/applications/" + s.f.app + "/forms/" + s.view + "/workflows/approvers/search"
}
func rootApproverCall(t *testing.T, s rootWorkflowManagementFixture, h http.Handler, page, size int, version string) *httptest.ResponseRecorder {
	t.Helper()
	raw, e := json.Marshal(rootManagerSearchBody(page, size, version))
	if e != nil {
		t.Fatal(e)
	}
	return rootWorkflowHTTPRequest(t, s, h, "POST", rootApproverPath(s), string(raw), true, true, "", nil)
}
func rootApproverIDs(t *testing.T, out map[string]any) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, raw := range rootManagerSearchItems(t, out) {
		item := raw.(map[string]any)
		if len(item) != 4 || item["status"] != "active" || item["label"] == "" {
			t.Fatalf("leaky or incomplete candidate %+v", item)
		}
		id, ok := item["id"].(string)
		if !ok || ids[id] {
			t.Fatalf("invalid duplicate candidate %+v", item)
		}
		ids[id] = true
	}
	return ids
}
func TestRootApproverDiscoveryCurrentTupleEligibilityAndClosedResponse(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	own, _ := rootWorkflowHTTPCandidate(t, s, "own")
	all, _ := rootWorkflowHTTPCandidate(t, s, "all")
	none, _ := rootWorkflowHTTPCandidate(t, s, "")
	inactive, _ := rootWorkflowHTTPCandidate(t, s, "all")
	disabled, group := rootWorkflowHTTPCandidate(t, s, "all")
	noMenu, menuGroup := rootWorkflowHTTPCandidate(t, s, "all")
	noFields, fieldsGroup := rootWorkflowHTTPCandidate(t, s, "own")
	c := context.Background()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"UPDATE auth.users SET status='disabled' WHERE id=$1", []any{inactive}},
		{"UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1 AND id=$2", []any{s.f.app, group}},
		{"DELETE FROM applications.grants WHERE app_id=$1 AND group_id=$2 AND action='menu.enter'", []any{s.f.app, menuGroup}},
		{"DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND group_id=$2 AND action='data.read')", []any{s.f.app, fieldsGroup}},
	} {
		if _, e := s.f.owner.Exec(c, q.sql, q.args...); e != nil {
			t.Fatal(e)
		}
	}
	var before int
	if e := s.f.owner.QueryRow(c, "SELECT count(*) FROM applications.operations WHERE app_id=$1", s.f.app).Scan(&before); e != nil {
		t.Fatal(e)
	}
	response := rootApproverCall(t, s, h, 1, 100, "")
	out := data(t, response, 200)
	if len(out) != 7 || out["page"] != float64(1) || out["pageSize"] != float64(100) || out["schemaVersion"] != float64(1) || out["policyRevision"] == nil || out["queryVersion"] == "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("discovery receipt %+v", out)
	}
	ids := rootApproverIDs(t, out)
	for _, id := range []string{s.f.actor, own, all} {
		if !ids[id] {
			t.Fatalf("eligible missing %s %+v", id, out)
		}
	}
	for _, id := range []string{none, inactive, disabled, noMenu, noFields} {
		if ids[id] {
			t.Fatalf("ineligible exposed %s %+v", id, out)
		}
	}
	var after int
	if e := s.f.owner.QueryRow(c, "SELECT count(*) FROM applications.operations WHERE app_id=$1", s.f.app).Scan(&after); e != nil || before != after {
		t.Fatal("read created an operation", e, before, after)
	}
}
func TestRootApproverDiscoveryCurrentRevocationChangesOldQuery(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	id, group := rootWorkflowHTTPCandidate(t, s, "own")
	first := data(t, rootApproverCall(t, s, h, 1, 100, ""), 200)
	if !rootApproverIDs(t, first)[id] {
		t.Fatal("own candidate missing")
	}
	token := first["queryVersion"].(string)
	if _, e := s.f.owner.Exec(context.Background(), "DELETE FROM applications.group_members WHERE app_id=$1 AND group_id=$2 AND user_id=$3", s.f.app, group, id); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootApproverCall(t, s, h, 1, 100, token), 409, "COMMON_QUERY_CHANGED")
	fresh := data(t, rootApproverCall(t, s, h, 1, 100, ""), 200)
	if rootApproverIDs(t, fresh)[id] {
		t.Fatal("revoked candidate survived refresh")
	}
}
func TestRootApproverDiscoveryStablePagesAndSessionIsolation(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	rootWorkflowHTTPCandidate(t, s, "all")
	rootWorkflowHTTPCandidate(t, s, "own")
	first := data(t, rootApproverCall(t, s, h, 1, 1, ""), 200)
	token := first["queryVersion"].(string)
	second := data(t, rootApproverCall(t, s, h, 2, 1, token), 200)
	if first["total"] != second["total"] || second["queryVersion"] != token || reflect.DeepEqual(first["items"], second["items"]) {
		t.Fatalf("page identity %+v %+v", first, second)
	}
	raw, _ := json.Marshal(rootManagerSearchBody(2, 1, token))
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", rootApproverPath(s), string(raw), true, true, s.f.actor, nil), 409, "COMMON_QUERY_CONTEXT_EXPIRED")
	empty := data(t, rootApproverCall(t, s, h, 100, 100, token), 200)
	if len(rootManagerSearchItems(t, empty)) != 0 {
		t.Fatal("out of range page invented candidates")
	}
}
func TestRootApproverDiscoveryManagerSessionAndReadBoundary(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	other, _ := rootWorkflowHTTPCandidate(t, s, "all")
	path := rootApproverPath(s)
	raw := `{"page":1,"pageSize":20}`
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, false, false, "", nil), 401, "AUTH_UNAUTHENTICATED")
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, true, false, "", nil), 403, "COMMON_CSRF_REJECTED")
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, raw, true, true, other, nil), 403, "APPLICATION_FORBIDDEN")
	for _, body := range []string{`{}`, `{"page":1,"pageSize":0}`, `{"page":1,"pageSize":101}`, `{"page":1,"pageSize":20,"scope":"all"}`, `{"page":1,"page":2,"pageSize":20}`, `{"page":1,"pageSize":20,"queryVersion":null}`} {
		rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path, body, true, true, "", nil), 400, "COMMON_INVALID_ARGUMENT")
	}
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "GET", path, "", true, false, "", nil), 404, "API_NOT_FOUND")
	rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", path+"?scope=all", raw, true, true, "", nil), 400, "COMMON_INVALID_ARGUMENT")
	if _, e := s.f.owner.Exec(context.Background(), "UPDATE personnel.permission_catalog SET enabled=false WHERE app_id=$1 AND category='application'", s.f.app); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootApproverCall(t, s, h, 1, 20, "invalid"), 403, "APPLICATION_FORBIDDEN")
}

// P16 extension oracle: source-approved account/current-department prefix, not page-local filtering.
func rootApproverTextCall(t *testing.T, s rootWorkflowManagementFixture, h http.Handler, page, size int, query, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := rootManagerSearchBody(page, size, token)
	body["q"] = query
	raw, e := json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	return rootWorkflowHTTPRequest(t, s, h, "POST", rootApproverPath(s), string(raw), true, true, "", nil)
}
func rootApproverTestDepartment(t *testing.T, s rootWorkflowManagementFixture, name string, users ...string) string {
	t.Helper()
	c := context.Background()
	id := uuid(t, s.f.owner)
	var parent string
	if e := s.f.owner.QueryRow(c, "SELECT id::text FROM personnel.departments WHERE is_root").Scan(&parent); e != nil {
		t.Fatal(e)
	}
	if _, e := s.f.owner.Exec(c, "INSERT INTO personnel.departments(id,parent_id,name) VALUES($1,$2,$3)", id, parent, name); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if _, e := s.f.owner.Exec(c, "DELETE FROM personnel.department_members WHERE department_id=$1", id); e != nil {
			t.Error(e)
		}
		if _, e := s.f.owner.Exec(c, "DELETE FROM personnel.departments WHERE id=$1", id); e != nil {
			t.Error(e)
		}
	})
	for _, user := range users {
		if _, e := s.f.owner.Exec(c, "INSERT INTO personnel.department_members(department_id,user_id) VALUES($1,$2)", id, user); e != nil {
			t.Fatal(e)
		}
	}
	return id
}
func TestRootApproverTextSearchUsesCurrentDepartmentsAndAuthorizedDistinctUsers(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	var account string
	if e := s.f.owner.QueryRow(context.Background(), "SELECT account FROM auth.users WHERE id=$1", s.f.actor).Scan(&account); e != nil {
		t.Fatal(e)
	}
	own := data(t, rootApproverTextCall(t, s, h, 1, 100, account, ""), 200)
	if !rootApproverIDs(t, own)[s.f.actor] {
		t.Fatal("full account prefix did not find owner")
	}
	a, _ := rootWorkflowHTTPCandidate(t, s, "all")
	b, _ := rootWorkflowHTTPCandidate(t, s, "own")
	hidden, _ := rootWorkflowHTTPCandidate(t, s, "")
	prefix := "财务_" + s.f.actor
	rootApproverTestDepartment(t, s, prefix+"甲", a, b, hidden)
	rootApproverTestDepartment(t, s, prefix+"乙", a)
	out := data(t, rootApproverTextCall(t, s, h, 1, 100, prefix, ""), 200)
	got := rootApproverIDs(t, out)
	if out["total"] != float64(2) || len(got) != 2 || !got[a] || !got[b] || got[hidden] {
		t.Fatalf("department search leaked or duplicated users: %+v", out)
	}
	for _, q := range []string{"%", "_", prefix + "甲suffix"} {
		empty := data(t, rootApproverTextCall(t, s, h, 1, 100, q, ""), 200)
		if empty["total"] != float64(0) || len(rootApproverIDs(t, empty)) != 0 {
			t.Fatalf("not literal prefix %q %+v", q, empty)
		}
	}
	first := data(t, rootApproverTextCall(t, s, h, 1, 1, prefix, ""), 200)
	next := data(t, rootApproverTextCall(t, s, h, 2, 1, prefix, first["queryVersion"].(string)), 200)
	if first["total"] != float64(2) || next["total"] != float64(2) || reflect.DeepEqual(first["items"], next["items"]) {
		t.Fatalf("matched paging is not complete: %+v %+v", first, next)
	}
}
func TestRootApproverTextSearchBindsQueryAndRefreshesDepartmentMembership(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	a, _ := rootWorkflowHTTPCandidate(t, s, "all")
	b, _ := rootWorkflowHTTPCandidate(t, s, "own")
	prefix := "SearchDept-" + s.f.actor
	dept := rootApproverTestDepartment(t, s, prefix, a, b)
	first := data(t, rootApproverTextCall(t, s, h, 1, 1, prefix, ""), 200)
	token := first["queryVersion"].(string)
	rootWorkflowHTTPError(t, rootApproverTextCall(t, s, h, 1, 1, "different", token), 409, "COMMON_QUERY_CONTEXT_EXPIRED")
	if _, e := s.f.owner.Exec(context.Background(), "DELETE FROM personnel.department_members WHERE department_id=$1 AND user_id=$2", dept, a); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootApproverTextCall(t, s, h, 2, 1, prefix, token), 409, "COMMON_QUERY_CHANGED")
	fresh := data(t, rootApproverTextCall(t, s, h, 1, 100, prefix, ""), 200)
	if fresh["total"] != float64(1) || !rootApproverIDs(t, fresh)[b] || rootApproverIDs(t, fresh)[a] {
		t.Fatal("moved member remained", fresh)
	}
	if _, e := s.f.owner.Exec(context.Background(), "UPDATE personnel.departments SET name=$2 WHERE id=$1", dept, "Renamed-"+s.f.actor); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootApproverTextCall(t, s, h, 1, 100, prefix, fresh["queryVersion"].(string)), 409, "COMMON_QUERY_CHANGED")
	empty := data(t, rootApproverTextCall(t, s, h, 1, 100, prefix, ""), 200)
	if empty["total"] != float64(0) {
		t.Fatal("old department name remained searchable", empty)
	}
}
func TestRootApproverTextSearchRejectsInvalidTextWithoutExpandingManagerSearch(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	data(t, rootApproverTextCall(t, s, h, 1, 20, "", ""), 200)
	data(t, rootApproverTextCall(t, s, h, 1, 20, strings.Repeat("🦊", 100), ""), 200)
	for _, q := range []any{nil, 3, []string{}, map[string]any{}, strings.Repeat("🦊", 101), "nul\x00query"} {
		raw, _ := json.Marshal(map[string]any{"page": 1, "pageSize": 20, "q": q})
		rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", rootApproverPath(s), string(raw), true, true, "", nil), 400, "COMMON_INVALID_ARGUMENT")
	}
	for _, raw := range []string{`{"page":1,"pageSize":20,"q":"a","q":"b"}`, "{\"page\":1,\"pageSize\":20,\"q\":\"\xff\"}"} {
		rootWorkflowHTTPError(t, rootWorkflowHTTPRequest(t, s, h, "POST", rootApproverPath(s), raw, true, true, "", nil), 400, "COMMON_INVALID_ARGUMENT")
	}
	manager := rootManagerSearchHandler(t, s)
	rootWorkflowHTTPError(t, rootManagerSearchCall(t, s, manager, map[string]any{"page": 1, "pageSize": 20, "q": "no widening"}), 400, "COMMON_INVALID_ARGUMENT")
}

func TestRootApproverDepartmentsAreCurrentMinimalDistinctAndPermissionFiltered(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	a, _ := rootWorkflowHTTPCandidate(t, s, "all")
	hidden, _ := rootWorkflowHTTPCandidate(t, s, "")
	nameA, nameB := "A-"+s.f.actor, "B-"+s.f.actor
	rootApproverTestDepartment(t, s, nameB, a)
	rootApproverTestDepartment(t, s, nameA, a, hidden)
	rootApproverTestDepartment(t, s, nameA, a)
	out := data(t, rootApproverTextCall(t, s, h, 1, 100, nameA, ""), 200)
	items := rootManagerSearchItems(t, out)
	if out["total"] != float64(1) || len(items) != 1 {
		t.Fatalf("unqualified or duplicate department candidate: %+v", out)
	}
	row := items[0].(map[string]any)
	if len(row) != 4 || row["id"] != a || !reflect.DeepEqual(row["departmentNames"], []any{nameA, nameB}) {
		t.Fatalf("missing current direct department projection: %+v", row)
	}
	own := data(t, rootApproverCall(t, s, h, 1, 100, ""), 200)
	found := false
	for _, raw := range rootManagerSearchItems(t, own) {
		r := raw.(map[string]any)
		if r["id"] == s.f.actor {
			found = true
			if names, ok := r["departmentNames"].([]any); !ok || len(names) != 0 {
				t.Fatalf("unassigned must be a nonnull empty list: %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("owner missing")
	}
}
func TestRootApproverDepartmentDisplayChangesInvalidateOnlyVisibleProjection(t *testing.T) {
	s := rootWorkflowManagementSetup(t)
	h := rootApproverHandler(t, s)
	a, _ := rootWorkflowHTTPCandidate(t, s, "all")
	hidden, _ := rootWorkflowHTTPCandidate(t, s, "")
	prefix := "Visible-" + s.f.actor
	rootApproverTestDepartment(t, s, prefix, a)
	dept := rootApproverTestDepartment(t, s, "Before-"+s.f.actor, a)
	unrelated := rootApproverTestDepartment(t, s, "Hidden-"+s.f.actor, hidden)
	first := data(t, rootApproverTextCall(t, s, h, 1, 100, prefix, ""), 200)
	row := rootManagerSearchItems(t, first)[0].(map[string]any)
	if !reflect.DeepEqual(row["departmentNames"], []any{"Before-" + s.f.actor, prefix}) {
		t.Fatalf("missing initial names: %+v", row)
	}
	token := first["queryVersion"].(string)
	if _, e := s.f.owner.Exec(context.Background(), "UPDATE personnel.departments SET name=$2 WHERE id=$1", unrelated, "HiddenAfter-"+s.f.actor); e != nil {
		t.Fatal(e)
	}
	data(t, rootApproverTextCall(t, s, h, 1, 100, prefix, token), 200)
	if _, e := s.f.owner.Exec(context.Background(), "UPDATE personnel.departments SET name=$2 WHERE id=$1", dept, "After-"+s.f.actor); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootApproverTextCall(t, s, h, 1, 100, prefix, token), 409, "COMMON_QUERY_CHANGED")
	fresh := data(t, rootApproverTextCall(t, s, h, 1, 100, prefix, ""), 200)
	if !reflect.DeepEqual(rootManagerSearchItems(t, fresh)[0].(map[string]any)["departmentNames"], []any{"After-" + s.f.actor, prefix}) {
		t.Fatal("old department name survived")
	}
	if _, e := s.f.owner.Exec(context.Background(), "DELETE FROM personnel.department_members WHERE department_id=$1 AND user_id=$2", dept, a); e != nil {
		t.Fatal(e)
	}
	rootWorkflowHTTPError(t, rootApproverTextCall(t, s, h, 1, 100, prefix, fresh["queryVersion"].(string)), 409, "COMMON_QUERY_CHANGED")
}
