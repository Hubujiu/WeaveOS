package appstructure

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDefinitionAuditCorrelatesActualHTTPRequest(t *testing.T) {
	f := setup(t)
	create := map[string]any{"operationId": uuid(t, f.owner), "name": "correlation", "source": map[string]any{"kind": "new_table"}, "directoryId": nil, "position": 0, "expectedStructureVersion": 0}
	w := f.call(t, "POST", "/forms", create)
	view := data(t, w, 201)["form"].(map[string]any)["id"].(string)
	assert := func(operation any, requestID string) {
		t.Helper()
		var actual string
		if e := f.owner.QueryRow(context.Background(), "SELECT request_id FROM auth.authentication_events WHERE change_summary->>'operationId'=$1", operation).Scan(&actual); e != nil || requestID == "" || actual != requestID {
			t.Fatalf("audit must carry the HTTP request ID: got %q want %q: %v", actual, requestID, e)
		}
	}
	assert(create["operationId"], w.Header().Get("X-Request-Id"))
	in := input(t, f, 0, 0, field(t, f, "text", nil, map[string]any{"maxLength": nil}))
	w = f.call(t, "PUT", "/forms/"+view+"/definition", in)
	data(t, w, 200)
	assert(in["operationId"], w.Header().Get("X-Request-Id"))
}

func TestMissingConfirmationReportsActualNonNullImpact(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	field := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, field)), 200)
	insertRow(t, f, table, map[string]any{field["id"].(string): "private old value"})
	w := f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1))
	var out struct {
		Code string
		Data struct{ Impacts []Impact }
	}
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil || w.Code != 409 || out.Code != "APPLICATION_SCHEMA_CONFIRMATION_REQUIRED" || len(out.Data.Impacts) != 1 {
		t.Fatalf("missing confirmation must explain the real impact: %d %s %v", w.Code, w.Body.String(), e)
	}
	impact := out.Data.Impacts[0]
	if impact.FieldID != field["id"] || impact.Kind != "column_removal" || impact.NonNullRows != 1 || impact.OptionID != nil {
		t.Fatalf("wrong impact: %+v", impact)
	}
}
