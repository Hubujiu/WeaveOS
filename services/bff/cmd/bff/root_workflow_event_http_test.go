package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowprojection"
	"github.com/jackc/pgx/v5"
)

func rootHTTPConfirmedEvent(t *testing.T, f *rootTaskHTTPFixture) string {
	t.Helper()
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	accepted := rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", rootHTTPString(t, preview, "basisToken")), nil), 202)
	id := rootHTTPString(t, accepted, "commandId")
	f.transaction(t, func(tx pgx.Tx) error {
		l := fc.Ledger{Namespace: "applications"}
		entry, e := l.GetInTx(f.ctx, tx, id)
		if e != nil {
			return e
		}
		c := entry.Command
		p, e := l.ExecutionPayloadInTx(f.ctx, tx, c)
		if e != nil {
			return e
		}
		body := rootHTTPResultBytes(fc.ExecutionResult{InstanceID: f.instance, EngineProcessID: "root-http-" + f.instance, State: "completed", SchemaVersion: 1, RecordVersion: 1, Tasks: []fc.ExecutionTask{}})
		hash, e := fc.Fingerprint(c)
		if e != nil {
			return e
		}
		r := fc.Receipt{CommandID: id, CommandHash: hash, Outcome: "success", Sequence: 2, ProofID: f.id(t), ResultHash: sha256.Sum256(body)}
		_, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, body)
		return e
	})
	return fmt.Sprintf("/api/v1/applications/%s/forms/%s/records/%s/workflow-events/%s", f.app, f.view, f.record, id)
}

func TestRootWorkflowEventHTTPSExactSafeSnapshot(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootHTTPConfirmedEvent(t, f)
	data := rootHTTPData(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }), 200)
	rootHTTPFieldSet(t, data, []string{"event", "basis"})
	var event, basis map[string]json.RawMessage
	if json.Unmarshal(data["event"], &event) != nil || json.Unmarshal(data["basis"], &basis) != nil {
		t.Fatal("bad nested DTO")
	}
	rootHTTPFieldSet(t, event, []string{"flowName", "flowNameSource", "id", "instanceId", "flowId", "nodeId", "targetNodeId", "actorId", "action", "outcome", "sequence", "schemaVersion", "recordVersion", "occurredAt"})
	if rootHTTPString(t, event, "nodeId") != f.node || rootHTTPString(t, event, "actorId") != f.actor || rootHTTPString(t, event, "action") != "agree" || rootHTTPString(t, event, "outcome") != "success" || string(event["targetNodeId"]) != "null" {
		t.Fatal("wrong actual confirmed event")
	}
	rootHTTPFieldSet(t, basis, []string{"status", "fields"})
	if rootHTTPString(t, basis, "status") != "available" {
		t.Fatal("real captured evidence unavailable")
	}
	var fields []map[string]json.RawMessage
	if json.Unmarshal(basis["fields"], &fields) != nil || len(fields) != 1 {
		t.Fatal("wrong historical field count")
	}
	rootHTTPFieldSet(t, fields[0], []string{"fieldId", "fieldName", "fieldKind", "value", "valueLabels"})
	if rootHTTPString(t, fields[0], "fieldId") != f.field || rootHTTPString(t, fields[0], "fieldName") != "Reason" || rootHTTPString(t, fields[0], "value") != "original" || string(fields[0]["valueLabels"]) != "{}" {
		t.Fatal("wrong exact historical value")
	}
}

func TestRootWorkflowEventHTTPSClosedReadAndIdentity(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	path := rootHTTPConfirmedEvent(t, f)
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }), 409, "AUTH_SESSION_CHANGED")
	for _, q := range []string{"?x=1", "?"} {
		rootHTTPError(t, f.call(t, "GET", path+q, "", nil), 400, "COMMON_VALIDATION_FAILED")
	}
	for _, body := range []string{"{}", " ", "null"} {
		rootHTTPError(t, f.call(t, "GET", path, body, nil), 400, "COMMON_VALIDATION_FAILED")
	}
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		rootHTTPError(t, f.call(t, method, path, "", nil), 404, "API_NOT_FOUND")
	}
	rootHTTPError(t, f.call(t, "GET", path+"/", "", nil), 404, "API_NOT_FOUND")
	sid, _, e := f.store.Create(f.ctx, session.Record{UserID: f.other, SessionRef: f.id(t), AuthVersion: "1"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.store.Revoke(context.Background(), sid) })
	rootHTTPError(t, f.call(t, "GET", path, "", func(r *http.Request) {
		r.Header.Del("Cookie")
		r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: sid})
	}), 403, "APPLICATION_FORBIDDEN")
}
