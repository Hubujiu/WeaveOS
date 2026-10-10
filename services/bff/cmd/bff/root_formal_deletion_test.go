//go:build workflowruntime_integration && workflowrpc_integration && linux

package main

import (
	"encoding/json"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"testing"
	"time"
)

func TestRootFormalRuntimeDeletionDrainsAndRecoversOriginalReceipt(t *testing.T) {
	x := rootFormalSetup(t)
	f := x.f
	base := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/workflows/" + f.flow + "/"
	var revision int64
	if e := f.owner.QueryRow(f.ctx, "SELECT revision FROM applications.workflow_definitions WHERE id=$1", f.flow).Scan(&revision); e != nil {
		t.Fatal(e)
	}
	op := f.id(t)
	raw, _ := json.Marshal(map[string]any{"operationId": op, "expectedRevision": revision})
	x.proxy.deleteDropped = make(chan *pb.FlowDeletionReceipt, 1)
	x.proxy.deleteArmed.Store(true)
	got := rootHTTPData(t, f.call(t, "POST", base+"delete", string(raw), nil), 202)
	if rootHTTPString(t, got, "status") != "pending" {
		t.Fatal("active flow deletion was not pending")
	}
	rootFormalEventually(t, f.ctx, func() (bool, error) {
		var waiting bool
		e := f.owner.QueryRow(f.ctx, "SELECT COALESCE(reason='waiting_work',false) FROM applications.workflow_deletions WHERE flow_id=$1", f.flow).Scan(&waiting)
		return waiting, e
	})
	if x.proxy.deletes.Load() != 0 {
		t.Fatal("engine deletion attempted while instance active")
	}
	action, c, body := rootHTTPAcceptedJavaAction(t, f, "agree")
	var receipt *pb.FlowDeletionReceipt
	select {
	case receipt = <-x.proxy.deleteDropped:
	case <-time.After(15 * time.Second):
		t.Fatal("formal worker never reached committed engine deletion")
	}
	if receipt.AppId != f.app || receipt.FlowId != f.flow || receipt.OperationId != op || receipt.DeletedVersions != 1 {
		t.Fatal("engine receipt identity/count changed")
	}
	x.finish(t, action, c, "completed")
	rootFormalEventually(t, f.ctx, func() (bool, error) {
		var unknown bool
		e := f.owner.QueryRow(f.ctx, "SELECT status='unknown' AND (SELECT count(*) FROM applications.workflow_definitions WHERE id=$1)=1 FROM applications.workflow_deletions WHERE flow_id=$1", f.flow).Scan(&unknown)
		return unknown, e
	})
	rootFormalStop(x.bff, true)
	rootFormalLog(t, "deletion-killed-bff", x.bff)
	rootFormalStop(x.java, true)
	rootFormalLog(t, "deletion-killed-java", x.java)
	x.java = rootFormalStartChild(t, "deletion-restarted-java", "java", x.javaArgs, x.javaEnv)
	x.waitEngine(t)
	x.bff = rootFormalStartChild(t, "deletion-restarted-bff", x.binary, nil, x.bffEnv)
	rootMainReady(t, x.bff, x.addr)
	x.proxy.deleteHeld.Store(false)
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_deletions SET next_attempt_at=clock_timestamp() WHERE flow_id=$1", f.flow); e != nil {
		t.Fatal(e)
	}
	rootFormalEventually(t, f.ctx, func() (bool, error) {
		d := rootHTTPData(t, f.call(t, "GET", base+"deletions/"+op, "", nil), 200)
		return rootHTTPString(t, d, "status") == "deleted", nil
	})
	final := rootHTTPData(t, f.call(t, "POST", base+"delete", string(raw), nil), 200)
	if len(final) != 7 || rootHTTPString(t, final, "status") != "deleted" || x.proxy.deletes.Load() != 1 {
		t.Fatal("recovery duplicated deletion or fabricated current result")
	}
	again := rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", body, nil), 200)
	if rootHTTPString(t, again, "commandId") != c.CommandID {
		t.Fatal("deleted configuration lost old approval receipt")
	}
	var defs, versions, instances, tasks, events, publications int
	if e := f.owner.QueryRow(f.ctx, `SELECT
 (SELECT count(*) FROM applications.workflow_definitions WHERE id=$1),
 (SELECT count(*) FROM applications.workflow_versions WHERE flow_id=$1),
 (SELECT count(*) FROM applications.workflow_instances WHERE flow_id=$1),
 (SELECT count(*) FROM applications.workflow_tasks WHERE instance_id=$2),
 (SELECT count(*) FROM applications.workflow_execution_events WHERE instance_id=$2),
 (SELECT count(*) FROM applications.workflow_publications WHERE flow_id=$1)`, f.flow, f.instance).Scan(&defs, &versions, &instances, &tasks, &events, &publications); e != nil {
		t.Fatal(e)
	}
	if defs+versions+instances+tasks != 0 || events == 0 || publications != 1 {
		t.Fatal("formal cleanup lost history or left executable catalog", defs, versions, instances, tasks, events, publications)
	}
}
