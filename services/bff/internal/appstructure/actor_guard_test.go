package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExpectedActorRejectsSwitchedSessionBeforeDefinitionClaim(t *testing.T) {
	a, b := setup(t), setup(t)
	sid, csrf, e := a.session.Create(context.Background(), session.Record{UserID: b.actor, SessionRef: uuid(t, a.owner), AuthVersion: "1"})
	if e != nil {
		t.Fatal(e)
	}
	defer a.session.Revoke(context.Background(), sid)
	op := uuid(t, a.owner)
	body := map[string]any{"operationId": op, "name": "stale-A", "source": map[string]any{"kind": "new_table"}, "directoryId": nil, "position": 0, "expectedStructureVersion": 0}
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "https://weaveos.test/api/v1/applications/"+b.app+"/forms", strings.NewReader(string(raw)))
	r.Header.Set("Origin", "https://weaveos.test")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("X-Expected-Actor-Id", a.actor)
	r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
	r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
	w := httptest.NewRecorder()
	a.service.ServeHTTP(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "AUTH_SESSION_CHANGED") {
		t.Fatalf("actual B must not bind A operation: %d %s", w.Code, w.Body.String())
	}
	var count int
	a.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", b.actor, op).Scan(&count)
	if count != 0 {
		t.Fatal("mismatched actor claimed operation")
	}
}
func TestExpectedActorGuardCoversOriginalOperationAndReads(t *testing.T) {
	f := setup(t)
	s := &applications.Service{Application: &applications.Application{Pool: f.runtime}, Authenticator: f.service.Authenticator, Definitions: f.service}
	for _, path := range []string{"/api/v1/applications/" + f.app + "/structure", "/api/v1/application-operations/" + uuid(t, f.owner)} {
		r := httptest.NewRequest("GET", "https://weaveos.test"+path, nil)
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
		r.Header.Set("X-Expected-Actor-Id", uuid(t, f.owner))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 409 || !strings.Contains(w.Body.String(), "AUTH_SESSION_CHANGED") {
			t.Fatalf("actor guard before business read %s %d %s", path, w.Code, w.Body.String())
		}
	}
}
