package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRootRuntimeAdmissionUnavailablePublicationCreatesNothingAndCanRetry(t *testing.T) {
	p := rootPublicationSetup(t)
	id := uuid(t, p.s.f.owner)
	p.app.RuntimeReady = func(context.Context) error { return errors.New("private_engine_failure") }
	body, _ := json.Marshal(map[string]any{"operationId": id, "expectedRevision": 1, "expectedSchemaVersion": 1})
	w := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", rootWorkflowHTTPPath(p.s, p.flow, "publish"), string(body), true, true, "", nil)
	rootWorkflowHTTPError(t, w, 503, "COMMON_SERVICE_UNAVAILABLE")
	if strings.Contains(w.Body.String(), "private_engine_failure") {
		t.Fatal("engine health failure leaked")
	}
	var count int
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.workflow_publications WHERE app_id=$1", p.s.f.app).Scan(&count); e != nil || count != 0 {
		t.Fatal("unavailable publication persisted an intent", e)
	}
	if p.head(t).CurrentVersion != 0 || len(p.peer.requests) != 0 {
		t.Fatal("unavailable publication reached engine")
	}
	p.app.RuntimeReady = func(context.Context) error { return nil }
	p.publish(t, id, 1)
}
func TestRootRuntimeAdmissionPublicationProbePrecedesBusinessTransaction(t *testing.T) {
	p := rootPublicationSetup(t)
	calls := 0
	p.app.RuntimeReady = func(ctx context.Context) error {
		calls++
		if ctx == nil || ctx.Err() != nil {
			t.Error("invalid request context")
		}
		if p.app.Pool.Stat().AcquiredConns() != 0 {
			t.Error("health probe holds business connection/locks")
		}
		return nil
	}
	p.publish(t, uuid(t, p.s.f.owner), 1)
	if calls != 1 {
		t.Fatalf("runtime health check count=%d want1", calls)
	}
}
func TestRootRuntimeAdmissionPublicationReplayRemainsAvailableDuringOutage(t *testing.T) {
	p := rootPublicationSetup(t)
	id := uuid(t, p.s.f.owner)
	first := p.publish(t, id, 1)
	p.app.RuntimeReady = func(context.Context) error { return errors.New("synthetic outage") }
	again := p.publish(t, id, 1)
	if first["operationId"] != again["operationId"] || again["status"] != "pending" {
		t.Fatal("outage lost original publication intent")
	}
	var count int
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.workflow_publications WHERE app_id=$1", p.s.f.app).Scan(&count); e != nil || count != 1 {
		t.Fatal("replay duplicated intent", e)
	}
}
func TestRootRuntimeAdmissionPublicationStillChecksRevisionBeforeOutage(t *testing.T) {
	p := rootPublicationSetup(t)
	p.app.RuntimeReady = func(context.Context) error { return errors.New("synthetic outage") }
	body, _ := json.Marshal(map[string]any{"operationId": uuid(t, p.s.f.owner), "expectedRevision": 999, "expectedSchemaVersion": 1})
	w := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", rootWorkflowHTTPPath(p.s, p.flow, "publish"), string(body), true, true, "", nil)
	rootWorkflowHTTPError(t, w, 409, "WORKFLOW_CONFLICT")
}
