//go:build workflowruntime_integration && workflowrpc_integration && linux

package main

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/jackc/pgx/v5"
	"io"
	"strings"
	"testing"
)

func TestRootFormalRuntimeNodeSaveThenApproveLatest(t *testing.T) {
	x := rootFormalSetupEditable(t, true)
	f := x.f
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	before := x.proxy.executions.Load()
	op := f.id(t)
	body := rootHTTPSaveBody(op, token, f.field, "finance corrected")
	saved := rootHTTPData(t, f.call(t, "PATCH", f.taskPath()+"/record", body, nil), 200)
	if string(saved["recordVersion"]) != "2" {
		t.Fatal("formal Save did not confirm v2")
	}
	if x.proxy.executions.Load() != before {
		t.Fatal("Save called the engine")
	}
	var state string
	var sequence int64
	if e := f.owner.QueryRow(f.ctx, "SELECT state,sequence FROM applications.workflow_instances WHERE id=$1", f.instance).Scan(&state, &sequence); e != nil || state != "active" || sequence != 1 {
		t.Fatal("Save advanced the current node", e)
	}
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", token), nil), 409, "WORKFLOW_BASIS_CHANGED")
	// A later engine outage must not undo an already committed local Save.
	fresh := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	approvalID := f.id(t)
	approvalBody := rootHTTPActionBody(approvalID, "agree", rootHTTPString(t, fresh, "basisToken"))
	rootFormalStop(x.java, true)
	rootFormalEventually(t, f.ctx, func() (bool, error) {
		r, e := f.client.Get(f.server.URL + "/health/ready")
		if e != nil {
			return false, e
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		return r.StatusCode == 503, nil
	})
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", approvalBody, nil), 503, "COMMON_SERVICE_UNAVAILABLE")
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.field, "-", "")}.Sanitize()
	var value string
	var version int64
	if e := f.owner.QueryRow(f.ctx, "SELECT "+column+",record_version FROM "+relation+" WHERE id=$1", f.record).Scan(&value, &version); e != nil || value != "finance corrected" || version != 2 {
		t.Fatal("failed approval lost committed Save", e)
	}
	x.java = rootFormalStartChild(t, "save-restarted-java", "java", x.javaArgs, x.javaEnv)
	x.waitEngine(t)
	rootMainReady(t, x.bff, x.addr)
	accepted := rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", approvalBody, nil), 202)
	commandID := rootHTTPString(t, accepted, "commandId")
	rootFormalEventually(t, f.ctx, func() (bool, error) {
		data := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-workflow-operations/"+approvalID, "", nil), 200)
		return rootHTTPString(t, data, "status") == "success", nil
	})
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT command_json FROM applications.workflow_commands WHERE command_id=$1", commandID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var command flowcommands.Command
	if json.Unmarshal(raw, &command) != nil || command.RecordVersion != 2 {
		t.Fatal("approval did not bind the newly seen v2")
	}
	receipt, found, e := x.execution.Lookup(f.ctx, command)
	if e != nil || !found || receipt == nil || receipt.Result.State != "completed" {
		t.Fatal("actual Java engine did not confirm approval", e)
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_change_events WHERE app_id=$1 AND operation_id=$2 AND origin='task_save' AND record_version_before=1 AND record_version_after=2", f.app, op).Scan(&n); e != nil || n != 1 {
		t.Fatal("Save history not exact", e)
	}
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events WHERE command_id=$1 AND record_version=2 AND schema_version=1", commandID).Scan(&n); e != nil || n != 1 {
		t.Fatal("approval evidence did not preserve v2", e)
	}
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 0 {
		t.Fatal("completed approval retained fence", e)
	}
	replay := rootHTTPData(t, f.call(t, "PATCH", f.taskPath()+"/record", body, nil), 200)
	if string(replay["recordVersion"]) != "2" {
		t.Fatal("closed-task Save replay changed receipt")
	}
}
