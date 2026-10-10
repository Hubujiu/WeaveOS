package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppresets"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func rootPresetService(f *rootTaskHTTPFixture) *apppresets.Service {
	return &apppresets.Service{Pool: f.runtime, Limits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}}
}
func rootPresetActor(f *rootTaskHTTPFixture, actor string) session.Principal {
	return session.Principal{UserID: actor, Record: session.Record{UserID: actor, AuthVersion: "1"}}
}
func rootPresetRequest(t *testing.T, f *rootTaskHTTPFixture, name string) apppresets.Request {
	return apppresets.Request{AppID: f.app, ViewID: f.view, OperationID: f.id(t), ExpectedSchemaVersion: 1, State: apppresets.State{Name: name, Filter: json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + f.field + `","operator":"eq","value":"PRIVATE-FILTER"}]}`), Sort: json.RawMessage(`null`), HiddenColumnIDs: []string{}, ColumnOrder: []string{f.field}, ColumnWidths: map[string]int64{f.field: 240}}}
}
func rootPresetReceipt(t *testing.T, r applications.Result, op string, status int, version int64) string {
	t.Helper()
	var m map[string]any
	if r.Status != status || json.Unmarshal(r.Data, &m) != nil || len(m) != 3 || m["operationId"] != op || m["version"] != float64(version) {
		t.Fatalf("not closed minimum receipt: %+v %s", r, r.Data)
	}
	id, ok := m["id"].(string)
	if !ok || id == "" {
		t.Fatal("missing preset identity")
	}
	return id
}
func rootPresetGrant(t *testing.T, f *rootTaskHTTPFixture, actor, view string) string {
	t.Helper()
	group := f.id(t)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.permission_groups(id,app_id,name) VALUES($1,$2,$3)", group, f.app, "preset-"+group); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", f.app, group, actor); e != nil {
		t.Fatal(e)
	}
	for _, action := range []string{"menu.enter", "data.read"} {
		var grant string
		if e := f.owner.QueryRow(f.ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,$4,'all') RETURNING id::text", f.app, group, view, action).Scan(&grant); e != nil {
			t.Fatal(e)
		}
		if action == "data.read" {
			if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, f.field, f.table); e != nil {
				t.Fatal(e)
			}
		}
	}
	return group
}
func TestRootPrivatePresetsStoreRoundTripCASAndOriginalRecovery(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	s := rootPresetService(f)
	p := rootPresetActor(f, f.actor)
	req := rootPresetRequest(t, f, "  我的方案  ")
	created, e := s.Create(f.ctx, p, req)
	if e != nil {
		t.Fatal("approved private preset create missing", e)
	}
	id := rootPresetReceipt(t, created, req.OperationID, 201, 1)
	if created.Location != "/api/v1/applications/"+f.app+"/forms/"+f.view+"/table-presets/"+id {
		t.Fatal("wrong Location", created.Location)
	}
	replay, e := s.Create(f.ctx, p, req)
	if e != nil || !rootPresetEqualJSON(replay.Data, created.Data) {
		t.Fatal("same request duplicated or changed receipt", e, string(replay.Data))
	}
	raw, e := s.Get(f.ctx, p, f.app, f.view, id)
	if e != nil {
		t.Fatal(e)
	}
	var item map[string]any
	json.Unmarshal(raw, &item)
	if len(item) != 13 || item["id"] != id || item["name"] != "我的方案" || item["invalid"] != false || item["version"] != float64(1) || item["appId"] != f.app || item["viewId"] != f.view || !bytes.Contains(raw, []byte("PRIVATE-FILTER")) {
		t.Fatal("lost persistent state", string(raw))
	}
	list, e := s.List(f.ctx, p, f.app, f.view)
	if e != nil || len(list) != 1 {
		t.Fatal("list", e, len(list))
	}
	req.ID = id
	req.OperationID = f.id(t)
	req.ExpectedVersion = 1
	req.State.Name = "编辑后的方案"
	updated, e := s.Update(f.ctx, p, req)
	if e != nil {
		t.Fatal(e)
	}
	rootPresetReceipt(t, updated, req.OperationID, 200, 2)
	req.OperationID = f.id(t)
	if _, e = s.Update(f.ctx, p, req); !errors.Is(e, apppresets.ErrConflict) {
		t.Fatal("stale CAS overwrote", e)
	}
	req.ExpectedVersion = 2
	req.OperationID = f.id(t)
	deleted, e := s.Delete(f.ctx, p, req)
	if e != nil {
		t.Fatal(e)
	}
	rootPresetReceipt(t, deleted, req.OperationID, 204, 2)
	if _, e = s.Get(f.ctx, p, f.app, f.view, id); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("deleted configuration still returned", e)
	}
	original, e := s.Delete(f.ctx, p, req)
	if e != nil || !rootPresetEqualJSON(original.Data, deleted.Data) {
		t.Fatal("discard retry lost confirmation", e)
	}
	op, e := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, p, req.OperationID)
	if e != nil || op.HTTPStatus != 204 {
		t.Fatal("independent recovery failed", e)
	}
}
func TestRootPrivatePresetsStorePrivateActorAndSharedTableViews(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	s := rootPresetService(f)
	p := rootPresetActor(f, f.actor)
	r := rootPresetRequest(t, f, "相同名称")
	got, e := s.Create(f.ctx, p, r)
	if e != nil {
		t.Fatal(e)
	}
	id := rootPresetReceipt(t, got, r.OperationID, 201, 1)
	rootPresetGrant(t, f, f.other, f.view)
	other := rootPresetActor(f, f.other)
	if _, e = s.Get(f.ctx, other, f.app, f.view, id); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("cross actor leak", e)
	}
	r.OperationID = f.id(t)
	if _, e = s.Create(f.ctx, other, r); e != nil {
		t.Fatal("names leaked across private owners", e)
	}
	view := f.id(t)
	if _, e = f.owner.Exec(f.ctx, "INSERT INTO applications.form_views(id,app_id,table_id,name,position,view_version) VALUES($1,$2,$3,'other view',1,1)", view, f.app, f.table); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "INSERT INTO applications.menu_resources VALUES($1,'form',$2)", f.app, view); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(f.ctx, p, f.app, view, id); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("same table cross view leak", e)
	}
	r.ViewID = view
	r.OperationID = f.id(t)
	if _, e = s.Create(f.ctx, p, r); e != nil {
		t.Fatal("same-table private view not independent", e)
	}
}
func TestRootPrivatePresetsStoreInvalidRedactionExplicitRepairAndRevocation(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	s := rootPresetService(f)
	group := rootPresetGrant(t, f, f.other, f.view)
	p := rootPresetActor(f, f.other)
	r := rootPresetRequest(t, f, "私人条件")
	got, e := s.Create(f.ctx, p, r)
	if e != nil {
		t.Fatal(e)
	}
	id := rootPresetReceipt(t, got, r.OperationID, 201, 1)
	// A type change remains valid storage input, but must not reinterpret old criteria.
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=jsonb_set(fields_json,'{0,kind}','\"multiline\"'),schema_version=schema_version+1 WHERE id=$1", f.table); e != nil {
		t.Fatal(e)
	}
	raw, e := s.Get(f.ctx, p, f.app, f.view, id)
	if e != nil {
		t.Fatal(e)
	}
	var invalid map[string]any
	json.Unmarshal(raw, &invalid)
	if len(invalid) != 5 || invalid["invalid"] != true || invalid["reason"] != "DEFINITION_CHANGED" || bytes.Contains(raw, []byte(f.field)) || bytes.Contains(raw, []byte("PRIVATE-FILTER")) {
		t.Fatal("invalid configuration leaked", string(raw))
	}
	var stored string
	if e = f.owner.QueryRow(f.ctx, "SELECT definition_json FROM applications.table_presets WHERE id=$1", id).Scan(&stored); e != nil || !strings.Contains(stored, "PRIVATE-FILTER") {
		t.Fatal("invalid read rewrote stored definition", e)
	}
	r.ID = id
	r.ExpectedVersion = 1
	r.ExpectedSchemaVersion = 2
	r.OperationID = f.id(t)
	r.State.Filter = json.RawMessage(`null`)
	if _, e = s.Update(f.ctx, p, r); e != nil {
		t.Fatal("explicit valid replacement denied", e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1 AND id=$2", f.app, group); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(f.ctx, p, f.app, f.view, id); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("revoked menu/read leaked private state", e)
	}
	if replay, e := s.Update(f.ctx, p, r); e != nil || replay.Status != 200 {
		t.Fatal("confirmed minimal receipt blocked by revocation", e)
	}
	r.OperationID = f.id(t)
	r.ExpectedVersion = 2
	if _, e = s.Delete(f.ctx, p, r); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("revoked new write allowed", e)
	}
}
func TestRootPrivatePresetsStoreTwentySlotsAndConcurrentCAS(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	s := rootPresetService(f)
	p := rootPresetActor(f, f.actor)
	requests := make([]apppresets.Request, 21)
	for i := range requests {
		requests[i] = rootPresetRequest(t, f, fmt.Sprintf("方案-%02d", i))
	}
	var wg sync.WaitGroup
	results := make([]applications.Result, 21)
	errs := make([]error, 21)
	for i := range requests {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = s.Create(f.ctx, p, requests[i]) }(i)
	}
	wg.Wait()
	success, limited := 0, 0
	winner := -1
	for i, e := range errs {
		if e == nil {
			success++
			winner = i
		} else if errors.Is(e, apppresets.ErrLimit) {
			limited++
		} else {
			t.Fatalf("concurrent create unexpected: %v", e)
		}
	}
	if success != 20 || limited != 1 {
		t.Fatalf("hard quota result success=%d limited=%d", success, limited)
	}
	id := rootPresetReceipt(t, results[winner], requests[winner].OperationID, 201, 1)
	a, b := requests[winner], requests[winner]
	a.ID = id
	b.ID = id
	a.ExpectedVersion = 1
	b.ExpectedVersion = 1
	a.OperationID = f.id(t)
	b.OperationID = f.id(t)
	a.State.Name = "new-A"
	b.State.Name = "new-B"
	wg.Add(2)
	var ea, eb error
	go func() { defer wg.Done(); _, ea = s.Update(f.ctx, p, a) }()
	go func() { defer wg.Done(); _, eb = s.Update(f.ctx, p, b) }()
	wg.Wait()
	if !(ea == nil && errors.Is(eb, apppresets.ErrConflict) || eb == nil && errors.Is(ea, apppresets.ErrConflict)) {
		t.Fatalf("CAS winners: %v %v", ea, eb)
	}
}
func TestRootPrivatePresetsStoreNameSchemaAndOperationConflicts(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	s := rootPresetService(f)
	p := rootPresetActor(f, f.actor)
	r := rootPresetRequest(t, f, "CaseName")
	if _, e := s.Create(f.ctx, p, r); e != nil {
		t.Fatal(e)
	}
	r.State.Name = "different"
	if _, e := s.Create(f.ctx, p, r); !errors.Is(e, applications.ErrOperationConflict) {
		t.Fatal("same operation different config", e)
	}
	r.OperationID = f.id(t)
	r.State.Name = " CaseName "
	if _, e := s.Create(f.ctx, p, r); !errors.Is(e, apppresets.ErrNameConflict) {
		t.Fatal("TrimSpace duplicate not rejected", e)
	}
	r.OperationID = f.id(t)
	r.State.Name = "casename"
	if _, e := s.Create(f.ctx, p, r); e != nil {
		t.Fatal("case unexpectedly folded", e)
	}
	r.OperationID = f.id(t)
	r.State.Name = "schema"
	r.ExpectedSchemaVersion = 99
	_, e := s.Create(f.ctx, p, r)
	var domain *appstructure.Error
	if !errors.As(e, &domain) || domain.Code != "APPLICATION_SCHEMA_CONFLICT" {
		t.Fatal("stale schema allowed", e)
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, r.OperationID).Scan(&n); e != nil || n != 0 {
		t.Fatal("failed write leaked claim", n, e)
	}
}

func rootPresetEqualJSON(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func TestRootPrivatePresetsStoreDeletedFieldsAndPartialPermissionAreWhollyRedacted(t *testing.T) {
	for _, mode := range []string{"deleted", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			second := f.id(t)
			var raw json.RawMessage
			if e := f.owner.QueryRow(f.ctx, "SELECT fields_json FROM applications.logical_tables WHERE id=$1", f.table).Scan(&raw); e != nil {
				t.Fatal(e)
			}
			var fields []map[string]any
			if json.Unmarshal(raw, &fields) != nil {
				t.Fatal("fixture fields")
			}
			secondDefinition := map[string]any{"id": second, "name": "Other field", "kind": "text", "required": false, "default": nil, "config": map[string]any{"maxLength": nil}, "presentation": map[string]any{"helpText": nil, "displayTimeZone": nil}}
			fields = append(fields, secondDefinition)
			raw, _ = json.Marshal(fields)
			single, _ := json.Marshal(secondDefinition)
			if _, e := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=$2 WHERE id=$1", f.table, raw); e != nil {
				t.Fatal(e)
			}
			if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.fields(id,app_id,table_id,definition) VALUES($1,$2,$3,$4)", second, f.app, f.table, single); e != nil {
				t.Fatal(e)
			}
			group := rootPresetGrant(t, f, f.other, f.view)
			if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) SELECT app_id,id,$2,$3 FROM applications.grants WHERE app_id=$1 AND group_id=$4 AND action='data.read'", f.app, second, f.table, group); e != nil {
				t.Fatal(e)
			}
			service := rootPresetService(f)
			p := rootPresetActor(f, f.other)
			r := rootPresetRequest(t, f, "保留名称")
			result, e := service.Create(f.ctx, p, r)
			if e != nil {
				t.Fatal(e)
			}
			id := rootPresetReceipt(t, result, r.OperationID, 201, 1)
			var original string
			if e = f.owner.QueryRow(f.ctx, "SELECT definition_json FROM applications.table_presets WHERE id=$1", id).Scan(&original); e != nil {
				t.Fatal(e)
			}
			reason := "PERMISSION_CHANGED"
			if _, e = f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2", f.app, f.field); e != nil {
				t.Fatal(e)
			}
			if mode == "deleted" {
				reason = "FIELD_UNAVAILABLE"
				remaining, _ := json.Marshal([]map[string]any{secondDefinition})
				if _, e = f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=$2,schema_version=schema_version+1 WHERE id=$1", f.table, remaining); e != nil {
					t.Fatal(e)
				}
				if _, e = f.owner.Exec(f.ctx, "DELETE FROM applications.fields WHERE app_id=$1 AND id=$2", f.app, f.field); e != nil {
					t.Fatal(e)
				}
			}
			items, e := service.List(f.ctx, p, f.app, f.view)
			if e != nil || len(items) != 1 {
				t.Fatal("remaining read permission should permit redacted list", e, len(items))
			}
			var item map[string]any
			json.Unmarshal(items[0], &item)
			if len(item) != 5 || item["invalid"] != true || item["reason"] != reason || bytes.Contains(items[0], []byte(f.field)) || bytes.Contains(items[0], []byte("PRIVATE-FILTER")) {
				t.Fatal("private fields leaked through invalid list", string(items[0]))
			}
			var retained string
			if e = f.owner.QueryRow(f.ctx, "SELECT definition_json FROM applications.table_presets WHERE id=$1", id).Scan(&retained); e != nil || retained != original {
				t.Fatal("invalid list rebased stored definition", e)
			}
			r.ID = id
			r.ExpectedVersion = 1
			r.OperationID = f.id(t)
			if _, e = service.Delete(f.ctx, p, r); e != nil {
				t.Fatal("current menu+some read must allow explicit invalid discard", e)
			}
		})
	}
}
