package appstructure

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"os"
	"strings"
	"testing"
)

func TestHistoryHTTPRealOwnerCursorAndDeletedFieldTombstone(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	money := field(t, f, "money", nil, map[string]any{})
	money["name"] = "当期金额"
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, money)), 200)
	opts, e := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })
	f.service.Application.CandidateRedis = client
	f.service.Application.CandidateNamespace = "history-test-" + f.app
	c := context.Background()
	id := uuid(t, f.owner)
	fid := money["id"].(string)
	dml := RecordHistoryDML{History: RecordHistoryStore{}, Origin: "ordinary"}
	w := historyCommand(t, f, view, "record.create")
	op := recordOpID(t, w)
	header, e := dml.Insert(c, w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true}, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{fid: "1.00"}}, []string{fid})
	if e != nil {
		t.Fatal(e)
	}
	historyComplete(t, w, header, op, 201)
	if e = w.Commit(c); e != nil {
		t.Fatal(e)
	}
	w = historyCommand(t, f, view, "record.edit")
	op = recordOpID(t, w)
	header, e = dml.UpdateCAS(c, w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true}, RecordEdit{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{fid: "2.00"}}, []string{fid})
	if e != nil {
		t.Fatal(e)
	}
	historyComplete(t, w, header, op, 200)
	if e = w.Commit(c); e != nil {
		t.Fatal(e)
	}
	path := "/api/v1/applications/" + f.app + "/forms/" + view + "/records/" + id + "/history"
	response := rootCall(t, f, "GET", path+"?pageSize=1", nil, f.actor)
	d := data(t, response, 200)
	items := d["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["recordVersionAfter"] != float64(2) || strings.Contains(response.Body.String(), "opaqueTaskRef") {
		t.Fatal("frozen history DTO", response.Body.String())
	}
	change := items[0].(map[string]any)["changes"].([]any)[0].(map[string]any)
	if change["fieldLabel"] != "当期金额" || change["fieldDeleted"] != false || change["valueLabels"] == nil {
		t.Fatalf("ADR14 allowed delta display missing: fieldLabel=%q fieldDeleted=%v valueLabelsPresent=%t", change["fieldLabel"], change["fieldDeleted"], change["valueLabels"] != nil)
	}
	var env struct {
		Meta struct{ Pagination Pagination }
	}
	if json.Unmarshal(response.Body.Bytes(), &env) != nil || !env.Meta.Pagination.HasMore || env.Meta.Pagination.NextPageToken == nil {
		t.Fatal("cursor metadata absent", response.Body.String())
	}
	token := *env.Meta.Pagination.NextPageToken
	second := rootCall(t, f, "GET", path+"?pageSize=1&pageToken="+token, nil, f.actor)
	if got := data(t, second, 200)["items"].([]any); len(got) != 1 || got[0].(map[string]any)["recordVersionBefore"] != float64(0) {
		t.Fatal("cursor missed create event", second.Body.String())
	}
	if r := rootCall(t, f, "GET", path, nil, uuid(t, f.owner)); r.Code != 409 || !strings.Contains(r.Body.String(), "AUTH_SESSION_CHANGED") {
		t.Fatal("expected actor did not guard history", r.Code, r.Body.String())
	}
	if r := rootCall(t, f, "GET", strings.Replace(path, id, uuid(t, f.owner), 1), nil, f.actor); r.Code != 404 {
		t.Fatal("unknown row must404", r.Code, r.Body.String())
	}
	in := input(t, f, 1, 1)
	pre := data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(in)), 200)
	in["confirmationToken"] = pre["confirmation"].(map[string]any)["token"]
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", in), 200)
	if r := rootCall(t, f, "GET", path+"?pageSize=1&pageToken="+token, nil, f.actor); r.Code != 400 {
		t.Fatal("stale schema cursor must reject", r.Code, r.Body.String())
	}
	got := data(t, rootCall(t, f, "GET", path, nil, f.actor), 200)["items"].([]any)
	if len(got) != 2 || got[0].(map[string]any)["changes"].([]any)[0].(map[string]any)["fieldKind"] != "money" {
		t.Fatal("owner lost removed-field event-time type", got)
	}
	removed := got[0].(map[string]any)["changes"].([]any)[0].(map[string]any)
	if removed["fieldLabel"] != "当期金额" || removed["fieldDeleted"] != true {
		t.Fatal("ADR14 deleted field last label", removed)
	}
	var version int64
	if e = f.owner.QueryRow(c, "SELECT record_version FROM "+tablePhysical(table)+" WHERE id=$1", id).Scan(&version); e != nil || version != 2 {
		t.Fatal("schema Save fabricated record change/version", version, e)
	}
}
