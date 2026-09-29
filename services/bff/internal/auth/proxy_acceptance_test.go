package auth

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
)

func TestAuditClientIPOnlyTrustsConfiguredProxy(t *testing.T) {
	a := setup(t)
	a.service.TrustedProxyHosts = []string{"localhost"}
	for _, fixture := range []struct{ peer, forwarded, want string }{
		{"127.0.0.1:4321", "198.51.100.9", "198.51.100.9"},
		{"192.0.2.7:4321", "198.51.100.9", "192.0.2.7"},
		{"127.0.0.1:4321", "198.51.100.9, 203.0.113.1", "127.0.0.1"},
		{"127.0.0.1:4321", "forged-not-an-ip", "127.0.0.1"},
	} {
		r := httptest.NewRequest("POST", origin+"/api/v1/sessions", strings.NewReader(`{"account":"unknown-proxy-fixture","password":"Aa1!"}`))
		r.RemoteAddr = fixture.peer
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", fixture.forwarded)
		w := httptest.NewRecorder()
		httpserver.NewHandler(a.service.Ready, a.service).ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("wrong-credential status=%d", w.Code)
		}
		var ip string
		if err := a.pool.QueryRow(context.Background(), "SELECT host(client_ip) FROM auth.authentication_events WHERE request_id=$1", w.Header().Get("X-Request-Id")).Scan(&ip); err != nil {
			t.Fatal(err)
		}
		if ip != fixture.want {
			t.Errorf("trusted audit IP=%s want%s", ip, fixture.want)
		}
	}
}

func TestIngressRequestIDMatchesResponseAndAudit(t *testing.T) {
	a := setup(t)
	a.service.TrustedProxyHosts = []string{"127.0.0.1"}
	const id = "0123456789abcdef0123456789abcdef"
	r := httptest.NewRequest("POST", origin+"/api/v1/sessions", strings.NewReader(`{"account":"unknown-ingress","password":"Aa1!"}`))
	r.RemoteAddr = "127.0.0.1:4321"
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	r.Header.Set("X-Request-Id", id)
	w := httptest.NewRecorder()
	httpserver.NewHandler(a.service.Ready, a.service).ServeHTTP(w, r)
	checkStatus(t, w, 401)
	var body struct {
		Meta struct {
			RequestID string `json:"requestId"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.RequestID != id || w.Header().Get("X-Request-Id") != id {
		t.Fatal("ingress/header/JSON correlation mismatch")
	}
	var count int
	if err := a.pool.QueryRow(context.Background(), "SELECT count(*) FROM auth.authentication_events WHERE request_id=$1 AND client_ip='198.51.100.9'", id).Scan(&count); err != nil || count != 1 {
		t.Fatal("ingress/audit correlation mismatch")
	}
}
