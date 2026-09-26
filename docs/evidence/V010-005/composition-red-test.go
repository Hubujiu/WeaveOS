package main

import (
 "context"
 "net/http/httptest"
 "os"
 "strings"
 "testing"
)

func TestCompositionWiresRealAuthenticationAndReadiness(t *testing.T) {
 cfg:=config{DatabaseURL:os.Getenv("WEAVEOS_TEST_DATABASE_URL"),RedisURL:os.Getenv("WEAVEOS_TEST_REDIS_URL"),Origin:"https://app.example.test",Generation:"composition-test",AuditKeyID:"test",AuditKey:[]byte("synthetic-audit-key-32-bytes-only!")}
 if cfg.DatabaseURL==""||cfg.RedisURL=="" {t.Fatal("isolated PostgreSQL/Redis required")}
 h,close,err:=buildHandler(context.Background(),cfg);if err!=nil {t.Fatal("configured isolated service must compose")};defer close()
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/health/ready",nil));if w.Code!=200 {t.Fatalf("real dependency readiness=%d want200",w.Code)}
 r:=httptest.NewRequest("POST","/api/v1/sessions",strings.NewReader(`{"account":"unknown-composition","password":"Aa1!"}`));r.Header.Set("Origin",cfg.Origin);r.Header.Set("Content-Type","application/json");w=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=401 {t.Fatalf("configured login boundary=%d want401",w.Code)}
 if !strings.Contains(w.Body.String(),w.Header().Get("X-Request-Id")) {t.Fatal("trusted request ID missing from envelope")}
}
func TestCompositionRejectsIncompleteAuthenticationConfig(t *testing.T) {
 _,close,err:=buildHandler(context.Background(),config{DatabaseURL:"configured"});if close!=nil{close()};if err==nil {t.Fatal("partial authentication configuration must fail startup")}
 h,close,err:=buildHandler(context.Background(),config{});if err!=nil{t.Fatal(err)};defer close();w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/health/ready",nil));if w.Code!=503{t.Fatal("unconfigured platform host must remain not-ready")}
}
