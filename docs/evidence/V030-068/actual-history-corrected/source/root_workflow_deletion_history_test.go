package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appworkflows"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"github.com/jackc/pgx/v5"
	"reflect"
	"testing"
	"time"
)

type deletionHistoryPeer struct{ calls int }

func (p *deletionHistoryPeer) LookupFlowDeletion(context.Context, *pb.FlowDeletionRequest) (*pb.FlowDeletionLookupResponse, error) {
	p.calls++
	return &pb.FlowDeletionLookupResponse{Result: &pb.FlowDeletionLookupResponse_NotObserved{NotObserved: &pb.NotObserved{}}}, nil
}
func (p *deletionHistoryPeer) DeleteFlow(_ context.Context, q *pb.FlowDeletionRequest) (*pb.FlowDeletionReceipt, error) {
	p.calls++
	return &pb.FlowDeletionReceipt{AppId: q.AppId, FlowId: q.FlowId, OperationId: q.OperationId, DeletedVersions: 1, DeletedAtSeconds: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Unix()}, nil
}
func acceptHistoryDeletion(t *testing.T, f recordFixture, flow string) wc.DeletionInput {
	t.Helper()
	in := wc.DeletionInput{AppID: f.app, ViewID: f.view, FlowID: flow, ActorID: f.actor, OperationID: recordOperationID(t, f)}
	if e := f.owner.QueryRow(f.ctx, "SELECT revision FROM applications.workflow_definitions WHERE id=$1", flow).Scan(&in.ExpectedRevision); e != nil {
		t.Fatal(e)
	}
	rootCatalogTx(t, f, func(tx pgx.Tx) error { _, e := (wc.Catalog{}).RequestDeletionInTx(f.ctx, tx, in); return e })
	t.Cleanup(func() {
		if _, e := f.owner.Exec(context.Background(), "DELETE FROM applications.workflow_deletions WHERE app_id=$1", f.app); e != nil {
			t.Error(e)
		}
	})
	return in
}
func historyDeletionDispatch(t *testing.T, f recordFixture, peer *deletionHistoryPeer) {
	t.Helper()
	a := &appworkflows.Application{Pool: f.runtime, Limits: f.service.Limits, DeletionClient: peer}
	worked, e := a.DispatchDeletion(f.ctx)
	if e != nil || !worked {
		t.Fatal("deletion worker did not claim this fixture", worked, e)
	}
}
func historyDeletionDue(t *testing.T, f recordFixture, flow string) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_deletions SET next_attempt_at=clock_timestamp() WHERE flow_id=$1", flow); e != nil {
		t.Fatal(e)
	}
}
func historyDeletionBytes(t *testing.T, f recordFixture) string {
	t.Helper()
	var raw string
	if e := f.owner.QueryRow(f.ctx, `SELECT jsonb_build_array(
 (SELECT jsonb_agg(to_jsonb(e) ORDER BY command_id) FROM applications.workflow_execution_events e WHERE app_id=$1),
 (SELECT jsonb_agg(to_jsonb(c) ORDER BY command_id) FROM applications.workflow_commands c WHERE command_json->>'AppID'=$1::text),
 (SELECT jsonb_agg(to_jsonb(e) ORDER BY evidence_hash) FROM applications.workflow_evidence_documents e WHERE app_id=$1),
 (SELECT jsonb_agg(to_jsonb(o) ORDER BY operation_id) FROM applications.operations o WHERE app_id=$1))::text`, f.app).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestRootDeletionHistoryDrainsActiveTaskAndKeepsOriginalEvidence(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.reference)
	acceptHistoryDeletion(t, f.recordFixture, f.head.FlowID)
	peer := &deletionHistoryPeer{}
	historyDeletionDispatch(t, f.recordFixture, peer)
	if peer.calls != 0 {
		t.Fatal("active task caused premature engine deletion")
	}
	// Already accepted work remains actionable while configuration is closing.
	op := rootAction(t, f, rootActionRequest(t, f, "reject"))
	c, p := rootActionRead(t, f, op)
	r, body := f.receipt(t, c, "rejected", "")
	f.apply(t, c, p, r, body, true)
	q := WorkflowEventRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, EventID: c.CommandID}
	before := eventRead(t, f, f.principal, q)
	bytes := historyDeletionBytes(t, f.recordFixture)
	historyDeletionDue(t, f.recordFixture, f.head.FlowID)
	historyDeletionDispatch(t, f.recordFixture, peer)
	var state string
	var projections int
	if e := f.owner.QueryRow(f.ctx, `SELECT status,(SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1)+(SELECT count(*) FROM applications.workflow_tasks WHERE app_id=$1)+(SELECT count(*) FROM applications.workflow_versions WHERE app_id=$1)+(SELECT count(*) FROM applications.workflow_definitions WHERE app_id=$1) FROM applications.workflow_deletions WHERE flow_id=$2`, f.app, f.head.FlowID).Scan(&state, &projections); e != nil || state != "deleted" || projections != 0 {
		t.Fatal("drained physical cleanup incomplete", state, projections, e)
	}
	if got := eventRead(t, f, f.principal, q); !reflect.DeepEqual(got, before) {
		t.Fatal("physical deletion changed authorized original event")
	}
	if historyDeletionBytes(t, f.recordFixture) != bytes {
		t.Fatal("physical deletion changed original journals/evidence/commands/operation results")
	}
	if !f.apply(t, c, p, r, body, true).Duplicate {
		t.Fatal("physical deletion lost original confirmed execution replay")
	}
	if peer.calls != 2 {
		t.Fatal("drain recovery duplicated remote deletion", peer.calls)
	}
}
func TestRootDeletionHistoryPreservesManualStartOriginalReceipt(t *testing.T) {
	f, request := rootManualSetup(t)
	original := rootManualStart(t, f, request)
	acceptHistoryDeletion(t, f, request.FlowID)
	peer := &deletionHistoryPeer{}
	historyDeletionDispatch(t, f, peer)
	if peer.calls != 0 {
		t.Fatal("starting intent was not drained")
	}
	var command string
	if e := f.owner.QueryRow(f.ctx, "SELECT command_id::text FROM applications.workflow_commands WHERE command_json->>'InstanceID'=$1 AND command_json->>'Action'='start'", original.InstanceID).Scan(&command); e != nil {
		t.Fatal(e)
	}
	projection := rootProjection{recordFixture: f}
	c, p := rootActionRead(t, rootTaskFixture{rootProjection: projection}, WorkflowOperationResult{CommandID: command})
	r, body := projection.receipt(t, c, "unchanged", "cancelled")
	projection.apply(t, c, p, r, body, true)
	bytes := historyDeletionBytes(t, f)
	historyDeletionDue(t, f, request.FlowID)
	historyDeletionDispatch(t, f, peer)
	var state string
	if e := f.owner.QueryRow(f.ctx, "SELECT status FROM applications.workflow_deletions WHERE flow_id=$1", request.FlowID).Scan(&state); e != nil || state != "deleted" {
		t.Fatal("manual start cleanup did not complete", state, e)
	}
	if got := rootManualStart(t, f, request); got != original {
		t.Fatal("catalog cleanup changed original manual acceptance result")
	}
	if historyDeletionBytes(t, f) != bytes {
		t.Fatal("manual replay rewrote immutable history")
	}
}
