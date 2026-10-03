package appstructure

import (
	"context"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestOtherViewRemovalAndTypeConversionHaveDifferentGuards(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	other := data(t, f.call(t, "POST", "/forms", map[string]any{"operationId": uuid(t, f.owner), "name": "第二视图", "source": map[string]any{"kind": "existing_table", "tableId": table}, "directoryId": nil, "position": 0, "expectedStructureVersion": 1}), 201)["form"].(map[string]any)["id"].(string)
	in := input(t, f, 1, 0, text)
	in["layout"] = []any{map[string]any{"id": uuid(t, f.owner), "kind": "field", "fieldId": text["id"]}}
	data(t, f.call(t, "PUT", "/forms/"+other+"/definition", in), 200)
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1), 409, "APPLICATION_SCHEMA_DEPENDENCY_BLOCKED")
	number := field(t, f, "number", nil, map[string]any{})
	number["id"] = text["id"]
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, number)), 200)
}
func TestOtherViewLayoutChangesDependencyRevision(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	insertRow(t, f, table, map[string]any{text["id"].(string): "value"})
	in := input(t, f, 1, 1)
	path := "/forms/" + view + "/definition"
	out := data(t, f.call(t, "POST", path+"/preflight", preflight(in)), 200)
	in["confirmationToken"] = out["confirmation"].(map[string]any)["token"]
	data(t, f.call(t, "POST", "/forms", map[string]any{"operationId": uuid(t, f.owner), "name": "新视图", "source": map[string]any{"kind": "existing_table", "tableId": table}, "directoryId": nil, "position": 0, "expectedStructureVersion": 1}), 201)
	expectError(t, f, "PUT", path, in, 409, "APPLICATION_SCHEMA_CONFIRMATION_STALE")
}
func TestRequiredSelectionCannotBeClearedAndMultiToSingleCannotHideCardinality(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	a, b := uuid(t, f.owner), uuid(t, f.owner)
	choices := field(t, f, "multi_select", nil, map[string]any{"options": []any{map[string]any{"id": a, "label": "A"}, map[string]any{"id": b, "label": "B"}}})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, choices)), 200)
	insertRow(t, f, table, map[string]any{choices["id"].(string): []string{a, b, a}})
	choices["kind"] = "single_select"
	choices["config"] = map[string]any{"options": []any{map[string]any{"id": a, "label": "A"}}}
	in := input(t, f, 1, 1, choices)
	in["optionMappings"] = []any{map[string]any{"fieldId": choices["id"], "fromOptionId": b, "toOptionId": a}}
	out := data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(in)), 200)
	if out["saveAllowed"] != false {
		t.Fatal("mapping concealed two original distinct values")
	}
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 409, "APPLICATION_SCHEMA_CONVERSION_FAILED")
}

type unavailableRegistry struct{}

func (unavailableRegistry) Available(context.Context, pgx.Tx, string) error { return ErrUnavailable }
func TestDependencyUnavailableDoesNotPretendEmpty(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	for _, registry := range []DependencyRegistry{nil, unavailableRegistry{}} {
		f.service.Application.Dependencies = registry
		expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1), 503, "COMMON_SERVICE_UNAVAILABLE")
	}
}
func TestPreflightReferenceSourceAndStrictCompleteDTO(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	member := field(t, f, "member", uuid(t, f.owner), map[string]any{})
	in := input(t, f, 0, 0, member)
	expectError(t, f, "POST", "/forms/"+view+"/definition/preflight", preflight(in), 503, "COMMON_SERVICE_UNAVAILABLE")
	delete(member, "required")
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 400, "COMMON_VALIDATION_FAILED")
	member["required"] = false
	in["fields"] = nil
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 400, "COMMON_VALIDATION_FAILED")
}
