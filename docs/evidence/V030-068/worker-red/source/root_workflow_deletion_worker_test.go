package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appworkflows"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"google.golang.org/protobuf/proto"
	"strings"
	"sync"
	"testing"
	"time"
)

// Only the remote RPC boundary is simulated here. Acceptance, leases, all
// transactions, permissions, physical cleanup and auditing use real PostgreSQL.
// Formal Java/Flowable interop is a separate mandatory acceptance stage.
type deletionPeer struct {
	mu               sync.Mutex
	receipt          *pb.FlowDeletionReceipt
	lookups, deletes []*pb.FlowDeletionRequest
	loseReply        bool
	mutate           func(*pb.FlowDeletionReceipt)
	before           func(context.Context, *pb.FlowDeletionRequest) error
}

func (p *deletionPeer) LookupFlowDeletion(ctx context.Context, q *pb.FlowDeletionRequest) (*pb.FlowDeletionLookupResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lookups = append(p.lookups, proto.Clone(q).(*pb.FlowDeletionRequest))
	if p.before != nil {
		if e := p.before(ctx, q); e != nil {
			return nil, e
		}
	}
	if p.receipt != nil {
		return &pb.FlowDeletionLookupResponse{Result: &pb.FlowDeletionLookupResponse_Confirmed{Confirmed: proto.Clone(p.receipt).(*pb.FlowDeletionReceipt)}}, nil
	}
	return &pb.FlowDeletionLookupResponse{Result: &pb.FlowDeletionLookupResponse_NotObserved{NotObserved: &pb.NotObserved{}}}, nil
}
func (p *deletionPeer) DeleteFlow(ctx context.Context, q *pb.FlowDeletionRequest) (*pb.FlowDeletionReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deletes = append(p.deletes, proto.Clone(q).(*pb.FlowDeletionRequest))
	if p.before != nil {
		if e := p.before(ctx, q); e != nil {
			return nil, e
		}
	}
	if p.receipt == nil {
		p.receipt = &pb.FlowDeletionReceipt{AppId: q.AppId, FlowId: q.FlowId, OperationId: q.OperationId, DeletedVersions: 1, DeletedAtSeconds: time.Date(2026, 10, 1, 1, 2, 3, 0, time.UTC).Unix(), DeletedAtNanos: 456000}
	}
	r := proto.Clone(p.receipt).(*pb.FlowDeletionReceipt)
	if p.mutate != nil {
		p.mutate(r)
	}
	if p.loseReply {
		p.loseReply = false
		return nil, context.DeadlineExceeded
	}
	return r, nil
}
func deletionWorker(p rootPublicationFixture, peer *deletionPeer) *appworkflows.Application {
	return &appworkflows.Application{Pool: p.s.f.runtime, Limits: p.app.Limits, DeletionClient: peer}
}
func dispatchDeletion(t *testing.T, a *appworkflows.Application) {
	t.Helper()
	worked, e := a.DispatchDeletion(context.Background())
	if e != nil || !worked {
		t.Fatalf("durable deletion worker did not handle original intent: worked=%v err=%v", worked, e)
	}
}
func deletionDue(t *testing.T, p rootPublicationFixture) {
	t.Helper()
	if _, e := p.s.f.owner.Exec(context.Background(), "UPDATE applications.workflow_deletions SET next_attempt_at=clock_timestamp(),lease_until=CASE WHEN lease_token IS NULL THEN NULL ELSE clock_timestamp()-interval '1 second' END WHERE flow_id=$1", p.flow); e != nil {
		t.Fatal(e)
	}
}
func deletionState(t *testing.T, p rootPublicationFixture) (string, int) {
	t.Helper()
	var state string
	var defs int
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT status,(SELECT count(*) FROM applications.workflow_definitions WHERE id=$1) FROM applications.workflow_deletions WHERE flow_id=$1", p.flow).Scan(&state, &defs); e != nil {
		t.Fatal(e)
	}
	return state, defs
}
func deletionHistory(t *testing.T, p rootPublicationFixture) string {
	t.Helper()
	var out string
	if e := p.s.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(p) ORDER BY operation_id) FROM applications.workflow_publications p WHERE flow_id=$1),(SELECT jsonb_agg(to_jsonb(r) ORDER BY version_id) FROM applications.workflow_engine_receipts r WHERE flow_id=$1),(SELECT jsonb_agg(to_jsonb(d) ORDER BY version) FROM applications.workflow_deployments d WHERE flow_id=$1))::text`, p.flow).Scan(&out); e != nil {
		t.Fatal(e)
	}
	return out
}

func TestRootDeletionWorkerCompletesAndPreservesPublicationHistory(t *testing.T) {
	p := deletionFixture(t)
	other := deletionFixture(t)
	p.publish(t, uuid(t, p.s.f.owner), 1)
	p.dispatch(t)
	original := deletionHistory(t, p)
	in := deletionInput(t, p)
	acceptDeletion(t, p, in)
	peer := &deletionPeer{}
	// Prove the worker does not hold app/catalog/queue locks during network calls.
	peer.before = func(ctx context.Context, q *pb.FlowDeletionRequest) error {
		tx, e := p.s.f.owner.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		for _, sql := range []string{"SELECT 1 FROM applications.apps WHERE id=$1 FOR UPDATE NOWAIT", "SELECT 1 FROM applications.workflow_definitions WHERE app_id=$1 FOR UPDATE NOWAIT", "SELECT 1 FROM applications.workflow_deletions WHERE app_id=$1 FOR UPDATE NOWAIT"} {
			if _, e = tx.Exec(ctx, sql, p.s.f.app); e != nil {
				return e
			}
		}
		return nil
	}
	a := deletionWorker(p, peer)
	dispatchDeletion(t, a)
	if s, n := deletionState(t, p); s != "deleted" || n != 0 {
		t.Fatalf("deletion did not commit full cleanup: %s definitions=%d", s, n)
	}
	if after := deletionHistory(t, p); after != original {
		t.Fatal("deletion changed original publication/engine/deployment history")
	}
	var versions, otherDefs, audits int
	if e := p.s.f.owner.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM applications.workflow_versions WHERE flow_id=$1),(SELECT count(*) FROM applications.workflow_definitions WHERE id=$2),(SELECT count(*) FROM auth.authentication_events WHERE object_id=$3 AND reason_code='WORKFLOW_DELETION_COMPLETED')`, p.flow, other.flow, p.s.view).Scan(&versions, &otherDefs, &audits); e != nil || versions != 0 || otherDefs != 1 || audits != 1 {
		t.Fatal("cleanup scope or audit is incorrect", versions, otherDefs, audits, e)
	}
	got := acceptDeletion(t, p, in)
	if got.Status != "deleted" || got.DeletedVersions == nil || *got.DeletedVersions != 1 || got.DeletedAt == nil || got.CompletedAt == nil || got.DeletedAt.Unix() != peer.receipt.DeletedAtSeconds || got.DeletedAt.Nanosecond() != 456000 {
		t.Fatalf("original receipt was not preserved: %+v", got)
	}
	if len(peer.lookups) != 1 || len(peer.deletes) != 1 || peer.deletes[0].OperationId != in.OperationID || peer.deletes[0].FlowId != in.FlowID {
		t.Fatal("worker changed or duplicated original RPC identity")
	}
	if worked, e := a.DispatchDeletion(context.Background()); e != nil || worked {
		t.Fatal("completed intent was re-dispatched", worked, e)
	}
}

func TestRootDeletionWorkerRecoversLostReplyWithOriginalLookup(t *testing.T) {
	p := deletionFixture(t)
	in := deletionInput(t, p)
	acceptDeletion(t, p, in)
	peer := &deletionPeer{loseReply: true}
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, n := deletionState(t, p); s != "unknown" || n != 1 {
		t.Fatalf("lost reply was mistaken for completed deletion: %s %d", s, n)
	}
	deletionDue(t, p)
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, n := deletionState(t, p); s != "deleted" || n != 0 {
		t.Fatal("restart did not recover committed engine deletion", s, n)
	}
	if len(peer.deletes) != 1 || len(peer.lookups) != 2 || peer.lookups[1].OperationId != in.OperationID {
		t.Fatal("lost reply caused a new deletion instead of original lookup")
	}
}

func TestRootDeletionWorkerWaitsForAcceptedPublication(t *testing.T) {
	p := deletionFixture(t)
	p.publish(t, uuid(t, p.s.f.owner), 1)
	acceptDeletion(t, p, deletionInput(t, p))
	peer := &deletionPeer{}
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, n := deletionState(t, p); s != "pending" || n != 1 || len(peer.lookups) != 0 || len(peer.deletes) != 0 {
		t.Fatal("worker called engine before publication was drained", s, n)
	}
	p.dispatch(t)
	deletionDue(t, p)
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, _ := deletionState(t, p); s != "deleted" {
		t.Fatal("drained blocked publication did not release deletion", s)
	}
}

func TestRootDeletionWorkerRejectsUnboundReceipt(t *testing.T) {
	for _, kind := range []string{"flow", "operation", "count", "time", "precision"} {
		t.Run(kind, func(t *testing.T) {
			p := deletionFixture(t)
			acceptDeletion(t, p, deletionInput(t, p))
			peer := &deletionPeer{mutate: func(r *pb.FlowDeletionReceipt) {
				switch kind {
				case "flow":
					r.FlowId = "00000000-0000-0000-0000-000000000001"
				case "operation":
					r.OperationId = "00000000-0000-0000-0000-000000000001"
				case "count":
					r.DeletedVersions = 9007199254740992
				case "time":
					r.DeletedAtSeconds = 0
				case "precision":
					r.DeletedAtNanos = 1
				}
			}}
			dispatchDeletion(t, deletionWorker(p, peer))
			if s, n := deletionState(t, p); s != "unknown" || n != 1 {
				t.Fatal("unbound receipt drove irreversible application cleanup", kind, s, n)
			}
		})
	}
}

func TestRootDeletionWorkerWaitsForPendingStartCommand(t *testing.T) {
	p := deletionFixture(t)
	ctx := context.Background()
	command := uuid(t, p.s.f.owner)
	payload, _ := json.Marshal(map[string]any{"CommandID": command, "AppID": p.s.f.app, "FlowID": p.flow, "Action": "start"})
	if _, e := p.s.f.owner.Exec(ctx, `INSERT INTO applications.workflow_commands(command_id,command_json,command_hash,state) VALUES($1,$2,decode(repeat('01',32),'hex'),'pending')`, command, payload); e != nil {
		t.Fatal(e)
	}
	acceptDeletion(t, p, deletionInput(t, p))
	peer := &deletionPeer{}
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, n := deletionState(t, p); s != "pending" || n != 1 || len(peer.lookups) != 0 {
		t.Fatal("accepted start intent was not drained", s, n)
	}
}

func TestRootDeletionWorkerStaleLeaseCannotCommit(t *testing.T) {
	p := deletionFixture(t)
	acceptDeletion(t, p, deletionInput(t, p))
	peer := &deletionPeer{}
	newToken := uuid(t, p.s.f.owner)
	changed := false
	peer.before = func(ctx context.Context, q *pb.FlowDeletionRequest) error {
		if changed {
			return nil
		}
		changed = true
		_, e := p.s.f.owner.Exec(ctx, "UPDATE applications.workflow_deletions SET lease_token=$2,lease_until=clock_timestamp()+interval '45 seconds' WHERE flow_id=$1", p.flow, newToken)
		return e
	}
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, n := deletionState(t, p); s == "deleted" || n != 1 {
		t.Fatal("old lease committed cleanup after replacement", s, n)
	}
	var actual string
	var confirmed bool
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT lease_token::text,engine_deleted_at IS NOT NULL FROM applications.workflow_deletions WHERE flow_id=$1", p.flow).Scan(&actual, &confirmed); e != nil || actual != newToken || confirmed {
		t.Fatal("old worker overwrote fresh lease or receipt", actual, confirmed, e)
	}
	peer.before = nil
	deletionDue(t, p)
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, _ := deletionState(t, p); s != "deleted" {
		t.Fatal("replacement lease could not recover", s)
	}
	if len(peer.deletes) != 1 {
		t.Fatal("lease recovery repeated original deletion", len(peer.deletes))
	}
}

func TestRootDeletionWorkerCleanupFailureRollsBackAndRecovers(t *testing.T) {
	p := deletionFixture(t)
	acceptDeletion(t, p, deletionInput(t, p))
	ctx := context.Background()
	peer := &deletionPeer{}
	name := "root_deletion_fault_" + strings.ReplaceAll(p.flow, "-", "")
	sql := `CREATE FUNCTION appdata.` + name + `() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.flow_id='` + p.flow + `'::uuid THEN RAISE EXCEPTION 'isolated deletion cleanup fault'; END IF; RETURN OLD; END $$; CREATE TRIGGER ` + name + ` BEFORE DELETE ON applications.workflow_versions FOR EACH ROW EXECUTE FUNCTION appdata.` + name + `() `
	if _, e := p.s.f.owner.Exec(ctx, sql); e != nil {
		t.Fatal(e)
	}
	drop := `DROP TRIGGER IF EXISTS ` + name + ` ON applications.workflow_versions; DROP FUNCTION IF EXISTS appdata.` + name + `() `
	t.Cleanup(func() {
		if _, e := p.s.f.owner.Exec(context.Background(), drop); e != nil {
			t.Error(e)
		}
	})
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, n := deletionState(t, p); s != "unknown" || n != 1 {
		t.Fatal("cleanup failure partially committed deletion", s, n)
	}
	var receipt bool
	var versions, audits int
	if e := p.s.f.owner.QueryRow(ctx, `SELECT engine_deleted_at IS NOT NULL,(SELECT count(*) FROM applications.workflow_versions WHERE flow_id=$1),(SELECT count(*) FROM auth.authentication_events WHERE object_id=$2 AND reason_code='WORKFLOW_DELETION_COMPLETED') FROM applications.workflow_deletions WHERE flow_id=$1`, p.flow, p.s.view).Scan(&receipt, &versions, &audits); e != nil || !receipt || versions != 1 || audits != 0 {
		t.Fatal("failure lost confirmed receipt or partial audit/config cleanup", receipt, versions, audits, e)
	}
	if _, e := p.s.f.owner.Exec(ctx, drop); e != nil {
		t.Fatal(e)
	}
	deletionDue(t, p)
	dispatchDeletion(t, deletionWorker(p, peer))
	if s, n := deletionState(t, p); s != "deleted" || n != 0 {
		t.Fatal("confirmed engine deletion did not recover application cleanup", s, n)
	}
	if len(peer.deletes) != 1 {
		t.Fatal("application failure repeated engine delete", len(peer.deletes))
	}
}
