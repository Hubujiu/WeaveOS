package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowprojection"
	"github.com/jackc/pgx/v5/pgconn"
)

type rootProjection struct {
	recordFixture
	head                   workflowcatalog.Head
	instance               workflowcatalog.Instance
	version, first, second string
}

func rootProjectionFixture(t *testing.T) rootProjection {
	t.Helper()
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, false)
	next := recordOperationID(t, f)
	g.Nodes = append(g.Nodes, flowgraph.Node{ID: next, Kind: "approval", Approval: &flowgraph.Approval{Mode: "all", AssigneeIDs: []string{f.actor, f.other}}})
	g.Edges[1].To = next
	g.Edges = append(g.Edges, flowgraph.Edge{From: next, To: g.Nodes[2].ID})
	h := rootCatalogReady(t, f, g)
	inst := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var version string
	if err := f.owner.QueryRow(f.ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return rootProjection{f, h, inst, version, g.Nodes[1].ID, next}
}
func (f rootProjection) accept(t *testing.T, action, task, target string, epoch, seq int64) (flowcommands.Command, flowcommands.ExecutionPayload) {
	t.Helper()
	c := flowcommands.Command{ProtocolVersion: 2, CommandID: recordOperationID(t, f.recordFixture), AppID: f.app, TableID: f.table, ViewID: f.view, RecordID: f.ownRecord, InstanceID: f.instance.ID, ActorID: f.actor, Action: action, TaskID: task, TaskEpoch: epoch, ExpectedSequence: seq, RecordVersion: 1, SchemaVersion: 1, FlowID: f.head.FlowID, DefinitionVersion: 1, VersionID: f.version, TargetNodeID: target}
	p := flowcommands.ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("root-record-evidence:" + c.CommandID)), Routes: map[string]bool{}}
	if action == "start" {
		p.Start = &flowcommands.ExecutionStart{AllowWithdraw: true, Approvers: map[string][]string{f.first: {f.actor}}}
		if f.second != f.first {
			p.Start.Approvers[f.second] = []string{f.actor, f.other}
		}
	}
	raw, err := flowcommands.EncodeExecutionPayload(action, p)
	if err != nil {
		t.Fatal(err)
	}
	c.PayloadHash = sha256.Sum256(raw)
	tx, err := f.runtime.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if err = tx.QueryRow(f.ctx, rootAcquireSQL, f.app, f.table, f.view, f.ownRecord, c.CommandID, int64(1), int64(1)).Scan(&c.FenceEpoch); err != nil {
		t.Fatal(err)
	}
	if _, err = (flowcommands.Ledger{Namespace: "applications"}).AcceptInTx(f.ctx, tx, c); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	return c, p
}
func (f rootProjection) task(t *testing.T, node, actor string, epoch int64) flowcommands.ExecutionTask {
	id := recordOperationID(t, f.recordFixture)
	return flowcommands.ExecutionTask{ID: id, NodeID: node, AssigneeID: actor, EngineTaskID: "task-" + id, ActivationEpoch: epoch}
}
func rootProjectionBody(r flowcommands.ExecutionResult) []byte {
	out := []byte{'W', 'V', 'F', 'R', 'S', 'L', 0, 1}
	text := func(s string) {
		out = binary.BigEndian.AppendUint32(out, uint32(len(s)))
		out = append(out, []byte(s)...)
	}
	text(r.InstanceID)
	text(r.EngineProcessID)
	text(r.State)
	text(r.Reason)
	out = binary.BigEndian.AppendUint64(out, uint64(r.SchemaVersion))
	out = binary.BigEndian.AppendUint64(out, uint64(r.RecordVersion))
	out = binary.BigEndian.AppendUint32(out, uint32(len(r.Tasks)))
	for _, task := range r.Tasks {
		text(task.ID)
		text(task.NodeID)
		text(task.AssigneeID)
		text(task.EngineTaskID)
		out = binary.BigEndian.AppendUint64(out, uint64(task.ActivationEpoch))
	}
	return out
}
func (f rootProjection) receipt(t *testing.T, c flowcommands.Command, state, reason string, tasks ...flowcommands.ExecutionTask) (flowcommands.Receipt, []byte) {
	t.Helper()
	tasks = slices.Clone(tasks)
	slices.SortFunc(tasks, func(a, b flowcommands.ExecutionTask) int { return strings.Compare(a.ID, b.ID) })
	result := flowcommands.ExecutionResult{InstanceID: c.InstanceID, EngineProcessID: "process-" + c.InstanceID, State: state, Reason: reason, SchemaVersion: c.SchemaVersion, RecordVersion: c.RecordVersion, Tasks: tasks}
	outcome := "success"
	seq := c.ExpectedSequence + 1
	if state == "unchanged" {
		outcome = "no_effect"
		seq = c.ExpectedSequence
		result.EngineProcessID = ""
	}
	body := rootProjectionBody(result)
	hash, err := flowcommands.Fingerprint(c)
	if err != nil {
		t.Fatal(err)
	}
	return flowcommands.Receipt{CommandID: c.CommandID, CommandHash: hash, Outcome: outcome, Sequence: seq, ProofID: recordOperationID(t, f.recordFixture), ResultHash: sha256.Sum256(body)}, body
}
func (f rootProjection) apply(t *testing.T, c flowcommands.Command, p flowcommands.ExecutionPayload, r flowcommands.Receipt, b []byte, commit bool) workflowprojection.Applied {
	t.Helper()
	tx, err := f.runtime.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	got, err := (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Entry.Command != c || got.Entry.Receipt == nil || *got.Entry.Receipt != r || got.Entry.State != r.Outcome {
		t.Fatalf("incorrect confirmed result: %+v", got)
	}
	expected, err := flowcommands.DecodeExecutionResult(c, r, b)
	if err != nil || got.Result.InstanceID != expected.InstanceID || got.Result.State != expected.State {
		t.Fatalf("result does not match original body: %+v %v", got, err)
	}
	if commit {
		err = tx.Commit(f.ctx)
	} else {
		err = tx.Rollback(f.ctx)
	}
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func (f rootProjection) state(t *testing.T, state string, sequence int64, events, active, total int) {
	t.Helper()
	var got string
	var seq int64
	var process *string
	if e := f.owner.QueryRow(f.ctx, "SELECT state,sequence,engine_process_id FROM applications.workflow_instances WHERE app_id=$1 AND id=$2", f.app, f.instance.ID).Scan(&got, &seq, &process); e != nil || got != state || seq != sequence {
		t.Fatalf("state %s/%d want %s/%d: %v", got, seq, state, sequence, e)
	}
	if state == "active" || state == "completed" || state == "rejected" || state == "withdrawn" {
		if process == nil || *process != "process-"+f.instance.ID {
			t.Fatal("engine binding missing")
		}
	}
	for _, v := range []struct {
		q string
		n int
	}{{"SELECT count(*) FROM applications.workflow_execution_events WHERE app_id=$1 AND instance_id=$2", events}, {"SELECT count(*) FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2 AND closed_command_id IS NULL", active}, {"SELECT count(*) FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2", total}} {
		var n int
		if e := f.owner.QueryRow(f.ctx, v.q, f.app, f.instance.ID).Scan(&n); e != nil || n != v.n {
			t.Fatalf("%s got%d want%d: %v", v.q, n, v.n, e)
		}
	}
}
func (f rootProjection) ledger(t *testing.T, c flowcommands.Command, state string, dispatch, fences int) {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	got, e := (flowcommands.Ledger{Namespace: "applications"}).GetInTx(f.ctx, tx, c.CommandID)
	if e != nil || got.State != state {
		t.Fatalf("ledger %s want%s: %v", got.State, state, e)
	}
	var n int
	if e = tx.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_dispatch WHERE command_id=$1", c.CommandID).Scan(&n); e != nil || n != dispatch {
		t.Fatalf("dispatch %d want%d %v", n, dispatch, e)
	}
	if e = tx.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1 AND record_id=$2", f.app, f.ownRecord).Scan(&n); e != nil || n != fences {
		t.Fatalf("fence %d want%d %v", n, fences, e)
	}
}
func (f rootProjection) start(t *testing.T) flowcommands.ExecutionTask {
	c, p := f.accept(t, "start", "", "", 0, 0)
	task := f.task(t, f.first, f.actor, 1)
	r, b := f.receipt(t, c, "active", "", task)
	f.apply(t, c, p, r, b, true)
	return task
}

func TestRootProjectionStartCommitsBoundTasksEventAndFence(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	task := f.task(t, f.first, f.actor, 1)
	r, b := f.receipt(t, c, "active", "", task)
	got := f.apply(t, c, p, r, b, true)
	if got.Duplicate {
		t.Fatal("first application duplicate")
	}
	f.state(t, "active", 1, 1, 1, 1)
	f.ledger(t, c, "success", 0, 0)
	var actor, action, proof string
	var schema, record int64
	var evidence, payload, result []byte
	e := f.owner.QueryRow(f.ctx, "SELECT actor_id::text,action,proof_id::text,schema_version,record_version,evidence_hash,payload_bytes,result_bytes FROM applications.workflow_execution_events WHERE command_id=$1", c.CommandID).Scan(&actor, &action, &proof, &schema, &record, &evidence, &payload, &result)
	want, _ := flowcommands.EncodeExecutionPayload(c.Action, p)
	if e != nil || actor != c.ActorID || action != "start" || proof != r.ProofID || schema != 1 || record != 1 || !bytes.Equal(evidence, p.EvidenceHash[:]) || !bytes.Equal(payload, want) || !bytes.Equal(result, b) {
		t.Fatalf("immutable command evidence binding incorrect: %v", e)
	}
}
func TestRootProjectionCallerRollbackKeepsPending(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
	f.apply(t, c, p, r, b, false)
	f.state(t, "starting", 0, 0, 0, 0)
	f.ledger(t, c, "pending", 1, 1)
}
func TestRootProjectionDuplicateDoesNotReleaseNewFence(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	task := f.task(t, f.first, f.actor, 1)
	r, b := f.receipt(t, c, "active", "", task)
	f.apply(t, c, p, r, b, true)
	next, _ := f.accept(t, "agree", task.ID, "", 1, 1)
	if !f.apply(t, c, p, r, b, true).Duplicate {
		t.Fatal("exact replay not identified")
	}
	f.state(t, "active", 1, 1, 1, 1)
	f.ledger(t, next, "pending", 1, 1)
}
func TestRootProjectionTransitionsPreserveTaskHistory(t *testing.T) {
	f := rootProjectionFixture(t)
	old := f.start(t)
	c, p := f.accept(t, "agree", old.ID, "", 1, 1)
	a := f.task(t, f.second, f.actor, 2)
	other := f.task(t, f.second, f.other, 2)
	r, b := f.receipt(t, c, "active", "", a, other)
	f.apply(t, c, p, r, b, true)
	f.state(t, "active", 2, 2, 2, 3)
	c, p = f.accept(t, "agree", a.ID, "", 2, 2)
	r, b = f.receipt(t, c, "active", "", other)
	f.apply(t, c, p, r, b, true)
	f.state(t, "active", 3, 3, 1, 3)
	var closed string
	if e := f.owner.QueryRow(f.ctx, "SELECT closed_command_id::text FROM applications.workflow_tasks WHERE id=$1", a.ID).Scan(&closed); e != nil || closed != c.CommandID {
		t.Fatalf("wrong closed command: %v", e)
	}
}
func TestRootProjectionTerminalActionsRemoveOnlyActiveTasks(t *testing.T) {
	for _, v := range []struct{ action, state string }{{"agree", "completed"}, {"reject", "rejected"}, {"withdraw", "withdrawn"}} {
		t.Run(v.action, func(t *testing.T) {
			f := rootProjectionFixture(t)
			task := f.start(t)
			id, epoch := task.ID, int64(1)
			if v.action == "withdraw" {
				id = ""
				epoch = 0
			}
			c, p := f.accept(t, v.action, id, "", epoch, 1)
			r, b := f.receipt(t, c, v.state, "")
			f.apply(t, c, p, r, b, true)
			f.state(t, v.state, 2, 2, 0, 1)
			f.ledger(t, c, "success", 0, 0)
		})
	}
}
func TestRootProjectionConfirmedNoEffectLeavesActiveState(t *testing.T) {
	f := rootProjectionFixture(t)
	task := f.start(t)
	c, p := f.accept(t, "agree", task.ID, "", 1, 1)
	r, b := f.receipt(t, c, "unchanged", "cancelled")
	f.apply(t, c, p, r, b, true)
	f.state(t, "active", 1, 2, 1, 1)
	f.ledger(t, c, "no_effect", 0, 0)
}
func TestRootProjectionCancelledStartDrainsReservation(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	r, b := f.receipt(t, c, "unchanged", "cancelled")
	f.apply(t, c, p, r, b, true)
	f.state(t, "no_effect", 0, 1, 0, 0)
	f.ledger(t, c, "no_effect", 0, 0)
}
func TestRootProjectionReturnCreatesNewActivation(t *testing.T) {
	f := rootProjectionFixture(t)
	old := f.start(t)
	c, p := f.accept(t, "return", old.ID, f.first, 1, 1)
	fresh := f.task(t, f.first, f.actor, 2)
	r, b := f.receipt(t, c, "active", "", fresh)
	f.apply(t, c, p, r, b, true)
	f.state(t, "active", 2, 2, 1, 2)
}
func TestRootProjectionHistoricalReturnBindsTarget(t *testing.T) {
	f := rootProjectionFixture(t)
	old := f.start(t)
	c, p := f.accept(t, "agree", old.ID, "", 1, 1)
	current := f.task(t, f.second, f.actor, 2)
	r, b := f.receipt(t, c, "active", "", current)
	f.apply(t, c, p, r, b, true)
	c, p = f.accept(t, "return", old.ID, f.first, 1, 2)
	r, b = f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 3))
	f.apply(t, c, p, r, b, true)
	f.state(t, "active", 3, 3, 1, 3)
}
func TestRootProjectionNoEffectUnknownStartCannotHideExistingEngine(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	r, b := f.receipt(t, c, "unchanged", "instance_exists")
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b); !errors.Is(e, workflowprojection.ErrConflict) {
		t.Fatalf("ambiguous existing instance: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	f.state(t, "starting", 0, 0, 0, 0)
	f.ledger(t, c, "pending", 1, 1)
}
func TestRootProjectionInvalidBindingsNeverWrite(t *testing.T) {
	for _, name := range []string{"payload", "body", "receipt", "instance", "sequence", "schema", "record", "action_state", "process"} {
		t.Run(name, func(t *testing.T) {
			f := rootProjectionFixture(t)
			task := f.start(t)
			c, p := f.accept(t, "agree", task.ID, "", 1, 1)
			r, b := f.receipt(t, c, "completed", "")
			switch name {
			case "payload":
				p.EvidenceHash[0] ^= 1
			case "body":
				b[len(b)-1] ^= 1
			case "receipt":
				r.CommandHash[0] ^= 1
			case "sequence":
				r.Sequence++
			case "instance":
				c.InstanceID = recordOperationID(t, f.recordFixture)
			case "schema":
				c.SchemaVersion++
			case "record":
				c.RecordVersion++
			case "action_state":
				r, b = f.receipt(t, c, "withdrawn", "")
			case "process":
				z := flowcommands.ExecutionResult{InstanceID: c.InstanceID, EngineProcessID: "another-process", State: "completed", SchemaVersion: 1, RecordVersion: 1}
				b = rootProjectionBody(z)
				r.ResultHash = sha256.Sum256(b)
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			if _, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b); e == nil {
				t.Fatal("invalid binding applied")
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatal("failed operation poisoned outer tx", e)
			}
			f.state(t, "active", 1, 1, 1, 1)
		})
	}
}
func TestRootProjectionFailureSavepointPreventsPartialCommit(t *testing.T) {
	for _, stage := range []string{"event", "task", "instance", "ledger", "dispatch", "fence"} {
		t.Run(stage, func(t *testing.T) {
			f := rootProjectionFixture(t)
			c, p := f.accept(t, "start", "", "", 0, 0)
			r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
			suffix := strings.ReplaceAll(c.CommandID, "-", "")
			fn := "root_proj_fault_" + suffix
			table, op, field, value := "workflow_execution_events", "INSERT", "NEW.app_id", f.app
			switch stage {
			case "task":
				table = "workflow_tasks"
			case "instance":
				table = "workflow_instances"
				op = "UPDATE"
				field = "NEW.id"
				value = f.instance.ID
			case "ledger":
				table = "workflow_commands"
				op = "UPDATE"
				field = "NEW.command_id"
				value = c.CommandID
			case "dispatch":
				table = "workflow_dispatch"
				op = "DELETE"
				field = "OLD.command_id"
				value = c.CommandID
			case "fence":
				table = "record_command_fences"
				op = "DELETE"
				field = "OLD.command_id"
				value = c.CommandID
			}
			q := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root injected projection failure' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER %s ON applications.%s FOR EACH ROW WHEN (%s='%s'::uuid) EXECUTE FUNCTION applications.%s()", fn, fn, op, table, field, value, fn)
			if _, e := f.owner.Exec(f.ctx, q); e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() {
				_, _ = f.owner.Exec(context.Background(), "DROP TRIGGER IF EXISTS "+fn+" ON applications."+table+"; DROP FUNCTION IF EXISTS applications."+fn+"()")
			})
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			if _, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b); e == nil {
				t.Fatal("injected failure hidden")
			}
			var fault *pgconn.PgError
			if !errors.As(e, &fault) || fault.Code != "P0001" || !strings.Contains(fault.Message, "root injected projection failure") {
				t.Fatalf("required write-stage fault not reached: %v", e)
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatal("savepoint did not recover outer tx", e)
			}
			f.state(t, "starting", 0, 0, 0, 0)
			f.ledger(t, c, "pending", 1, 1)
		})
	}
}
func TestRootProjectionConcurrentExactReceiptHasOneEffect(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	dups := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				errs <- e
				return
			}
			defer tx.Rollback(f.ctx)
			got, e := (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b)
			if e == nil {
				e = tx.Commit(f.ctx)
			}
			errs <- e
			dups <- got.Duplicate
		}()
	}
	wg.Wait()
	close(errs)
	close(dups)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	n := 0
	for d := range dups {
		if d {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("duplicate count%d", n)
	}
	f.state(t, "active", 1, 1, 1, 1)
}
func TestRootProjectionNilPortsAndInvalidVersion(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
	if _, e := (workflowprojection.Store{}).ApplyInTx(f.ctx, nil, c, p, r, b); e == nil {
		t.Fatal("nil transaction accepted")
	}
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = (workflowprojection.Store{}).ApplyInTx(nil, tx, c, p, r, b); e == nil {
		t.Fatal("nil context accepted")
	}
	c.ProtocolVersion = 1
	if _, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b); e == nil {
		t.Fatal("v1 accepted")
	}
	f.state(t, "starting", 0, 0, 0, 0)
}
func TestRootProjectionMissingFenceCannotPublish(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
	rootFenceRelease(t, f.recordFixture, c.CommandID, c.FenceEpoch, true)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b); e == nil {
		t.Fatal("missing fence published confirmed result")
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	f.state(t, "starting", 0, 0, 0, 0)
	f.ledger(t, c, "pending", 1, 0)
}
func TestRootProjectionTaskIdentityCannotBeRewrittenOrReactivated(t *testing.T) {
	for _, name := range []string{"assignee", "engine", "epoch", "reactivate", "foreign_node"} {
		t.Run(name, func(t *testing.T) {
			f := rootProjectionFixture(t)
			old := f.start(t)
			c, p := f.accept(t, "agree", old.ID, "", 1, 1)
			next := f.task(t, f.second, f.actor, 2)
			r, b := f.receipt(t, c, "active", "", next)
			f.apply(t, c, p, r, b, true)
			c, p = f.accept(t, "return", next.ID, f.first, 2, 2)
			bad := next
			switch name {
			case "assignee":
				bad.AssigneeID = f.other
			case "engine":
				bad.EngineTaskID = "different"
			case "epoch":
				bad.ActivationEpoch = 3
			case "reactivate":
				bad = old
			case "foreign_node":
				bad = f.task(t, recordOperationID(t, f.recordFixture), f.actor, 3)
			}
			r, b = f.receipt(t, c, "active", "", bad)
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			if _, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b); e == nil {
				t.Fatal("task identity rewritten/reactivated")
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatal(e)
			}
			f.state(t, "active", 2, 2, 1, 2)
			f.ledger(t, c, "pending", 1, 1)
		})
	}
}
func TestRootProjectionIndependentFlowsKeepSeparateResults(t *testing.T) {
	a := rootProjectionFixture(t)
	at := a.start(t)
	b := a
	b.head = rootCatalogReady(t, b.recordFixture, rootCatalogGraph(t, b.recordFixture, false))
	b.instance = rootCatalogReserve(t, b.recordFixture, rootCatalogReserveInput(t, b.recordFixture, b.head))
	var raw []byte
	if e := b.owner.QueryRow(b.ctx, "SELECT version_id::text,graph_json FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", b.app, b.head.FlowID).Scan(&b.version, &raw); e != nil {
		t.Fatal(e)
	}
	// The second flow needs its own actual definition node. Read only that graph.
	var graph flowgraph.Graph
	if e := json.Unmarshal(raw, &graph); e != nil {
		t.Fatal(e)
	}
	b.first = graph.Nodes[1].ID
	b.second = b.first
	c, p := b.accept(t, "start", "", "", 0, 0)
	r, body := b.receipt(t, c, "active", "", b.task(t, b.first, b.actor, 1))
	b.apply(t, c, p, r, body, true)
	c, p = a.accept(t, "reject", at.ID, "", 1, 1)
	r, body = a.receipt(t, c, "rejected", "")
	a.apply(t, c, p, r, body, true)
	a.state(t, "rejected", 2, 2, 0, 1)
	b.state(t, "active", 1, 1, 1, 1)
}
func TestRootProjectionRuntimeEventsAreAppendOnly(t *testing.T) {
	f := rootProjectionFixture(t)
	for _, privilege := range []string{"UPDATE", "DELETE"} {
		var yes bool
		if e := f.runtime.QueryRow(f.ctx, "SELECT has_table_privilege(current_user,'applications.workflow_execution_events',$1)", privilege).Scan(&yes); e != nil || yes {
			t.Fatalf("runtime event privilege %s=%v: %v", privilege, yes, e)
		}
	}
	var yes bool
	if e := f.runtime.QueryRow(f.ctx, "SELECT has_column_privilege(current_user,'applications.workflow_tasks','assignee_id','UPDATE')").Scan(&yes); e != nil || yes {
		t.Fatalf("task identity mutable: %v", e)
	}
}

func TestRootProjectionAssigneeMustBelongToVersionNode(t *testing.T) {
	f := rootProjectionFixture(t)
	c, p := f.accept(t, "start", "", "", 0, 0)
	// f.other exists but is not assigned to this version's first approval node.
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.other, 1))
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = (workflowprojection.Store{}).ApplyInTx(f.ctx, tx, c, p, r, b); !errors.Is(e, workflowprojection.ErrConflict) {
		t.Fatalf("foreign node assignee accepted: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	f.state(t, "starting", 0, 0, 0, 0)
	f.ledger(t, c, "pending", 1, 1)
}
func TestRootProjectionForeignInstanceDoesNotAcquireItsLock(t *testing.T) {
	f := rootProjectionFixture(t)
	other := rootProjectionFixture(t)
	blocker, e := other.runtime.Begin(other.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer blocker.Rollback(other.ctx)
	var id string
	if e = blocker.QueryRow(other.ctx, "SELECT id::text FROM applications.workflow_instances WHERE app_id=$1 AND id=$2 FOR UPDATE", other.app, other.instance.ID).Scan(&id); e != nil {
		t.Fatal(e)
	}
	c, p := f.accept(t, "start", "", "", 0, 0)
	c.InstanceID = other.instance.ID
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
	ctx, cancel := context.WithTimeout(f.ctx, 2*time.Second)
	defer cancel()
	tx, e := f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = (workflowprojection.Store{}).ApplyInTx(ctx, tx, c, p, r, b); !errors.Is(e, workflowprojection.ErrConflict) {
		t.Fatalf("cross-app lookup must reject without waiting on foreign row: %v", e)
	}
}
func TestRootProjectionVisitedNodeHasScopedIndex(t *testing.T) {
	f := rootProjectionFixture(t)
	var yes bool
	e := f.owner.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='applications' AND c.relname='workflow_tasks' AND i.indisvalid AND i.indpred IS NULL AND (SELECT array_agg(a.attname::text ORDER BY k.ord) FROM unnest(i.indkey::smallint[]) WITH ORDINALITY k(attnum,ord) JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=k.attnum)=ARRAY['app_id','instance_id','node_id']::text[])`).Scan(&yes)
	if e != nil || !yes {
		t.Fatalf("visited-node lookup lacks bounded scoped index: %v", e)
	}
}
