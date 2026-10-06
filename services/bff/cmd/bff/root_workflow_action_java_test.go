//go:build workflowrpc_integration

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	we "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowexecution"
	wr "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type rootHTTPRealClient struct {
	client     *wr.ExecutionClient
	drop       bool
	executions int
	lookups    int
}

func (c *rootHTTPRealClient) Lookup(ctx context.Context, command fc.Command) (*wr.ExecutionConfirmed, bool, error) {
	c.lookups++
	return c.client.Lookup(ctx, command)
}
func (c *rootHTTPRealClient) Execute(ctx context.Context, command fc.Command, payload fc.ExecutionPayload) (*wr.ExecutionConfirmed, error) {
	c.executions++
	r, e := c.client.Execute(ctx, command, payload)
	if e == nil && c.drop {
		c.drop = false
		return nil, errors.New("Root discarded genuine successful Java response")
	}
	return r, e
}
func rootHTTPWorker(f *rootTaskHTTPFixture, c we.ExecutionClient) *we.Worker {
	return &we.Worker{Pool: f.runtime, Client: c, RPCTimeout: 20 * time.Second, Limits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}}
}
func rootHTTPJavaSetup(t *testing.T) (*rootTaskHTTPFixture, *wr.ExecutionClient) {
	t.Helper()
	target := os.Getenv("WEAVEOS_RPC_TEST_TARGET")
	host, _, e := net.SplitHostPort(target)
	if e != nil || host != "b3-workflow" {
		t.Fatal("dedicated unexposed Java fixture required")
	}
	connection, e := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { connection.Close() })
	deployment, e := wr.NewClient(connection, 15*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	client, e := wr.NewExecutionClient(connection, 15*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	f := rootHTTPResourceSetup(t)
	f.flow, f.instance, f.node = f.id(t), f.id(t), f.id(t)
	start, end := f.id(t), f.id(t)
	graph := flowgraph.Graph{Version: 1, Nodes: []flowgraph.Node{{ID: start, Kind: "start"}, {ID: f.node, Kind: "approval", Approval: &flowgraph.Approval{Mode: "all", AssigneeIDs: []string{f.actor}}}, {ID: end, Kind: "end"}}, Edges: []flowgraph.Edge{{From: start, To: f.node}, {From: f.node, To: end}}}
	var head wc.Head
	f.transaction(t, func(tx pgx.Tx) error {
		var err error
		head, err = (wc.Catalog{}).PutVersionInTx(f.ctx, tx, wc.VersionInput{AppID: f.app, TableID: f.table, ViewID: f.view, FlowID: f.flow, ActorID: f.actor, Name: "Real HTTP approval", ExpectedSchemaVersion: 1, Graph: graph, AllowWithdraw: true})
		return err
	})
	var bpmn []byte
	if e = f.owner.QueryRow(f.ctx, "SELECT version_id::text,bpmn_xml FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, f.flow).Scan(&f.version, &bpmn); e != nil {
		t.Fatal(e)
	}
	confirmed, e := deployment.Deploy(f.ctx, &pb.DeployRequest{AppId: f.app, FlowId: f.flow, VersionId: f.version, Version: 1, BpmnXml: bpmn})
	if e != nil {
		t.Fatal(e)
	}
	f.transaction(t, func(tx pgx.Tx) error {
		var err error
		head, err = (wc.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, f.flow, 1, confirmed.EngineDeploymentId)
		if err != nil {
			return err
		}
		head, err = (wc.Catalog{}).EnableInTx(f.ctx, tx, f.app, f.flow, head.Revision)
		if err != nil {
			return err
		}
		_, err = (wc.Catalog{}).ReserveInTx(f.ctx, tx, wc.ReserveInput{AppID: f.app, FlowID: f.flow, InstanceID: f.instance, RecordID: f.record, ActorID: f.actor, ExpectedRevision: head.Revision, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1})
		return err
	})
	// Existing-task setup goes through genuine Java execution and projection.
	// No claim is made that a public new-instance trigger is part of V037.
	command := fc.Command{ProtocolVersion: 2, CommandID: f.id(t), AppID: f.app, TableID: f.table, ViewID: f.view, RecordID: f.record, InstanceID: f.instance, ActorID: f.actor, Action: "start", RecordVersion: 1, SchemaVersion: 1, FlowID: f.flow, DefinitionVersion: 1, VersionID: f.version}
	payload := fc.ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("Root existing-task Java fixture")), Start: &fc.ExecutionStart{AllowWithdraw: true, Approvers: map[string][]string{f.node: {f.actor}}}, Routes: map[string]bool{}}
	raw, e := fc.EncodeExecutionPayload("start", payload)
	if e != nil {
		t.Fatal(e)
	}
	command.PayloadHash = sha256.Sum256(raw)
	f.transaction(t, func(tx pgx.Tx) error {
		if err := tx.QueryRow(f.ctx, "SELECT applications.acquire_record_command_fence($1,$2,$3,$4,$5,$6,$7)", f.app, f.table, f.view, f.record, command.CommandID, 1, 1).Scan(&command.FenceEpoch); err != nil {
			return err
		}
		_, err := (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(f.ctx, tx, command, payload)
		return err
	})
	if worked, e := rootHTTPWorker(f, client).DispatchOne(f.ctx); e != nil || !worked {
		t.Fatalf("real Java task start: %v %v", worked, e)
	}
	if e = f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2 AND closed_command_id IS NULL", f.app, f.instance).Scan(&f.task); e != nil {
		t.Fatal(e)
	}
	r, found, e := client.Lookup(f.ctx, command)
	if e != nil || !found || r == nil || r.Result.State != "active" || len(r.Result.Tasks) != 1 || r.Result.Tasks[0].ID != f.task {
		t.Fatalf("task lacks actual Java receipt: %v %v", found, e)
	}
	return f, client
}
func rootHTTPAcceptedJavaAction(t *testing.T, f *rootTaskHTTPFixture, action string) (string, fc.Command, string) {
	t.Helper()
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	op := f.id(t)
	body := rootHTTPActionBody(op, action, rootHTTPString(t, preview, "basisToken"))
	accepted := rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", body, nil), 202)
	id := rootHTTPString(t, accepted, "commandId")
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT command_json FROM applications.workflow_commands WHERE command_id=$1", id).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var c fc.Command
	if json.Unmarshal(raw, &c) != nil || c.CommandID != id || c.Action != action || c.ActorID != f.actor {
		t.Fatal("HTTP did not accept exact real command")
	}
	return op, c, body
}
func rootHTTPPendingJava(t *testing.T, f *rootTaskHTTPFixture, op string, c fc.Command) {
	t.Helper()
	status := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-workflow-operations/"+op, "", nil), 200)
	if rootHTTPString(t, status, "status") != "pending" || len(status) != 4 {
		t.Fatal("pending HTTP leaked engine-advanced outcome")
	}
	var state string
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_instances WHERE app_id=$1 AND id=$2", f.app, f.instance).Scan(&state); e != nil || state != "active" {
		t.Fatal("application projection advanced before confirmation", e)
	}
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1 AND command_id=$2", f.app, c.CommandID).Scan(&n); e != nil || n != 1 {
		t.Fatal("pending actual command released fence", e)
	}
}
func rootHTTPFinalJava(t *testing.T, f *rootTaskHTTPFixture, op string, c fc.Command, want string) {
	t.Helper()
	status := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-workflow-operations/"+op, "", nil), 200)
	if rootHTTPString(t, status, "status") != "success" || rootHTTPString(t, status, "instanceState") != want || rootHTTPString(t, status, "commandId") != c.CommandID {
		t.Fatal("confirmed HTTP outcome differs from real command")
	}
	var count int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1", f.app).Scan(&count); e != nil || count != 0 {
		t.Fatal("confirmed command still fenced", e)
	}
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events e JOIN applications.workflow_evidence_documents d ON d.app_id=e.app_id AND d.evidence_hash=e.evidence_hash WHERE e.command_id=$1 AND e.record_version=1 AND e.schema_version=1", c.CommandID).Scan(&count); e != nil || count != 1 {
		t.Fatal("actual action evidence missing", e)
	}
	rootHTTPData(t, f.call(t, "GET", f.root(), "", nil), 200)
}
func rootHTTPRetryDue(t *testing.T, f *rootTaskHTTPFixture, c fc.Command) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_dispatch SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE command_id=$1", c.CommandID); e != nil {
		t.Fatal(e)
	}
}
func TestRootWorkflowActionJavaHTTPSAgreeRejectInterop(t *testing.T) {
	for _, action := range []string{"agree", "reject"} {
		t.Run(action, func(t *testing.T) {
			f, client := rootHTTPJavaSetup(t)
			op, c, body := rootHTTPAcceptedJavaAction(t, f, action)
			rootHTTPPendingJava(t, f, op, c)
			worker := rootHTTPWorker(f, client)
			if worked, e := worker.DispatchOne(f.ctx); e != nil || !worked {
				t.Fatal("real worker failed", worked, e)
			}
			want := "completed"
			if action == "reject" {
				want = "rejected"
			}
			rootHTTPFinalJava(t, f, op, c, want)
			again := rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", body, nil), 200)
			if rootHTTPString(t, again, "commandId") != c.CommandID || rootHTTPString(t, again, "status") != "success" {
				t.Fatal("HTTP retry duplicated final real engine action")
			}
			receipt, found, e := client.Lookup(f.ctx, c)
			if e != nil || !found || receipt == nil || receipt.Result.State != want {
				t.Fatal("final status not backed by actual Java receipt", e)
			}
		})
	}
}
func TestRootWorkflowActionJavaLostReplyKeepsPendingUntilRecoveryInterop(t *testing.T) {
	f, client := rootHTTPJavaSetup(t)
	op, c, _ := rootHTTPAcceptedJavaAction(t, f, "agree")
	dropping := &rootHTTPRealClient{client: client, drop: true}
	worker := rootHTTPWorker(f, dropping)
	if worked, e := worker.DispatchOne(f.ctx); e == nil || !worked || dropping.executions != 1 {
		t.Fatal("did not discard actual successful engine reply", worked, e)
	}
	real, found, e := client.Lookup(f.ctx, c)
	if e != nil || !found || real == nil || real.Result.State != "completed" {
		t.Fatal("engine did not really commit before loss", e)
	}
	rootHTTPPendingJava(t, f, op, c)
	rootHTTPRetryDue(t, f, c)
	if worked, e := worker.DispatchOne(f.ctx); e != nil || !worked || dropping.executions != 1 {
		t.Fatal("recovery re-executed instead of finding durable original receipt", worked, e)
	}
	rootHTTPFinalJava(t, f, op, c, "completed")
}
func TestRootWorkflowActionJavaApplicationFaultRecoversOriginalReceiptInterop(t *testing.T) {
	f, client := rootHTTPJavaSetup(t)
	op, c, _ := rootHTTPAcceptedJavaAction(t, f, "agree")
	fn := "root_http_java_" + strings.ReplaceAll(c.CommandID, "-", "")
	sql := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root HTTP Java apply fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER INSERT ON applications.workflow_execution_events FOR EACH ROW WHEN (NEW.command_id='%s'::uuid) EXECUTE FUNCTION applications.%s()", fn, fn, c.CommandID, fn)
	if _, e := f.owner.Exec(f.ctx, sql); e != nil {
		t.Fatal(e)
	}
	cleanup := func() {
		_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON applications.workflow_execution_events; DROP FUNCTION IF EXISTS applications.%s()", fn, fn))
	}
	t.Cleanup(cleanup)
	counter := &rootHTTPRealClient{client: client}
	worker := rootHTTPWorker(f, counter)
	worked, e := worker.DispatchOne(f.ctx)
	var fault *pgconn.PgError
	if !worked || !errors.As(e, &fault) || fault.Code != "P0001" || counter.executions != 1 {
		t.Fatalf("actual application fault not reached: %v %v", worked, e)
	}
	rootHTTPPendingJava(t, f, op, c)
	actual, found, e := client.Lookup(f.ctx, c)
	if e != nil || !found || actual == nil || actual.Result.State != "completed" {
		t.Fatal("real engine receipt lost during application failure", e)
	}
	cleanup()
	rootHTTPRetryDue(t, f, c)
	if worked, e := worker.DispatchOne(f.ctx); e != nil || !worked || counter.executions != 1 {
		t.Fatal("application recovery repeated engine transition", worked, e)
	}
	rootHTTPFinalJava(t, f, op, c, "completed")
}
