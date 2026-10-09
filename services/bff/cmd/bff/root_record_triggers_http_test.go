package main

import (
	"encoding/json"
	"net/http"
	"testing"

	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

// Real HTTPS configuration + record writes. Only engine deployment confirmation
// is a synthetic catalog fixture: this proves entry/transaction wiring, not RPC.
func rootHTTPTriggerSetup(t *testing.T) *rootTaskHTTPFixture {
	t.Helper()
	f := rootHTTPResourceSetup(t)
	f.flow = f.id(t)
	start, node, end := f.id(t), f.id(t), f.id(t)
	body := map[string]any{"operationId": f.id(t), "name": "Record triggers", "expectedRevision": 0, "expectedSchemaVersion": 1, "allowWithdraw": true,
		"triggers": []any{map[string]any{"event": "record.created", "condition": nil}, map[string]any{"event": "record.updated", "condition": nil}},
		"graph":    map[string]any{"version": 1, "nodes": []any{map[string]any{"id": start, "kind": "start"}, map[string]any{"id": node, "kind": "approval", "approval": map[string]any{"mode": "all", "assigneeIds": []string{f.actor}, "editableFieldIds": []string{}}}, map[string]any{"id": end, "kind": "end"}}, "edges": []any{map[string]any{"from": start, "to": node}, map[string]any{"from": node, "to": end}}}}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rootHTTPData(t, f.call(t, "PUT", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/workflows/"+f.flow+"/definition", string(raw), nil), 201)
	f.transaction(t, func(tx pgx.Tx) error {
		h, err := (wc.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, f.flow, 1, "isolated-trigger-HTTP-deployment")
		if err != nil {
			return err
		}
		_, err = (wc.Catalog{}).EnableInTx(f.ctx, tx, f.app, f.flow, h.Revision)
		return err
	})
	return f
}
func rootHTTPTriggerCount(t *testing.T, f *rootTaskHTTPFixture, recordID string, want int) {
	t.Helper()
	var count int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND record_id=$3", f.app, f.flow, recordID).Scan(&count); err != nil || count != want {
		t.Fatalf("real HTTPS write retained %d intents, want %d: %v", count, want, err)
	}
}
func TestRootRecordTriggersHTTPSCreateSessionCSRFAndReplay(t *testing.T) {
	f := rootHTTPTriggerSetup(t)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records"
	raw := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":1,"values":{"` + f.field + `":"created over HTTPS"}}`
	rootHTTPError(t, f.call(t, "POST", path, raw, func(r *http.Request) { r.Header.Del("Cookie") }), 401, "AUTH_UNAUTHENTICATED")
	rootHTTPError(t, f.call(t, "POST", path, raw, func(r *http.Request) { r.Header.Del("X-CSRF-Token") }), 403, "COMMON_CSRF_REJECTED")
	got := rootHTTPData(t, f.call(t, "POST", path, raw, nil), 201)
	record := rootHTTPString(t, got, "id")
	rootHTTPTriggerCount(t, f, record, 1)
	again := rootHTTPData(t, f.call(t, "POST", path, raw, nil), 201)
	if rootHTTPString(t, again, "id") != record {
		t.Fatal("HTTPS replay created a second record")
	}
	rootHTTPTriggerCount(t, f, record, 1)
}
func TestRootRecordTriggersHTTPSEditSameValueThenChanged(t *testing.T) {
	f := rootHTTPTriggerSetup(t)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records/" + f.record
	raw := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":1,"expectedRecordVersion":1,"changes":{"` + f.field + `":"original"}}`
	got := rootHTTPData(t, f.call(t, "PATCH", path, raw, nil), 200)
	rootHTTPTriggerCount(t, f, f.record, 0)
	var version int64
	if err := json.Unmarshal(got["recordVersion"], &version); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"operationId": f.id(t), "expectedSchemaVersion": 1, "expectedRecordVersion": version, "changes": map[string]any{f.field: "actually changed over HTTPS"}}
	bytes, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rootHTTPData(t, f.call(t, "PATCH", path, string(bytes), nil), 200)
	rootHTTPTriggerCount(t, f, f.record, 1)
}
