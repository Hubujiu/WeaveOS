package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestReadAuthenticationConfiguration(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	values := map[string]string{"WEAVEOS_DATABASE_URL": "test-db", "WEAVEOS_REDIS_URL": "test-redis", "WEAVEOS_PUBLIC_ORIGIN": "https://app.example.test", "WEAVEOS_SESSION_GENERATION": "generation-1", "WEAVEOS_AUDIT_KEY_ID": "test", "WEAVEOS_AUDIT_HMAC_KEY": base64.StdEncoding.EncodeToString(key)}
	cfg, err := readConfig(func(k string) string { return values[k] })
	if err != nil || cfg.DatabaseURL != "test-db" || cfg.RedisURL != "test-redis" || cfg.Origin != "https://app.example.test" || cfg.Generation != "generation-1" || cfg.AuditKeyID != "test" || !bytes.Equal(cfg.AuditKey, key) {
		t.Fatal("must load explicit authentication settings and decode HMAC key")
	}
	values["WEAVEOS_AUDIT_HMAC_KEY"] = "not-base64"
	if _, err := readConfig(func(k string) string { return values[k] }); err == nil {
		t.Fatal("malformed audit key must fail configuration")
	}
}

func TestCompositionWiresRealAuthenticationAndReadiness(t *testing.T) {
	cfg := config{DatabaseURL: os.Getenv("WEAVEOS_TEST_DATABASE_URL"), RedisURL: os.Getenv("WEAVEOS_TEST_REDIS_URL"), Origin: "https://app.example.test", Generation: "composition-test", AuditKeyID: "test", AuditKey: []byte("synthetic-audit-key-32-bytes-only!")}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" {
		t.Fatal("isolated PostgreSQL/Redis required")
	}
	h, close, err := buildHandler(context.Background(), cfg)
	if err != nil {
		t.Fatal("configured isolated service must compose")
	}
	defer close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != 200 {
		t.Fatalf("real dependency readiness=%d want200", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader(`{"account":"unknown-composition","password":"Aa1!"}`))
	r.Header.Set("Origin", cfg.Origin)
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("configured login boundary=%d want401", w.Code)
	}
	if !strings.Contains(w.Body.String(), w.Header().Get("X-Request-Id")) {
		t.Fatal("trusted request ID missing from envelope")
	}
}
func TestCompositionRejectsIncompleteAuthenticationConfig(t *testing.T) {
	_, close, err := buildHandler(context.Background(), config{DatabaseURL: "configured"})
	if close != nil {
		close()
	}
	if err == nil {
		t.Fatal("partial authentication configuration must fail startup")
	}
	h, close, err := buildHandler(context.Background(), config{})
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != 503 {
		t.Fatal("unconfigured platform host must remain not-ready")
	}
}
