//go:build workflowrpc_integration

package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	fg "github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	we "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowexecution"
	wr "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// This is an isolated real Java/Flowable fixture, never a production connection.
func rootJavaWorkerFixture(t *testing.T) (rootProjection, *wr.ExecutionClient) {
	t.Helper()
	target := os.Getenv("WEAVEOS_RPC_TEST_TARGET")
	host, _, e := net.SplitHostPort(target)
	if e != nil || host != "b3-workflow" {
		t.Fatal("dedicated unexposed b3-workflow fixture required")
	}
	conn, e := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { conn.Close() })
	deploy, e := wr.NewClient(conn, 15*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	client, e := wr.NewExecutionClient(conn, 15*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, true)
	next := recordOperationID(t, f)
	g.Nodes = append(g.Nodes, fg.Node{ID: next, Kind: "approval", Approval: &fg.Approval{Mode: "all", AssigneeIDs: []string{f.actor, f.other}}})
	g.Edges[1].To = next
	g.Edges = append(g.Edges, fg.Edge{From: next, To: g.Nodes[2].ID})
	h := rootCatalogPut(t, f, recordOperationID(t, f), 0, g)
	var version string
	var bpmn []byte
	if e = f.runtime.QueryRow(f.ctx, "SELECT version_id::text,bpmn_xml FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version, &bpmn); e != nil {
		t.Fatal(e)
	}
	receipt, e := deploy.Deploy(f.ctx, &pb.DeployRequest{AppId: f.app, FlowId: h.FlowID, VersionId: version, Version: 1, BpmnXml: bpmn})
	if e != nil {
		t.Fatal(e)
	}
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		h, err = (wc.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, 1, receipt.EngineDeploymentId)
		return err
	})
	h = rootCatalogEnable(t, f, h)
	instance := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	return rootProjection{f, h, instance, version, g.Nodes[1].ID, next}, client
}
func rootJavaWorker(f rootProjection, client we.ExecutionClient) *we.Worker {
	return &we.Worker{Pool: f.runtime, Client: client, RPCTimeout: 20 * time.Second, Limits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}}
}
func rootJavaAcceptedAction(t *testing.T, f rootProjection, previous fc.Command, r *wr.ExecutionConfirmed, task fc.ExecutionTask, version int64) fc.Command {
	t.Helper()
	c := previous
	c.CommandID = recordOperationID(t, f.recordFixture)
	c.Action = "agree"
	c.TaskID = task.ID
	c.TaskEpoch = task.ActivationEpoch
	c.ActorID = task.AssigneeID
	c.ExpectedSequence = r.Receipt.Sequence
	c.RecordVersion = version
	c.TargetNodeID = ""
	p := fc.ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("root-real-java-evidence:" + c.CommandID)), Routes: map[string]bool{}}
	raw, e := fc.EncodeExecutionPayload(c.Action, p)
	if e != nil {
		t.Fatal(e)
	}
	c.PayloadHash = sha256.Sum256(raw)
	rootCatalogTx(t, f.recordFixture, func(tx pgx.Tx) error {
		if e := tx.QueryRow(f.ctx, rootAcquireSQL, f.app, f.table, f.view, f.ownRecord, c.CommandID, int64(1), version).Scan(&c.FenceEpoch); e != nil {
			return e
		}
		_, e := (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(f.ctx, tx, c, p)
		return e
	})
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), "UPDATE applications.workflow_dispatch SET next_attempt_at='9999-12-31T00:00:00Z' WHERE command_id=$1", c.CommandID)
	})
	return c
}
func rootJavaConfirmed(t *testing.T, f rootProjection, client *wr.ExecutionClient, c fc.Command, eventCount int) *wr.ExecutionConfirmed {
	t.Helper()
	r, found, e := client.Lookup(f.ctx, c)
	if e != nil || !found || r == nil {
		t.Fatalf("durable engine result missing: %v %v", found, e)
	}
	f.ledger(t, c, "success", 0, 0)
	var state, process string
	var seq int64
	if e = f.runtime.QueryRow(f.ctx, "SELECT state,sequence,engine_process_id FROM applications.workflow_instances WHERE id=$1", f.instance.ID).Scan(&state, &seq, &process); e != nil || state != r.Result.State || seq != r.Receipt.Sequence || process != r.Result.EngineProcessID || process == "" {
		t.Fatalf("application/engine state differs: %s %d %s %v", state, seq, process, e)
	}
	var body []byte
	var recordVersion int64
	if e = f.runtime.QueryRow(f.ctx, "SELECT result_bytes,record_version FROM applications.workflow_execution_events WHERE command_id=$1", c.CommandID).Scan(&body, &recordVersion); e != nil || !bytes.Equal(body, r.ResultBytes) || recordVersion != c.RecordVersion {
		t.Fatalf("original bytes/version lost: %v", e)
	}
	var n int
	if e = f.runtime.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events WHERE instance_id=$1", f.instance.ID).Scan(&n); e != nil || n != eventCount {
		t.Fatalf("event count %d want%d: %v", n, eventCount, e)
	}
	rows, e := f.runtime.Query(f.ctx, "SELECT id::text,node_id::text,assignee_id::text,engine_task_id,activation_epoch FROM applications.workflow_tasks WHERE instance_id=$1 AND closed_command_id IS NULL ORDER BY id", f.instance.ID)
	if e != nil {
		t.Fatal(e)
	}
	tasks := []fc.ExecutionTask{}
	for rows.Next() {
		var task fc.ExecutionTask
		if e = rows.Scan(&task.ID, &task.NodeID, &task.AssigneeID, &task.EngineTaskID, &task.ActivationEpoch); e != nil {
			rows.Close()
			t.Fatal(e)
		}
		tasks = append(tasks, task)
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(tasks) != len(r.Result.Tasks) {
		t.Fatalf("tasks differ: %v", e)
	}
	for i := range tasks {
		if tasks[i] != r.Result.Tasks[i] {
			t.Fatalf("task %d differs", i)
		}
	}
	return r
}
func rootJavaDispatch(t *testing.T, f rootProjection, client we.ExecutionClient) {
	t.Helper()
	if worked, e := rootJavaWorker(f, client).DispatchOne(f.ctx); e != nil || !worked {
		t.Fatalf("real worker dispatch: %v %v", worked, e)
	}
}

func TestRootRecoveryJavaLifecycleInterop(t *testing.T) {
	begin := time.Now()
	f, client := rootJavaWorkerFixture(t)
	c, _ := rootRecoveryAccept(t, f)
	rootJavaDispatch(t, f, client)
	r := rootJavaConfirmed(t, f, client, c, 1)
	if r.Result.State != "active" || len(r.Result.Tasks) != 1 {
		t.Fatal("real first node did not activate")
	}
	// Exercise the existing authorized record-edit path between two decisions.
	// Node-specific user command admission remains a separate integration layer.
	edit, e := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f.recordFixture), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "revised before next approval"}}, applications.Metadata{RequestID: "root-real-worker-edit"})
	if e != nil || edit.RecordVersion != 2 {
		t.Fatalf("record edit failed: %+v %v", edit, e)
	}
	for i, want := range []int{2, 1, 0} {
		if len(r.Result.Tasks) == 0 {
			t.Fatal("flow ended before all approvals")
		}
		c = rootJavaAcceptedAction(t, f, c, r, r.Result.Tasks[0], 2)
		rootJavaDispatch(t, f, client)
		r = rootJavaConfirmed(t, f, client, c, i+2)
		if len(r.Result.Tasks) != want || r.Result.RecordVersion != 2 {
			t.Fatalf("step%d latest version/tasks lost", i)
		}
	}
	if r.Result.State != "completed" || r.Receipt.Sequence != 4 {
		t.Fatal("real flow did not finish")
	}
	var versions []int64
	rows, e := f.runtime.Query(f.ctx, "SELECT record_version FROM applications.workflow_execution_events WHERE instance_id=$1 ORDER BY sequence", f.instance.ID)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var v int64
		if e = rows.Scan(&v); e != nil {
			rows.Close()
			t.Fatal(e)
		}
		versions = append(versions, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil || !reflect.DeepEqual(versions, []int64{1, 2, 2, 2}) {
		t.Fatalf("historical decision versions lost: %v %v", versions, e)
	}
	if worked, e := rootJavaWorker(f, client).DispatchOne(f.ctx); e != nil || worked {
		t.Fatal("completed command remained queued")
	}
	t.Logf("real deployment + durable start + record edit + three decisions + application projection elapsed=%s", time.Since(begin))
}

func TestRootRecoveryJavaDroppedResponseInterop(t *testing.T) {
	f, real := rootJavaWorkerFixture(t)
	c, _ := rootRecoveryAccept(t, f)
	var committed *wr.ExecutionConfirmed
	first := &rootRecoveryClient{lookup: real.Lookup, execute: func(ctx context.Context, cmd fc.Command, p fc.ExecutionPayload) (*wr.ExecutionConfirmed, error) {
		var e error
		committed, e = real.Execute(ctx, cmd, p)
		if e != nil {
			return nil, e
		}
		return nil, context.DeadlineExceeded // Deliberately discard a real successful reply at the client boundary.
	}}
	worked, e := rootJavaWorker(f, first).DispatchOne(f.ctx)
	if !worked || !errors.Is(e, context.DeadlineExceeded) || committed == nil {
		t.Fatalf("did not reach deliberate lost-response boundary: %v", e)
	}
	rootWorkerPending(t, f, c, "DEPENDENCY_UNAVAILABLE")
	rootWorkerDue(t, f, c)
	second := &rootRecoveryClient{lookup: real.Lookup, execute: real.Execute}
	rootJavaDispatch(t, f, second)
	recovered := rootJavaConfirmed(t, f, real, c, 1)
	if !reflect.DeepEqual(committed, recovered) || first.executes.Load() != 1 || second.executes.Load() != 0 {
		t.Fatal("recovery changed result or executed again")
	}
	// An explicit transport replay also returns the same original engine facts.
	p := rootRecoveryPayload(t, f, c)
	again, e := real.Execute(f.ctx, c, p)
	if e != nil || !reflect.DeepEqual(committed, again) {
		t.Fatalf("real engine replay changed effect: %v", e)
	}
	rootJavaConfirmed(t, f, real, c, 1)
}

func TestRootRecoveryJavaApplicationCommitFaultInterop(t *testing.T) {
	f, real := rootJavaWorkerFixture(t)
	c, _ := rootRecoveryAccept(t, f)
	fn := "root_real_worker_" + strings.ReplaceAll(c.CommandID, "-", "")
	query := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root real Java projection fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER INSERT ON applications.workflow_execution_events FOR EACH ROW WHEN (NEW.command_id='%s'::uuid) EXECUTE FUNCTION applications.%s()", fn, fn, c.CommandID, fn)
	if _, e := f.owner.Exec(f.ctx, query); e != nil {
		t.Fatal(e)
	}
	cleanup := func() error {
		_, e := f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON applications.workflow_execution_events; DROP FUNCTION IF EXISTS applications.%s()", fn, fn))
		return e
	}
	t.Cleanup(func() {
		if e := cleanup(); e != nil {
			t.Error(e)
		}
	})
	first := &rootRecoveryClient{lookup: real.Lookup, execute: real.Execute}
	worked, e := rootJavaWorker(f, first).DispatchOne(f.ctx)
	var fault *pgconn.PgError
	if !worked || !errors.As(e, &fault) || fault.Code != "P0001" || !strings.Contains(fault.Message, "root real Java projection fault") {
		t.Fatalf("actual application fault not reached: %v", e)
	}
	rootWorkerPending(t, f, c, "APPLICATION_CONFIRMATION_FAILED")
	before, found, e := real.Lookup(f.ctx, c)
	if e != nil || !found || before == nil {
		t.Fatalf("real engine did not commit before application fault: %v", e)
	}
	if e = cleanup(); e != nil {
		t.Fatal(e)
	}
	rootWorkerDue(t, f, c)
	second := &rootRecoveryClient{lookup: real.Lookup, execute: real.Execute}
	rootJavaDispatch(t, f, second)
	after := rootJavaConfirmed(t, f, real, c, 1)
	if !reflect.DeepEqual(before, after) || first.executes.Load() != 1 || second.executes.Load() != 0 {
		t.Fatal("application retry repeated engine execution or changed original proof")
	}
}
