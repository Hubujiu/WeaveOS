//go:build workflowruntime_integration && workflowrpc_integration && linux

package main

import (
	"encoding/json"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"reflect"
	"testing"
	"time"
)

func rootFormalRoundBody(t *testing.T, f *rootTaskHTTPFixture, instance, kind string) string {
	t.Helper()
	p := rootHTTPData(t, f.call(t, "GET", rootHTTPRoundPath(f, instance)+"/round-actions", "", nil), 200)
	b, e := json.Marshal(map[string]any{"operationId": f.id(t), "kind": kind, "expectedWorkflowRevision": p["workflowRevision"], "expectedSchemaVersion": p["schemaVersion"], "expectedRecordVersion": p["recordVersion"]})
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func rootFormalRoundRework(t *testing.T, f *rootTaskHTTPFixture, instance string) {
	t.Helper()
	body := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":1,"expectedRecordVersion":1,"changes":{"` + f.field + `":"formal rework"}}`
	d := rootHTTPData(t, f.call(t, "POST", rootHTTPRoundPath(f, instance)+"/rework", body, nil), 200)
	if string(d["recordVersion"]) != "2" {
		t.Fatal("rework did not persist next record version")
	}
}
func rootFormalRoundTask(t *testing.T, f *rootTaskHTTPFixture, instance string) string {
	t.Helper()
	var task string
	rootFormalEventually(t, f.ctx, func() (bool, error) {
		e := f.owner.QueryRow(f.ctx, "SELECT t.id::text FROM applications.workflow_tasks t JOIN applications.workflow_instances i ON i.id=t.instance_id WHERE i.id=$1 AND i.state='active' AND t.closed_command_id IS NULL", instance).Scan(&task)
		return e == nil, e
	})
	return task
}
func rootFormalRoundFinish(t *testing.T, x *rootFormalFixture, op string, c fc.Command, want string, version int64) {
	t.Helper()
	f := x.f
	rootFormalEventually(t, f.ctx, func() (bool, error) {
		d := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-workflow-operations/"+op, "", nil), 200)
		return rootHTTPString(t, d, "status") == "success", nil
	})
	receipt, found, e := x.execution.Lookup(f.ctx, c)
	if e != nil || !found || receipt == nil || receipt.Result.State != want {
		t.Fatal("round action missing genuine engine receipt", e)
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events e JOIN applications.workflow_evidence_documents d ON d.app_id=e.app_id AND d.evidence_hash=e.evidence_hash WHERE e.command_id=$1 AND e.record_version=$2 AND e.schema_version=1", c.CommandID, version).Scan(&n); e != nil || n != 1 {
		t.Fatal("round action evidence did not use latest saved record", n, e)
	}
}
func TestRootFormalRuntimeRoundResubmitReviewAndRecovery(t *testing.T) {
	x := rootFormalSetup(t)
	f := x.f
	first := f.instance
	op, c, _ := rootHTTPAcceptedJavaAction(t, f, "reject")
	x.finish(t, op, c, "rejected")
	rootFormalRoundRework(t, f, first)
	body := rootFormalRoundBody(t, f, first, "resubmit")
	path := rootHTTPRoundPath(f, first) + "/round-actions"
	before := x.proxy.executions.Load()
	x.proxy.armed.Store(true)
	got := rootHTTPData(t, f.call(t, "POST", path, body, nil), 202)
	second := rootHTTPString(t, got, "instanceId")
	if string(got["roundNumber"]) != "2" || rootHTTPString(t, got, "previousInstanceId") != first {
		t.Fatal("resubmit lineage wrong")
	}
	var droppedID string
	select {
	case receipt := <-x.proxy.dropped:
		droppedID = receipt.CommandId
	case <-time.After(15 * time.Second):
		t.Fatal("actual start reply not discarded")
	}
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT command_json FROM applications.workflow_commands WHERE command_id=$1", droppedID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var start fc.Command
	if e := json.Unmarshal(raw, &start); e != nil {
		t.Fatal(e)
	}
	if droppedID != start.CommandID {
		t.Fatal("lost reply belongs to wrong command")
	}
	receipt, found, e := x.execution.Lookup(f.ctx, start)
	if e != nil || !found || receipt == nil || receipt.Result.State != "active" {
		t.Fatal("engine did not actually commit start before loss", e)
	}
	var pending bool
	if e = f.owner.QueryRow(f.ctx, "SELECT state='starting' AND EXISTS(SELECT 1 FROM applications.record_command_fences WHERE command_id=$2) FROM applications.workflow_instances WHERE id=$1", second, start.CommandID).Scan(&pending); e != nil || !pending {
		t.Fatal("unconfirmed start was projected or unfenced", e)
	}
	rootFormalStop(x.bff, true)
	rootFormalLog(t, "round-killed-bff", x.bff)
	rootFormalStop(x.java, true)
	rootFormalLog(t, "round-killed-java", x.java)
	x.java = rootFormalStartChild(t, "round-restarted-java", "java", x.javaArgs, x.javaEnv)
	x.waitEngine(t)
	x.bff = rootFormalStartChild(t, "round-restarted-bff", x.binary, nil, x.bffEnv)
	rootMainReady(t, x.bff, x.addr)
	x.proxy.held.Store(false)
	rootHTTPRetryDue(t, f, start)
	f.instance = second
	f.task = rootFormalRoundTask(t, f, second)
	if x.proxy.executions.Load() != before+1 {
		t.Fatal("recovery repeated engine execution")
	}
	again := rootHTTPData(t, f.call(t, "POST", path, body, nil), 202)
	if !reflect.DeepEqual(got, again) {
		t.Fatal("original resubmit operation replay changed")
	}
	old := rootHTTPData(t, f.call(t, "GET", path, "", nil), 200)
	if string(old["canResubmit"]) != "false" || string(old["canReview"]) != "false" || string(old["canRework"]) != "false" {
		t.Fatal("old round writable")
	}
	denied := f.call(t, "POST", path, rootFormalRoundBody(t, f, first, "review"), nil)
	if denied.Code != 409 {
		t.Fatalf("old round start status=%d", denied.Code)
	}
	op, c, _ = rootHTTPAcceptedJavaAction(t, f, "agree")
	rootFormalRoundFinish(t, x, op, c, "completed", 2)
	review := rootHTTPData(t, f.call(t, "POST", rootHTTPRoundPath(f, second)+"/round-actions", rootFormalRoundBody(t, f, second, "review"), nil), 202)
	third := rootHTTPString(t, review, "instanceId")
	if string(review["roundNumber"]) != "3" || rootHTTPString(t, review, "previousInstanceId") != second {
		t.Fatal("review lineage wrong")
	}
	f.instance = third
	f.task = rootFormalRoundTask(t, f, third)
	op, c, _ = rootHTTPAcceptedJavaAction(t, f, "agree")
	rootFormalRoundFinish(t, x, op, c, "completed", 2)
	var count int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_rounds WHERE app_id=$1 AND flow_id=$2 AND record_id=$3", f.app, f.flow, f.record).Scan(&count); e != nil || count != 3 {
		t.Fatal("round history changed", count, e)
	}
	var firstState, secondState string
	if e = f.owner.QueryRow(f.ctx, "SELECT a.state,b.state FROM applications.workflow_instances a,applications.workflow_instances b WHERE a.id=$1 AND b.id=$2", first, second).Scan(&firstState, &secondState); e != nil || firstState != "rejected" || secondState != "completed" {
		t.Fatal("prior results mutated", e)
	}
}
func TestRootFormalRuntimeRoundIsolation(t *testing.T) {
	x := rootFormalSetup(t)
	f := x.f
	aFlow, aInstance, aTask, aNode, aVersion := f.flow, f.instance, f.task, f.node, f.version
	x.publishAndStartEditable(t, false)
	bFlow, bInstance, bTask, bNode, bVersion := f.flow, f.instance, f.task, f.node, f.version
	stale := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	bPath := f.taskPath()
	f.flow, f.instance, f.task, f.node, f.version = aFlow, aInstance, aTask, aNode, aVersion
	op, c, _ := rootHTTPAcceptedJavaAction(t, f, "reject")
	x.finish(t, op, c, "rejected")
	rootFormalRoundRework(t, f, aInstance)
	res := f.call(t, "POST", bPath+"/actions", rootHTTPActionBody(f.id(t), "agree", rootHTTPString(t, stale, "basisToken")), nil)
	if res.Code != 409 {
		t.Fatalf("other workflow stale record basis status=%d", res.Code)
	}
	next := rootHTTPData(t, f.call(t, "POST", rootHTTPRoundPath(f, aInstance)+"/round-actions", rootFormalRoundBody(t, f, aInstance, "resubmit"), nil), 202)
	_ = rootFormalRoundTask(t, f, rootHTTPString(t, next, "instanceId"))
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE flow_id=$1 AND state='active' AND id=$2", bFlow, bInstance).Scan(&n); e != nil || n != 1 {
		t.Fatal("A changed B active instance", e)
	}
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_rounds WHERE flow_id=$1", bFlow).Scan(&n); e != nil || n != 1 {
		t.Fatal("A restarted B", e)
	}
	f.flow, f.instance, f.task, f.node, f.version = bFlow, bInstance, bTask, bNode, bVersion
	op, c, _ = rootHTTPAcceptedJavaAction(t, f, "agree")
	rootFormalRoundFinish(t, x, op, c, "completed", 2)
}
