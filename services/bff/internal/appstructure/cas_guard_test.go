package appstructure

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTwoSessionsSchemaViewCASAndIdenticalOperationSerialize(t *testing.T) {
	for _, kind := range []string{"schema", "view", "same_operation"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			second := *f
			var err error
			second.sid, second.csrf, err = f.session.Create(context.Background(), session.Record{UserID: f.actor, SessionRef: uuid(t, f.owner), AuthVersion: "1"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { f.session.Revoke(context.Background(), second.sid) })
			if second.sid == f.sid {
				t.Fatal("independent sessions required")
			}
			view, _ := newForm(t, f)
			text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
			schema, layout := 0, 0
			if kind != "same_operation" {
				data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
				schema, layout = 1, 1
			}
			left, right := input(t, f, schema, layout, text), input(t, f, schema, layout, text)
			if kind == "same_operation" {
				right = left
			} else if kind == "schema" {
				rightField := map[string]any{}
				for k, v := range text {
					rightField[k] = v
				}
				text["name"] = "left"
				rightField["name"] = "right"
				right["fields"] = []any{rightField}
			} else {
				left["layout"] = []any{map[string]any{"id": uuid(t, f.owner), "kind": "field", "fieldId": text["id"], "span": 4}}
				right["layout"] = []any{map[string]any{"id": uuid(t, f.owner), "kind": "field", "fieldId": text["id"], "span": 6}}
			}
			start := make(chan struct{})
			responses := make(chan *httptest.ResponseRecorder, 2)
			for i, body := range []map[string]any{left, right} {
				caller := []*fixture{f, &second}[i]
				go func(in map[string]any) { <-start; responses <- caller.call(t, "PUT", "/forms/"+view+"/definition", in) }(body)
			}
			close(start)
			good, conflict := 0, 0
			for range 2 {
				w := <-responses
				if w.Code == 200 {
					good++
				} else if w.Code == 409 {
					conflict++
					expected := "APPLICATION_SCHEMA_CONFLICT"
					if kind == "view" {
						expected = "APPLICATION_VIEW_CONFLICT"
					}
					if !strings.Contains(w.Body.String(), expected) {
						t.Fatal("CAS conflict kind", w.Body.String())
					}
				} else {
					t.Fatal("race response", w.Code, w.Body.String())
				}
			}
			if kind == "same_operation" {
				if good != 2 || conflict != 0 {
					t.Fatal("same operation replay")
				}
			} else if good != 1 || conflict != 1 {
				t.Fatal("single CAS winner", good, conflict)
			}
		})
	}
}
func TestExpiredTamperedWrongViewAndChangedPlanConfirmation(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	a, b := field(t, f, "text", nil, map[string]any{"maxLength": nil}), field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, a, b)), 200)
	insertRow(t, f, table, map[string]any{a["id"].(string): "guarded"})
	other := data(t, f.call(t, "POST", "/forms", map[string]any{"operationId": uuid(t, f.owner), "name": "other", "source": map[string]any{"kind": "existing_table", "tableId": table}, "directoryId": nil, "position": 0, "expectedStructureVersion": 1}), 201)["form"].(map[string]any)["id"].(string)
	in := input(t, f, 1, 1, b)
	out := data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(in)), 200)
	token := out["confirmation"].(map[string]any)["token"].(string)
	in["confirmationToken"] = token + "x"
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 400, "COMMON_VALIDATION_FAILED")
	in["confirmationToken"] = token
	b["name"] = "changed-plan"
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 409, "APPLICATION_SCHEMA_CONFIRMATION_STALE")
	b["name"] = "text"
	in["expectedViewVersion"] = 0
	expectError(t, f, "PUT", "/forms/"+other+"/definition", in, 400, "COMMON_VALIDATION_FAILED")
	in["expectedViewVersion"] = 1
	f.service.Application.Now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 409, "APPLICATION_SCHEMA_CONFIRMATION_STALE")
}
func TestLiveFlowRegistryBlocksAndDependencyOnlyChangeStalesToken(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	a, b := field(t, f, "text", nil, map[string]any{"maxLength": nil}), field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, a, b)), 200)
	insertRow(t, f, table, map[string]any{a["id"].(string): "guarded"})
	in := input(t, f, 1, 1, b)
	out := data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(in)), 200)
	in["confirmationToken"] = out["confirmation"].(map[string]any)["token"]
	if _, e := f.runtime.Exec(context.Background(), "INSERT INTO applications.table_field_dependencies VALUES($1,$2,$3,'in_flight',$4)", f.app, table, b["id"], uuid(t, f.owner)); e != nil {
		t.Fatal(e)
	}
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 409, "APPLICATION_SCHEMA_CONFIRMATION_STALE")
	if _, e := f.runtime.Exec(context.Background(), "INSERT INTO applications.table_field_dependencies VALUES($1,$2,$3,'enabled_flow',$4)", f.app, table, a["id"], uuid(t, f.owner)); e != nil {
		t.Fatal(e)
	}
	expectError(t, f, "PUT", "/forms/"+view+"/definition", in, 409, "APPLICATION_SCHEMA_DEPENDENCY_BLOCKED")
}
func TestDirectoryCycleCrossAppAndVersionGate(t *testing.T) {
	f, g := setup(t), setup(t)
	create := func(name string, parent any, version int) map[string]any {
		return map[string]any{"operationId": uuid(t, f.owner), "name": name, "parentId": parent, "position": 0, "expectedStructureVersion": version}
	}
	a := data(t, f.call(t, "POST", "/directories", create("A", nil, 0)), 201)["directory"].(map[string]any)["id"].(string)
	b := data(t, f.call(t, "POST", "/directories", create("B", a, 1)), 201)["directory"].(map[string]any)["id"].(string)
	expectError(t, f, "PUT", "/directories/"+a, create("A", b, 2), 400, "COMMON_VALIDATION_FAILED")
	foreign := data(t, g.call(t, "POST", "/directories", map[string]any{"operationId": uuid(t, g.owner), "name": "foreign", "parentId": nil, "position": 0, "expectedStructureVersion": 0}), 201)["directory"].(map[string]any)["id"].(string)
	expectError(t, f, "POST", "/directories", create("C", foreign, 2), 400, "APPLICATION_RESOURCE_INVALID")
	expectError(t, f, "POST", "/directories", create("C", nil, 1), 409, "APPLICATION_STRUCTURE_CONFLICT")
}
