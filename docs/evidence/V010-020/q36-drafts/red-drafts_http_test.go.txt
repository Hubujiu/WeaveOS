package personnel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Exercises the delegated dispatcher behind the existing real prepare/auth/CSRF
// chain; the actual Service.ServeHTTP registration remains integrator-owned.
func draftsWeb(t *testing.T) *webFixture {
	t.Helper()
	f := setupWeb(t)
	t.Cleanup(func() {
		_, e := f.owner.Exec(context.Background(), "DELETE FROM personnel.drafts WHERE owner_user_id=$1", f.actor.UserID)
		if e != nil {
			t.Error(e)
		}
	})
	s := &Service{Application: f.app, Authenticator: session.Authenticator{Sessions: f.store, DB: f.app.Pool, Origin: "https://weaveos.test"}}
	f.handler = httpserver.NewHandler(func(context.Context) error { return nil }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, e := httpserver.Prepare(w, r, nil)
		if e != nil {
			fail(w, r, e)
			return
		}
		p, e := s.Authenticator.Authenticate(r, r.Method != "GET" && r.Method != "HEAD")
		if e != nil {
			fail(w, r, e)
			return
		}
		if !s.DraftHTTP(w, r, p) {
			respond(w, r, 404, "API_NOT_FOUND", nil)
		}
	}))
	return f
}

const draftCreateBody = `{"kind":"department","targetId":null,"baseVersion":null,"payload":{"name":"","parentId":null}}`

func draftHTTPID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	data := envelopeData(t, w)
	var id string
	_ = json.Unmarshal(data["id"], &id)
	if id == "" {
		t.Fatal("detail id required")
	}
	return id
}
func TestDraftHTTPQ36SessionCSRFAndRestore(t *testing.T) {
	f := draftsWeb(t)
	if w := f.request("GET", "/api/v1/personnel/drafts", "", false, false); w.Code != 401 {
		t.Fatal("session required")
	}
	if w := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, false); w.Code != 403 {
		t.Fatal("shared CSRF required")
	}
	created := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, true)
	if created.Code != 201 {
		t.Fatalf("null target/base allowed for new object: %d %s", created.Code, created.Body)
	}
	id := draftHTTPID(t, created)
	if created.Header().Get("Location") != "/api/v1/personnel/drafts/"+id {
		t.Fatal("created Location")
	}
	if _, e := f.store.Revoke(context.Background(), f.sid); e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		if w := f.request(method, "/api/v1/personnel/drafts/"+id, `{"version":1,"payload":{"name":"","parentId":null}}`, true, true); w.Code != 401 {
			t.Fatalf("revoked session %s: %d", method, w.Code)
		}
	}
	// Fresh real Session for the same account restores persistent inputs.
	var e error
	f.sid, f.csrf, e = f.store.Create(context.Background(), session.Record{UserID: f.actor.UserID, AuthVersion: "1", SessionRef: "11111111-1111-4111-8111-111111111111"})
	if e != nil {
		t.Fatal(e)
	}
	if w := f.request("GET", "/api/v1/personnel/drafts/"+id, "", true, false); w.Code != 200 {
		t.Fatalf("relogin restores: %d", w.Code)
	}
	// Draft APIs do not require any queryVersion/context to save.
	update := `{"version":1,"payload":{"name":"saved after context expiry","parentId":null}}`
	if w := f.request("PUT", "/api/v1/personnel/drafts/"+id, update, true, true); w.Code != 200 {
		t.Fatalf("save without context: %d", w.Code)
	}
	if w := f.request("PUT", "/api/v1/personnel/drafts/"+id, update, true, true); w.Code != 409 || !strings.Contains(w.Body.String(), "PERSONNEL_DRAFT_CONFLICT") {
		t.Fatalf("stale specific draft conflict: %d %s", w.Code, w.Body)
	}
	if w := f.request("DELETE", "/api/v1/personnel/drafts/"+id+"?version=2", "", true, true); w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("versioned delete 204")
	}
}
func TestDraftHTTPQ36StrictEnvelopeFields(t *testing.T) {
	f := draftsWeb(t)
	bodies := []string{
		strings.Replace(draftCreateBody, `"kind":"department"`, `"kind":"department","ownerId":"00000000-0000-4000-8000-000000000001"`, 1),
		strings.Replace(draftCreateBody, `"targetId":null,`, "", 1),
		strings.Replace(draftCreateBody, `"kind":"department"`, `"kind":"department","kind":"identity"`, 1),
		strings.Replace(draftCreateBody, `"name":""`, `"name":"","password":"secret"`, 1),
		strings.Replace(draftCreateBody, `"kind":"department"`, `"kind":null`, 1),
	}
	for i, b := range bodies {
		if w := f.request("POST", "/api/v1/personnel/drafts", b, true, true); w.Code != 400 {
			t.Fatalf("strict create %d: %d", i, w.Code)
		}
	}
	w := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, true)
	if w.Code != 201 {
		t.Fatalf("create: %d", w.Code)
	}
	id := draftHTTPID(t, w)
	for _, b := range []string{`{"version":1,"payload":{"name":"","parentId":null},"baseVersion":9}`, `{"version":1,"payload":{"name":"","parentId":null},"kind":"identity"}`, `{"version":1,"payload":{"name":"","parentId":null},"targetId":null}`, `{"version":1,"payload":{"name":"","parentId":null},"ownerId":"fake"}`, `{"version":null,"payload":{"name":"","parentId":null}}`, `{"version":1,"version":1,"payload":{"name":"","parentId":null}}`} {
		if w := f.request("PUT", "/api/v1/personnel/drafts/"+id, b, true, true); w.Code != 400 {
			t.Fatalf("immutable envelope rejects: %d", w.Code)
		}
	}
	for _, q := range []string{"?version=1&version=1", "?version=1&ownerId=x", "?version=0", "?version=9007199254740992", ""} {
		if w := f.request("DELETE", "/api/v1/personnel/drafts/"+id+q, "", true, true); w.Code != 400 {
			t.Fatalf("strict delete %s: %d", q, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/personnel/drafts?queryVersion=expired", "/api/v1/personnel/drafts?ownerId=x", "/api/v1/personnel/drafts/not-a-uuid"} {
		if w := f.request("GET", path, "", true, false); w.Code != 400 {
			t.Fatalf("strict query/path: %d", w.Code)
		}
	}
	if _, e := f.owner.Exec(context.Background(), "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template); e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		path := "/api/v1/personnel/drafts/" + id
		if method == "DELETE" {
			path += "?version=1"
		}
		if w := f.request(method, path, `{"version":1,"payload":{"name":"","parentId":null}}`, true, true); w.Code != 403 {
			t.Fatalf("live permission revoke %s: %d", method, w.Code)
		}
	}
}
func TestDraftHTTPQ36QuotaAndSummaryOrder(t *testing.T) {
	f := draftsWeb(t)
	ids := []string{}
	for range 20 {
		w := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, true)
		if w.Code != 201 {
			t.Fatalf("quota create: %d", w.Code)
		}
		ids = append(ids, draftHTTPID(t, w))
	}
	if w := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, true); w.Code != 409 || !strings.Contains(w.Body.String(), "PERSONNEL_DRAFT_LIMIT_REACHED") {
		t.Fatalf("quota domain 409: %d", w.Code)
	}
	// Explicit equal timestamps establish id DESC independently of creation order.
	if _, e := f.owner.Exec(context.Background(), "UPDATE personnel.drafts SET updated_at='2026-01-01Z' WHERE owner_user_id=$1", f.actor.UserID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(context.Background(), "UPDATE personnel.drafts SET updated_at='2026-01-02Z' WHERE id=$1", ids[0]); e != nil {
		t.Fatal(e)
	}
	w := f.request("GET", "/api/v1/personnel/drafts", "", true, false)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	data := envelopeData(t, w)
	var items []map[string]json.RawMessage
	_ = json.Unmarshal(data["items"], &items)
	if len(items) != 20 || string(items[0]["id"]) != strconv.Quote(ids[0]) {
		t.Fatal("updated_at DESC first")
	}
	prev := ""
	for i, item := range items {
		if len(item) != 7 || item["payload"] != nil || item["expiresAt"] != nil {
			t.Fatal("exact summary fields")
		}
		var id string
		_ = json.Unmarshal(item["id"], &id)
		if i > 1 && prev < id {
			t.Fatal("equal timestamp id DESC tie-break")
		}
		prev = id
	}
}
