package httpserver_test

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBusinessHandlerReceivesPlatformSecurityHeaders(t *testing.T) {
	business := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if w.Header().Get("X-Request-Id") == "" || w.Header().Get("Cache-Control") != "no-store" {
			t.Error("platform metadata must precede business dispatch")
		}
		w.WriteHeader(201)
	})
	h := httpserver.NewHandler(nil, business)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/registrations", nil))
	if w.Code != 201 {
		t.Fatalf("business route status=%d want201", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != 503 {
		t.Fatal("dispatch must preserve unwired readiness fail-closed")
	}
}
