//go:build workflowruntime_integration && workflowrpc_integration && linux

package main

import (
	"encoding/json"
	"fmt"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"testing"
	"time"
)

func rootFormalLifecycleAccept(t *testing.T, x *rootFormalFixture, action, target string) (string, fc.Command, string) {
	t.Helper()
	f := x.f
	path := rootLifecycleHTTPPath(f)
	preview := rootHTTPData(t, f.call(t, "GET", path+"/lifecycle", "", nil), 200)
	op := f.id(t)
	body := map[string]any{"operationId": op, "action": action, "basisToken": rootHTTPString(t, preview, "basisToken")}
	if target != "" {
		body["targetNodeId"] = target
	}
	encoded, _ := json.Marshal(body)
	accepted := rootHTTPData(t, f.call(t, "POST", path+"/actions", string(encoded), nil), 202)
	id := rootHTTPString(t, accepted, "commandId")
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT command_json FROM applications.workflow_commands WHERE command_id=$1", id).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var c fc.Command
	if json.Unmarshal(raw, &c) != nil || c.CommandID != id || c.Action != action {
		t.Fatal("HTTP lifecycle lacks exact durable command")
	}
	return op, c, string(encoded)
}
func TestRootFormalRuntimeReturnThenWithdrawRecovery(t *testing.T) {
	x := rootFormalSetup(t)
	f := x.f
	oldTask := f.task
	oldPreview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	oldToken := rootHTTPString(t, oldPreview, "basisToken")
	op, command, _ := rootFormalLifecycleAccept(t, x, "return", f.node)
	x.finish(t, op, command, "active")
	var fresh string
	var epoch int64
	if e := f.owner.QueryRow(f.ctx, "SELECT id::text,activation_epoch FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2 AND closed_command_id IS NULL", f.app, f.instance).Scan(&fresh, &epoch); e != nil || fresh == oldTask || epoch != 2 {
		t.Fatalf("return failed to renew native activation %s %d %v", fresh, epoch, e)
	}
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", oldToken), nil), 409, "WORKFLOW_TASK_CHANGED")
	f.task = fresh
	before := x.proxy.executions.Load()
	x.proxy.armed.Store(true)
	withdraw, withdrawCommand, body := rootFormalLifecycleAccept(t, x, "withdraw", "")
	select {
	case receipt := <-x.proxy.dropped:
		if receipt.CommandId != withdrawCommand.CommandID || receipt.Outcome != "success" {
			t.Fatal("wrong real response dropped")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("real withdrawal receipt not intercepted")
	}
	rootHTTPPendingJava(t, f, withdraw, withdrawCommand)
	real, found, e := x.execution.Lookup(f.ctx, withdrawCommand)
	if e != nil || !found || real.Result.State != "withdrawn" {
		t.Fatal("engine withdrawal not durable", e)
	}
	rootFormalStop(x.bff, true)
	rootFormalLog(t, "lifecycle-killed-bff", x.bff)
	x.bff = rootFormalStartChild(t, "lifecycle-restarted-bff", x.binary, nil, x.bffEnv)
	rootMainReady(t, x.bff, x.addr)
	x.proxy.held.Store(false)
	rootHTTPRetryDue(t, f, withdrawCommand)
	x.finish(t, withdraw, withdrawCommand, "withdrawn")
	if x.proxy.executions.Load() != before+1 {
		t.Fatal("withdrawal recovery executed a second command")
	}
	again := rootHTTPData(t, f.call(t, "POST", rootLifecycleHTTPPath(f)+"/actions", body, nil), 200)
	if rootHTTPString(t, again, "commandId") != withdrawCommand.CommandID || rootHTTPString(t, again, "status") != "success" {
		t.Fatal("withdraw replay lost final result")
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events WHERE instance_id=$1 AND action IN ('return','withdraw')", f.instance).Scan(&n); e != nil || n != 2 {
		t.Fatal("flow action history duplicated or lost", e)
	}
	t.Log(fmt.Sprintf("FORMAL_LIFECYCLE return_epoch=%d durable_actions=%d", epoch, n))
}
