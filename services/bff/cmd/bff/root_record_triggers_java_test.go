//go:build workflowrpc_integration

package main

import (
	"encoding/json"
	ars "github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	we "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowexecution"
	wr "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	"os"
	"testing"
	"time"
)

// V044 PRD: accepted Create trigger must eventually start the published flow.
// No synthetic reserve/command/fence is injected to conceal missing admission.
func rootHTTPTriggerJavaConfigure(t *testing.T, f *rootTaskHTTPFixture, deployment *wr.Client) {
	t.Helper()
	f.flow = f.id(t)
	start, node, end := f.id(t), f.id(t), f.id(t)
	body := map[string]any{"operationId": f.id(t), "name": "Record triggers", "expectedRevision": 0, "expectedSchemaVersion": 1, "allowWithdraw": true,
		"triggers": []any{map[string]any{"event": "record.created", "condition": nil}, map[string]any{"event": "record.updated", "condition": nil}},
		"graph":    map[string]any{"version": 1, "nodes": []any{map[string]any{"id": start, "kind": "start"}, map[string]any{"id": node, "kind": "approval", "approval": map[string]any{"mode": "all", "assigneeIds": []string{f.actor}, "editableFieldIds": []string{}}}, map[string]any{"id": end, "kind": "end"}}, "edges": []any{map[string]any{"from": start, "to": node}, map[string]any{"from": node, "to": end}}}}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rootHTTPData(t, f.call(t, "PUT", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/workflows/"+f.flow+"/definition", string(raw), nil), 201)

	var version string
	var bpmn []byte
	if err := f.owner.QueryRow(f.ctx, "SELECT version_id::text,bpmn_xml FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, f.flow).Scan(&version, &bpmn); err != nil {
		t.Fatal(err)
	}
	receipt, err := deployment.Deploy(f.ctx, &pb.DeployRequest{AppId: f.app, FlowId: f.flow, VersionId: version, Version: 1, BpmnXml: bpmn})
	if err != nil {
		t.Fatal(err)
	}
	f.transaction(t, func(tx pgx.Tx) error {
		h, err := (wc.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, f.flow, 1, receipt.EngineDeploymentId)
		if err != nil {
			return err
		}
		_, err = (wc.Catalog{}).EnableInTx(f.ctx, tx, f.app, f.flow, h.Revision)
		return err
	})
}

func TestRootRecordTriggerJavaPublicCreateReachesApproval(t *testing.T) {
	rootPublicTriggerJavaContract(t, false, false)
}
func TestRootRecordTriggerJavaIndependentFlowsBothReachApproval(t *testing.T) {
	rootPublicTriggerJavaContract(t, true, false)
}
func TestRootRecordTriggerJavaClosingDrainsAlreadyAcceptedIntent(t *testing.T) {
	rootPublicTriggerJavaContract(t, false, true)
}
func rootPublicTriggerJavaContract(t *testing.T, twoFlows, closeAfterCreate bool) {
	t.Helper()
	target := os.Getenv("WEAVEOS_RPC_TEST_TARGET")
	host, _, err := net.SplitHostPort(target)
	if err != nil || (host != "b3-workflow" && host != "127.0.0.1") {
		t.Fatal("dedicated Docker or native loopback fixture required")
	}
	options := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry()}
	if token := os.Getenv("WEAVEOS_NATIVE_FIXTURE_TOKEN"); token != "" {
		identity, e := wr.NewServiceIdentity(token)
		if e != nil {
			t.Fatal(e)
		}
		options = append(options, grpc.WithUnaryInterceptor(identity.UnaryInterceptor()))
	}
	connection, err := grpc.NewClient(target, options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	deployment, err := wr.NewClient(connection, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client, err := wr.NewExecutionClient(connection, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	f := rootHTTPResourceSetup(t)
	rootHTTPTriggerJavaConfigure(t, f, deployment)
	flows := []string{f.flow}
	if twoFlows {
		rootHTTPTriggerJavaConfigure(t, f, deployment)
		flows = append(flows, f.flow)
	}
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records"
	body := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":1,"values":{"` + f.field + `":"public record to actual approval"}}`
	record := rootHTTPString(t, rootHTTPData(t, f.call(t, "POST", path, body, nil), 201), "id")
	for _, flow := range flows {
		f.flow = flow
		rootHTTPTriggerCount(t, f, record, 1)
	}
	if closeAfterCreate {
		f.transaction(t, func(tx pgx.Tx) error {
			var revision int64
			if err := tx.QueryRow(f.ctx, "SELECT revision FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2", f.app, f.flow).Scan(&revision); err != nil {
				return err
			}
			head, err := (wc.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, f.flow, revision)
			if err == nil && head.State != "closing" {
				t.Fatalf("accepted starting intent must keep closing pending, got %s", head.State)
			}
			return err
		})
	}
	service := &ars.Service{Pool: f.runtime, Limits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}}
	worker := &we.Worker{AdmitStart: service.NewStartAdmitter(), Pool: f.runtime, Client: client, RPCTimeout: 20 * time.Second, Limits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}}
	// Drain the current runtime path until idle, without fabricating a start ledger entry.
	for {
		worked, err := worker.DispatchOne(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	for _, flow := range flows {
		f.flow = flow
		var state string
		var tasks int
		if err := f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND record_id=$3", f.app, f.flow, record).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != "active" {
			t.Fatalf("public trigger persisted but never reached actual Flowable approval: state=%s, want active", state)
		}
		if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_tasks t JOIN applications.workflow_instances i ON i.id=t.instance_id WHERE i.app_id=$1 AND i.flow_id=$2 AND i.record_id=$3", f.app, f.flow, record).Scan(&tasks); err != nil {
			t.Fatal(err)
		}
		if tasks != 1 {
			t.Fatalf("actual approval task count=%d want 1", tasks)
		}
	}
}

// This case uses the actual configured BFF host lifecycle, not an injected
// test worker. The isolated Java process authenticates the internal RPC.
func TestRootRecordTriggerJavaHostRunsStartAdmission(t *testing.T) {
	target := os.Getenv("WEAVEOS_RPC_TEST_TARGET")
	token := os.Getenv("WEAVEOS_NATIVE_FIXTURE_TOKEN")
	if token == "" {
		token = "synthetic_v044_loopback_service_identity_01"
	}
	identity, err := wr.NewServiceIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry(), grpc.WithUnaryInterceptor(identity.UnaryInterceptor()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	deployment, err := wr.NewClient(connection, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	f := rootHTTPResourceSetupRuntime(t, workflowRuntimeConfig{Enabled: true, Identity: identity, Target: target, RPCTimeout: 15 * time.Second})
	rootHTTPTriggerJavaConfigure(t, f, deployment)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records"
	body := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":1,"values":{"` + f.field + `":"host dispatched record"}}`
	record := rootHTTPString(t, rootHTTPData(t, f.call(t, "POST", path, body, nil), 201), "id")
	deadline := time.Now().Add(5 * time.Second)
	state := ""
	for time.Now().Before(deadline) {
		if err := f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND record_id=$3", f.app, f.flow, record).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state == "active" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("actual host never dispatched accepted public start: state=%s want active", state)
}
