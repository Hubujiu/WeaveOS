package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
)

// Expected results are fixed by docs/architecture/foundation-contract.md FND-01.
func TestLiveness(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpserver.NewHandler(nil).ServeHTTP(w, httptest.NewRequest(method, "/health/live", nil))
			if w.Code != 200 {
				t.Fatalf("expected 200; got %d", w.Code)
			}
			if method == "HEAD" {
				if w.Body.Len() != 0 {
					t.Fatal("HEAD must have no body")
				}
				return
			}
			var data map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if data["status"] != "alive" {
				t.Fatalf("unexpected liveness: %v", data)
			}
		})
	}
}
func TestReadiness(t *testing.T) {
	cases := []struct {
		name   string
		check  httpserver.ReadyCheck
		status int
		body   string
	}{
		{"unwired", nil, 503, "not_ready"},
		{"healthy", func(context.Context) error { return nil }, 200, "ready"},
		{"failed", func(context.Context) error { return errors.New("PRIVATE_DATABASE_URL") }, 503, "not_ready"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpserver.NewHandler(tc.check).ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
			if w.Code != tc.status {
				t.Fatalf("want %d, got %d", tc.status, w.Code)
			}
			var data map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if data["status"] != tc.body {
				t.Fatalf("want %q, got %v", tc.body, data)
			}
			if strings.Contains(w.Body.String(), "PRIVATE_DATABASE_URL") {
				t.Fatal("readiness leaked detail")
			}
		})
	}
}
func TestCancelledReadinessFailsClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/health/ready", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	httpserver.NewHandler(func(context.Context) error { return nil }).ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("cancelled request must not be ready, got %d", w.Code)
	}
}
func TestHealthRejectsWriteMethods(t *testing.T) {
	for _, path := range []string{"/health/live", "/health/ready"} {
		w := httptest.NewRecorder()
		httpserver.NewHandler(nil).ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("method guard failed: %d %v", w.Code, w.Header())
		}
	}
}
func TestHeadersAndUntrustedRequestID(t *testing.T) {
	handler := httpserver.NewHandler(nil)
	ids := map[string]bool{}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/health/live", nil)
		req.Header.Set("X-Request-Id", "attacker-controlled")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		id := w.Header().Get("X-Request-Id")
		if id == "" || id == "attacker-controlled" || ids[id] {
			t.Fatalf("unsafe request ID: %q", id)
		}
		ids[id] = true
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("no-store required")
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("nosniff required")
		}
		if w.Header().Get("Set-Cookie") != "" {
			t.Fatal("health must not create session")
		}
	}
}
func TestUnknownAPINeverFallsBackToHTML(t *testing.T) {
	w := httptest.NewRecorder()
	httpserver.NewHandler(nil).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/unknown", nil))
	if w.Code != 404 {
		t.Fatalf("want 404, got %d", w.Code)
	}
	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatalf("must return JSON, not HTML: %v", err)
	}
	if len(data) != 4 || data["code"] != "API_NOT_FOUND" || data["data"] != nil {
		t.Fatalf("wrong envelope: %v", data)
	}
	meta, ok := data["meta"].(map[string]any)
	if !ok || meta["requestId"] != w.Header().Get("X-Request-Id") {
		t.Fatal("request ID mismatch")
	}
}
