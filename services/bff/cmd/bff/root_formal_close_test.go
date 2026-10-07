//go:build workflowruntime_integration && workflowrpc_integration && linux

package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRootFormalRuntimeCloseWaitsForConfirmedProjection(t *testing.T) {
	x := rootFormalSetup(t)
	f := x.f
	base := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/workflows/" + f.flow + "/"
	var revision int64
	if e := f.owner.QueryRow(f.ctx, "SELECT revision FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.flow).Scan(&revision); e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"operationId": f.id(t), "expectedRevision": revision})
	closing := rootHTTPData(t, f.call(t, "POST", base+"close", string(body), nil), 200)
	if rootHTTPString(t, closing, "state") != "closing" {
		t.Fatal("close reported finished with active instance")
	}
	before := x.proxy.executions.Load()
	x.proxy.armed.Store(true)
	op, command, replayBody := rootHTTPAcceptedJavaAction(t, f, "agree")
	select {
	case receipt := <-x.proxy.dropped:
		if receipt.CommandId != command.CommandID || receipt.Outcome != "success" {
			t.Fatal("wrong real completion response dropped")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("real completion response was not intercepted")
	}
	rootHTTPPendingJava(t, f, op, command)
	real, found, e := x.execution.Lookup(f.ctx, command)
	if e != nil || !found || real.Result.State != "completed" {
		t.Fatal("engine did not actually complete before lost response", e)
	}
	var state string
	var pendingRevision int64
	if e = f.owner.QueryRow(f.ctx, "SELECT state,revision FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.flow).Scan(&state, &pendingRevision); e != nil || state != "closing" || pendingRevision != revision+1 {
		t.Fatalf("unknown application result finished close prematurely: %s %d %v", state, pendingRevision, e)
	}
	rootFormalStop(x.bff, true)
	rootFormalLog(t, "close-killed-bff", x.bff)
	x.bff = rootFormalStartChild(t, "close-restarted-bff", x.binary, nil, x.bffEnv)
	rootMainReady(t, x.bff, x.addr)
	x.proxy.held.Store(false)
	rootHTTPRetryDue(t, f, command)
	x.finish(t, op, command, "completed")
	final := rootHTTPData(t, f.call(t, "GET", base+"definition", "", nil), 200)
	if rootHTTPString(t, final, "state") != "disabled" {
		t.Fatal("confirmed last instance left requested close unfinished")
	}
	if e = f.owner.QueryRow(f.ctx, "SELECT revision FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.flow).Scan(&pendingRevision); e != nil || pendingRevision != revision+2 {
		t.Fatalf("closure revision did not advance exactly once: %d %v", pendingRevision, e)
	}
	again := rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", replayBody, nil), 200)
	if rootHTTPString(t, again, "commandId") != command.CommandID || x.proxy.executions.Load() != before+1 {
		t.Fatal("recovery/replay executed the completed command again")
	}
}
