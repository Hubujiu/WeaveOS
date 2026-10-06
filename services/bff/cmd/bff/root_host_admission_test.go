package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	hp "google.golang.org/grpc/health/grpc_health_v1"
)

// The existing-task projection is synthetic; this verifies real host/HTTPS
// admission wiring, not the later actual Flowable execution acceptance.
func rootHostAdmissionSetup(t *testing.T) (*rootTaskHTTPFixture, *rootHostFixture, *bffHost) {
	t.Helper()
	f := rootHTTPTaskSetup(t)
	runtime := rootHostSetup(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, e := f.owner.Exec(ctx, "DELETE FROM applications.workflow_publications WHERE app_id=$1 AND actor_user_id=$2 AND status IN ('pending','unknown')", f.app, f.actor)
		if e != nil {
			t.Error(e)
		}
	})
	server := httptest.NewUnstartedServer(nil)
	t.Cleanup(func() { _ = server.Listener.Close() })
	cfg := runtime.cfg
	cfg.Origin = "https://" + server.Listener.Addr().String()
	cfg.Generation = "task-http-" + strings.ReplaceAll(f.app, "-", "")
	host := rootHostOpen(t, context.Background(), context.Background(), cfg)
	server.Config.Handler = host.Handler
	server.StartTLS()
	t.Cleanup(server.Close)
	f.server.Close()
	f.server = server
	f.client = server.Client()
	rootHostReady(t, host, 200)
	return f, runtime, host
}
func TestRootHostAdmissionRejectsNewTaskWhenEngineBecomesUnavailable(t *testing.T) {
	f, runtime, _ := rootHostAdmissionSetup(t)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	operation := f.id(t)
	runtime.engine.health.SetServingStatus(rootExecutionHealth, hp.HealthCheckResponse_NOT_SERVING)
	w := f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(operation, "agree", token), nil)
	rootHTTPError(t, w, 503, "COMMON_SERVICE_UNAVAILABLE")
	var count int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, operation).Scan(&count); e != nil || count != 0 {
		t.Fatal("unavailable formal host persisted a task operation", e)
	}
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1", f.app).Scan(&count); e != nil || count != 0 {
		t.Fatal("unavailable formal host left a command fence", e)
	}
}
func TestRootHostAdmissionRejectsNewPublicationWhenEngineBecomesUnavailable(t *testing.T) {
	f, runtime, _ := rootHostAdmissionSetup(t)
	var revision int64
	if e := f.owner.QueryRow(f.ctx, "SELECT revision FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.flow).Scan(&revision); e != nil {
		t.Fatal(e)
	}
	runtime.engine.health.SetServingStatus(rootDeploymentHealth, hp.HealthCheckResponse_NOT_SERVING)
	operation := f.id(t)
	body, _ := json.Marshal(map[string]any{"operationId": operation, "expectedRevision": revision, "expectedSchemaVersion": 1})
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/workflows/" + f.flow + "/publish"
	w := f.call(t, "POST", path, string(body), nil)
	rootHTTPError(t, w, 503, "COMMON_SERVICE_UNAVAILABLE")
	var count int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_publications WHERE app_id=$1 AND operation_id=$2", f.app, operation).Scan(&count); e != nil || count != 0 {
		t.Fatal("unavailable formal host persisted publication intent", e)
	}
}
