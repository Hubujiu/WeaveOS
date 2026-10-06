package main

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	"github.com/jackc/pgx/v5/pgxpool"
	hp "google.golang.org/grpc/health/grpc_health_v1"
)

type rootHostFixture struct {
	cfg             config
	engine          *rootRPCFixture
	admin           *pgxpool.Pool
	applicationName string
}

func rootHostSetup(t *testing.T) *rootHostFixture {
	t.Helper()
	database, redis := os.Getenv("WEAVEOS_TEST_DATABASE_URL"), os.Getenv("WEAVEOS_TEST_REDIS_URL")
	if database == "" || redis == "" {
		t.Fatal("isolated migrated PostgreSQL and Redis are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	admin, e := pgxpool.New(ctx, database)
	if e != nil {
		t.Fatal("invalid isolated PG configuration")
	}
	t.Cleanup(admin.Close)
	if e = admin.Ping(ctx); e != nil {
		t.Fatal("isolated PG unavailable")
	}
	u, e := url.Parse(database)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("v041-host-%d", time.Now().UnixNano())
	q := u.Query()
	q.Set("application_name", name)
	q.Set("options", "-c role=auth_app")
	u.RawQuery = q.Encode()
	engine := rootRPCServer(t, false)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go func() { _ = engine.server.Serve(listener) }()
	t.Cleanup(func() { _ = listener.Close() })
	cfg := config{DatabaseURL: u.String(), RedisURL: redis, Origin: "https://app.example.test", Generation: name, AuditKeyID: "test", AuditKey: []byte("synthetic-audit-key-32-bytes-only!"), DefinitionKeyID: "test", DefinitionKey: []byte("isolated-definition-test-32bytes!"), SchemaLimits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}, WorkflowRuntime: rootRPCConfig(t)}
	cfg.WorkflowRuntime.Target = listener.Addr().String()
	return &rootHostFixture{cfg: cfg, engine: engine, admin: admin, applicationName: name}
}
func rootHostOpen(t *testing.T, startup, process context.Context, cfg config) *bffHost {
	t.Helper()
	h, e := buildHost(startup, process, cfg)
	if e != nil || h == nil || h.Handler == nil {
		t.Fatalf("formal host unavailable: %v", e)
	}
	t.Cleanup(func() {
		h.Quiesce()
		if e := h.Close(); e != nil {
			t.Error("host cleanup failed", e)
		}
	})
	return h
}
func rootHostReady(t *testing.T, h *bffHost, want int) {
	t.Helper()
	w := httptest.NewRecorder()
	h.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != want {
		t.Fatalf("readiness=%d want%d body=%s", w.Code, want, w.Body.String())
	}
}
func (f *rootHostFixture) noConnections(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var count int
		if e := f.admin.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE application_name=$1", f.applicationName).Scan(&count); e != nil {
			t.Fatal("connection observation failed")
		}
		if count == 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("closed host leaked PG connections")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func TestRootHostEnabledWiresRealDependenciesAndKeepsAuthentication(t *testing.T) {
	f := rootHostSetup(t)
	h := rootHostOpen(t, context.Background(), context.Background(), f.cfg)
	rootHostReady(t, h, 200)
	if h.WorkersDone() == nil {
		t.Fatal("enabled host omitted workers")
	}
	select {
	case <-h.WorkersDone():
		t.Fatal("worker loops exited during normal operation")
	default:
	}
	r := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader(`{"account":"unknown-runtime-composition","password":"Aa1!"}`))
	r.Header.Set("Origin", f.cfg.Origin)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("existing authentication boundary changed: %d", w.Code)
	}
}
func TestRootHostStartupCancellationDoesNotOwnWorkerLifetime(t *testing.T) {
	f := rootHostSetup(t)
	startup, cancel := context.WithCancel(context.Background())
	h := rootHostOpen(t, startup, context.Background(), f.cfg)
	cancel()
	rootHostReady(t, h, 200)
	select {
	case <-h.WorkersDone():
		t.Fatal("workers used canceled startup context")
	default:
	}
}
func TestRootHostProcessCancellationStopsBothWorkers(t *testing.T) {
	f := rootHostSetup(t)
	process, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := rootHostOpen(t, context.Background(), process, f.cfg)
	cancel()
	rootWorkerSignal(t, h.WorkersDone())
	rootHostReady(t, h, 503)
	if e := h.Err(); e != nil {
		t.Fatal("normal process cancellation treated as failure", e)
	}
	if e := h.Close(); e != nil {
		t.Fatal(e)
	}
	f.noConnections(t)
}
func TestRootHostUnhealthyEngineFailsStartupAndCleansDependencies(t *testing.T) {
	f := rootHostSetup(t)
	f.engine.health.SetServingStatus(rootExecutionHealth, hp.HealthCheckResponse_NOT_SERVING)
	h, e := buildHost(context.Background(), context.Background(), f.cfg)
	if e == nil || h != nil {
		t.Fatal("unhealthy configured engine silently accepted")
	}
	f.noConnections(t)
}
func TestRootHostWrongIdentityFailsStartupWithoutEcho(t *testing.T) {
	f := rootHostSetup(t)
	secret := strings.Repeat("WRONG_SYNTHETIC_TOKEN", 2)
	f.cfg.WorkflowRuntime.Identity, _ = workflowrpc.NewServiceIdentity(secret)
	h, e := buildHost(context.Background(), context.Background(), f.cfg)
	if e == nil || h != nil {
		t.Fatal("wrong service identity accepted")
	}
	if strings.Contains(e.Error(), secret) || strings.Contains(e.Error(), "synthetic secret") {
		t.Fatal("initialization leaks identity or remote detail")
	}
	f.noConnections(t)
}
func TestRootHostReadinessTracksEngineFailureAndRecovery(t *testing.T) {
	f := rootHostSetup(t)
	h := rootHostOpen(t, context.Background(), context.Background(), f.cfg)
	rootHostReady(t, h, 200)
	f.engine.health.SetServingStatus(rootExecutionHealth, hp.HealthCheckResponse_NOT_SERVING)
	rootHostReady(t, h, 503)
	f.engine.health.SetServingStatus(rootExecutionHealth, hp.HealthCheckResponse_SERVING)
	rootHostReady(t, h, 200)
	h.Quiesce()
	rootHostReady(t, h, 503)
	rootWorkerSignal(t, h.WorkersDone())
}
func TestRootHostCloseWaitsWorkersAndReleasesStorageIdempotently(t *testing.T) {
	f := rootHostSetup(t)
	h := rootHostOpen(t, context.Background(), context.Background(), f.cfg)
	rootHostReady(t, h, 200)
	if e := h.Close(); e != nil {
		t.Fatal(e)
	}
	rootWorkerSignal(t, h.WorkersDone())
	f.noConnections(t)
	rootHostReady(t, h, 503)
	if e := h.Close(); e != nil {
		t.Fatal("repeat close failed", e)
	}
}
func TestRootHostDisabledModePreservesExistingReadiness(t *testing.T) {
	f := rootHostSetup(t)
	f.cfg.WorkflowRuntime = workflowRuntimeConfig{}
	h := rootHostOpen(t, context.Background(), context.Background(), f.cfg)
	rootHostReady(t, h, 200)
	if h.WorkersDone() != nil {
		t.Fatal("disabled configuration started workers")
	}
	f.engine.mu.Lock()
	calls := len(f.engine.methods)
	f.engine.mu.Unlock()
	if calls != 0 {
		t.Fatal("disabled configuration contacted engine")
	}
	empty := rootHostOpen(t, context.Background(), context.Background(), config{})
	rootHostReady(t, empty, 503)
}
func TestRootHostRejectsIncompleteLifecycleAndWriteLimits(t *testing.T) {
	f := rootHostSetup(t)
	bad := f.cfg
	bad.SchemaLimits = appschema.Limits{}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, c := range []struct {
		startup, process context.Context
		cfg              config
	}{{nil, context.Background(), f.cfg}, {context.Background(), nil, f.cfg}, {canceled, context.Background(), f.cfg}, {context.Background(), canceled, f.cfg}, {context.Background(), context.Background(), bad}} {
		h, e := buildHost(c.startup, c.process, c.cfg)
		if e == nil || h != nil {
			t.Error("invalid startup/lifetime configuration accepted")
		}
	}
	f.noConnections(t)
}
