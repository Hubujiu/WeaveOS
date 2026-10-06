package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5/pgconn"
)

func rootActionRequest(t *testing.T, f rootTaskFixture, action string) WorkflowTaskActionRequest {
	t.Helper()
	return WorkflowTaskActionRequest{WorkflowTaskRequest: f.request(), OperationID: recordOperationID(t, f.recordFixture), Action: action, BasisToken: rootTaskPreview(t, f).BasisToken}
}
func rootAction(t *testing.T, f rootTaskFixture, req WorkflowTaskActionRequest) WorkflowOperationResult {
	t.Helper()
	r, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "root-v037-action"})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func rootActionRead(t *testing.T, f rootTaskFixture, r WorkflowOperationResult) (fc.Command, fc.ExecutionPayload) {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	l := fc.Ledger{Namespace: "applications"}
	entry, e := l.GetInTx(f.ctx, tx, r.CommandID)
	if e != nil {
		t.Fatal(e)
	}
	payload, e := l.ExecutionPayloadInTx(f.ctx, tx, entry.Command)
	if e != nil {
		t.Fatal(e)
	}
	return entry.Command, payload
}
func rootActionCount(t *testing.T, f rootTaskFixture, pending, evidence, operations int) {
	t.Helper()
	for _, item := range []struct {
		q string
		n int
	}{
		{"SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1", pending},
		{"SELECT count(*) FROM applications.workflow_evidence_documents WHERE app_id=$1", evidence},
		{"SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_kind IN('workflow.task.agree','workflow.task.reject')", operations},
		{"SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1", 1 + operations},
		{"SELECT count(*) FROM applications.workflow_dispatch d JOIN applications.workflow_commands c USING(command_id) WHERE c.command_json->>'AppID'=$1", pending},
	} {
		var n int
		if e := f.owner.QueryRow(f.ctx, item.q, f.app).Scan(&n); e != nil || n != item.n {
			t.Fatalf("wrong atomic count got%d want%d %s: %v", n, item.n, item.q, e)
		}
	}
}
func TestRootWorkflowActionAcceptsOriginalEvidenceCommandQueueFenceAndMinimumReceipt(t *testing.T) {
	for _, action := range []string{"agree", "reject"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, true)
			req := rootActionRequest(t, f, action)
			basis := rootTaskBasis(t, f, req.BasisToken)
			r := rootAction(t, f, req)
			if r.OperationID != req.OperationID || r.CommandID == "" || r.CommandID == req.OperationID || r.InstanceID != f.instance.ID || r.Status != "pending" || r.InstanceState != "" || r.Sequence != 0 || r.Reason != "" {
				t.Fatalf("acceptance misreported final state: %+v", r)
			}
			c, p := rootActionRead(t, f, r)
			if c.ProtocolVersion != 2 || c.Action != action || c.AppID != f.app || c.TableID != f.table || c.ViewID != f.view || c.RecordID != f.ownRecord || c.ActorID != f.actor || c.InstanceID != f.instance.ID || c.TaskID != f.task.ID || c.TaskEpoch != 1 || c.ExpectedSequence != 1 || c.RecordVersion != 1 || c.SchemaVersion != 1 || c.FenceEpoch < 1 || c.FlowID != f.head.FlowID || c.VersionID != f.version || c.DefinitionVersion != 1 {
				t.Fatal("command has invented identity or versions")
			}
			if p.Start != nil || len(p.Routes) != 0 || hex.EncodeToString(p.EvidenceHash[:]) != basis.Fingerprint {
				t.Fatal("payload not bound to actual preview basis")
			}
			raw, e := fc.EncodeExecutionPayload(action, p)
			if e != nil || sha256.Sum256(raw) != c.PayloadHash {
				t.Fatal("durable payload differs")
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			m, e := (ev.Store{}).ManifestInTx(f.ctx, tx, f.app, p.EvidenceHash)
			if e != nil || m.Header.RecordID != f.ownRecord {
				t.Fatal("actual evidence missing", e)
			}
			old, e := (ev.Store{}).FieldsInTx(f.ctx, tx, f.app, p.EvidenceHash, []string{f.secret})
			if e != nil || len(old) != 1 || string(old[0].Value) != `"private beta"` {
				t.Fatal("complete internal evidence omitted hidden field", e)
			}
			public, e := json.Marshal(r)
			if e != nil || bytes.Contains(public, []byte(f.secret)) || bytes.Contains(public, []byte(basis.Fingerprint)) || bytes.Contains(public, []byte("private beta")) {
				t.Fatal("receipt exposed hidden evidence")
			}
			rootActionCount(t, f, 1, 1, 1)
			f.state(t, "active", 1, 1, 1, 1)
			status, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID)
			if e != nil || status != r {
				t.Fatalf("pending status wrong: %+v %v", status, e)
			}
		})
	}
}
func TestRootWorkflowActionReplayIgnoresExpiredTokenAndRevokedDataButChecksLiveActor(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	r := rootAction(t, f, req)
	prefix, e := f.service.WorkflowBases.Prefix(f.principal.SessionRef)
	if e != nil {
		t.Fatal(e)
	}
	if e = rootTaskRedis(t).Del(f.ctx, prefix+req.BasisToken).Err(); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	again := rootAction(t, f, req)
	if again != r {
		t.Fatal("exact replay after expiry/revocation changed identity")
	}
	rootActionCount(t, f, 1, 1, 1)
	status, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID)
	if e != nil || status != r {
		t.Fatal("own minimum result lost after revoke", e)
	}
	other := session.Principal{UserID: f.other, SessionRef: recordOperationID(t, f.recordFixture), Record: session.Record{AuthVersion: "1"}}
	if _, e = f.service.WorkflowOperation(f.ctx, other, req.OperationID); !errors.Is(e, applications.ErrMissing) {
		t.Fatalf("owner read someone else's operation: %v", e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.actor); e != nil {
		t.Fatal(e)
	}
	if _, e = f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "disabled-replay"}); !errors.Is(e, session.ErrUnauthorized) {
		t.Fatalf("inactive actor replay allowed: %v", e)
	}
	if _, e = f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID); !errors.Is(e, session.ErrUnauthorized) {
		t.Fatalf("inactive actor status allowed: %v", e)
	}
}
func TestRootWorkflowActionSameOperationDifferentRequestCannotMutate(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	r := rootAction(t, f, req)
	for _, kind := range []string{"action", "record", "instance", "task", "token"} {
		t.Run(kind, func(t *testing.T) {
			changed := req
			switch kind {
			case "action":
				changed.Action = "reject"
			case "record":
				changed.RecordID = f.otherRecord
			case "instance":
				changed.InstanceID = recordOperationID(t, f.recordFixture)
			case "task":
				changed.TaskID = recordOperationID(t, f.recordFixture)
			case "token":
				changed.BasisToken = strings.Repeat("A", 43)
			}
			got, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, changed, applications.Metadata{RequestID: "conflicting-replay"})
			if !errors.Is(e, applications.ErrOperationConflict) || got.CommandID != "" {
				t.Fatalf("changed %s reused operation: %v", kind, e)
			}
		})
	}
	status, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID)
	if e != nil || status != r {
		t.Fatal("conflict changed original result", e)
	}
	rootActionCount(t, f, 1, 1, 1)
}
func TestRootWorkflowActionRejectsStaleRelatedEvidenceAndAcceptsUnrelatedSourceChange(t *testing.T) {
	for _, change := range []string{"record", "reference", "visible-mask", "schema", "unrelated"} {
		t.Run(change, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			req := rootActionRequest(t, f, "agree")
			switch change {
			case "record":
				if _, e := f.service.Edit(f.ctx, f.principal, EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f.recordFixture), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "changed"}}, applications.Metadata{RequestID: "before-approval"}); e != nil {
					t.Fatal(e)
				}
			case "reference":
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", f.other, "new-display-"+f.other); e != nil {
					t.Fatal(e)
				}
			case "visible-mask":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app, f.secret); e != nil {
					t.Fatal(e)
				}
			case "schema":
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET schema_version=schema_version+1 WHERE app_id=$1 AND id=$2", f.app, f.table); e != nil {
					t.Fatal(e)
				}
			case "unrelated":
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", f.actor, "unrelated-"+f.actor); e != nil {
					t.Fatal(e)
				}
			}
			got, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "stale-basis"})
			if change == "unrelated" {
				if e != nil || got.Status != "pending" {
					t.Fatal("unrelated source blocked this task", e)
				}
				rootActionCount(t, f, 1, 1, 1)
			} else {
				if !errors.Is(e, ErrWorkflowBasisChanged) || got.CommandID != "" {
					t.Fatalf("stale %s silently approved: %v", change, e)
				}
				rootActionCount(t, f, 0, 0, 0)
			}
		})
	}
}
func TestRootWorkflowActionExpiredCrossSessionAndWrongTaskTokensCreateNothing(t *testing.T) {
	for _, kind := range []string{"expired", "different-session", "different-task", "different-record"} {
		t.Run(kind, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			req := rootActionRequest(t, f, "agree")
			principal := f.principal
			want := ErrWorkflowBasisChanged
			switch kind {
			case "expired":
				prefix, e := f.service.WorkflowBases.Prefix(principal.SessionRef)
				if e != nil {
					t.Fatal(e)
				}
				if e = rootTaskRedis(t).Del(f.ctx, prefix+req.BasisToken).Err(); e != nil {
					t.Fatal(e)
				}
				want = ErrWorkflowBasisExpired
			case "different-session":
				principal.SessionRef = recordOperationID(t, f.recordFixture)
				want = ErrWorkflowBasisExpired
			case "different-task":
				req.TaskID = recordOperationID(t, f.recordFixture)
			case "different-record":
				req.RecordID = f.otherRecord
			}
			got, e := f.service.AcceptWorkflowTask(f.ctx, principal, req, applications.Metadata{RequestID: "bad-token"})
			if !errors.Is(e, want) || got.CommandID != "" {
				t.Fatalf("bad token %s: %v", kind, e)
			}
			rootActionCount(t, f, 0, 0, 0)
		})
	}
}
func TestRootWorkflowActionRevokedDataOrTaskNeverGrantsAuthority(t *testing.T) {
	for _, kind := range []string{"data", "assignee", "epoch", "closed"} {
		t.Run(kind, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			req := rootActionRequest(t, f, "agree")
			want := applications.ErrDenied
			switch kind {
			case "data":
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
					t.Fatal(e)
				}
			case "assignee":
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_tasks SET assignee_id=$2 WHERE id=$1", f.task.ID, f.other); e != nil {
					t.Fatal(e)
				}
			case "epoch":
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_tasks SET activation_epoch=2 WHERE id=$1", f.task.ID); e != nil {
					t.Fatal(e)
				}
				want = ErrWorkflowTaskChanged
			case "closed":
				c, p := f.accept(t, "agree", f.task.ID, "", 1, 1)
				r, b := f.receipt(t, c, "completed", "")
				f.apply(t, c, p, r, b, true)
				want = ErrWorkflowTaskChanged
			}
			got, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "revoked-task"})
			if !errors.Is(e, want) || got.CommandID != "" {
				t.Fatalf("revoked %s allowed action: %v", kind, e)
			}
			var n int
			if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, req.OperationID).Scan(&n); e != nil || n != 0 {
				t.Fatal("rejected request claimed operation", e)
			}
		})
	}
}
func TestRootWorkflowActionConcurrentExactRequestsCreateOneCommand(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	var wg sync.WaitGroup
	results := make(chan WorkflowOperationResult, 12)
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "concurrent-exact"})
			results <- r
			failures <- e
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	var first WorkflowOperationResult
	for r := range results {
		if first.CommandID == "" {
			first = r
		}
		if r != first || r.Status != "pending" {
			t.Fatal("concurrent exact request duplicated identity")
		}
	}
	rootActionCount(t, f, 1, 1, 1)
}
func TestRootWorkflowActionDistinctConcurrentKeysCannotBypassRecordFence(t *testing.T) {
	f := rootTaskSetup(t, false)
	a := rootActionRequest(t, f, "agree")
	b := a
	b.OperationID = recordOperationID(t, f.recordFixture)
	b.Action = "reject"
	start := make(chan struct{})
	errorsC := make(chan error, 2)
	for _, req := range []WorkflowTaskActionRequest{a, b} {
		go func(req WorkflowTaskActionRequest) {
			<-start
			_, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "concurrent-distinct"})
			errorsC <- e
		}(req)
	}
	close(start)
	passed, blocked := 0, 0
	for i := 0; i < 2; i++ {
		e := <-errorsC
		if e == nil {
			passed++
			continue
		}
		var domain *appstructure.Error
		if errors.As(e, &domain) && domain.Code == "APPLICATION_RECORD_FENCED" {
			blocked++
		} else {
			t.Fatalf("wrong competing-key error: %v", e)
		}
	}
	if passed != 1 || blocked != 1 {
		t.Fatalf("passed%d blocked%d", passed, blocked)
	}
	rootActionCount(t, f, 1, 1, 1)
}
func TestRootWorkflowActionEveryDatabaseFailureRollsBackAllNewState(t *testing.T) {
	for _, table := range []string{"workflow_evidence_documents", "workflow_commands", "workflow_dispatch", "operations"} {
		t.Run(table, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			req := rootActionRequest(t, f, "agree")
			fn := "root_action_" + strings.ReplaceAll(f.app, "-", "")
			predicate := "NEW.app_id='" + f.app + "'::uuid"
			if table == "workflow_commands" {
				predicate = "NEW.command_json->>'AppID'='" + f.app + "'"
			}
			if table == "workflow_dispatch" {
				predicate = "true"
			}
			q := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root action write fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER INSERT ON applications.%s FOR EACH ROW WHEN (%s) EXECUTE FUNCTION applications.%s()", fn, fn, table, predicate, fn)
			if _, e := f.owner.Exec(f.ctx, q); e != nil {
				t.Fatal(e)
			}
			cleanup := func() {
				_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON applications.%s; DROP FUNCTION IF EXISTS applications.%s()", fn, table, fn))
			}
			t.Cleanup(cleanup)
			got, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "actual-write-fault"})
			var fault *pgconn.PgError
			if !errors.As(e, &fault) || fault.Code != "P0001" || !strings.Contains(fault.Message, "root action write fault") || got.CommandID != "" {
				t.Fatalf("did not reach actual %s injected failure: %v", table, e)
			}
			rootActionCount(t, f, 0, 0, 0)
			cleanup()
			r := rootAction(t, f, req)
			if r.Status != "pending" {
				t.Fatal("rolled-back operation could not retry")
			}
			rootActionCount(t, f, 1, 1, 1)
		})
	}
}
func TestRootWorkflowActionConfirmedStatusRequiresOriginalAppliedEvent(t *testing.T) {
	for _, action := range []string{"agree", "reject"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			req := rootActionRequest(t, f, action)
			accepted := rootAction(t, f, req)
			c, p := rootActionRead(t, f, accepted)
			state := "completed"
			if action == "reject" {
				state = "rejected"
			}
			receipt, body := f.receipt(t, c, state, "")
			f.apply(t, c, p, receipt, body, true)
			got, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID)
			if e != nil || got.OperationID != req.OperationID || got.CommandID != c.CommandID || got.Status != "success" || got.InstanceState != state || got.Sequence != 2 || got.Reason != "" {
				t.Fatalf("applied status not reconstructed: %+v %v", got, e)
			}
			again := rootAction(t, f, req)
			if again != got {
				t.Fatal("exact action replay regressed confirmed state")
			}
			rootActionCount(t, f, 0, 1, 1)
			raw := append([]byte(nil), body...)
			raw[len(raw)-1] ^= 1
			if _, e = f.owner.Exec(f.ctx, "UPDATE applications.workflow_execution_events SET result_bytes=$2 WHERE command_id=$1", c.CommandID, raw); e != nil {
				t.Fatal(e)
			}
			if result, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID); e == nil || result.CommandID != "" {
				t.Fatal("corrupted original event reported success")
			}
		})
	}
}
func TestRootWorkflowActionNoEffectIsConfirmedOnlyAfterApplicationProjection(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	accepted := rootAction(t, f, req)
	c, p := rootActionRead(t, f, accepted)
	receipt, body := f.receipt(t, c, "unchanged", "task_inactive")
	f.apply(t, c, p, receipt, body, true)
	got, e := f.service.WorkflowOperation(f.ctx, f.principal, req.OperationID)
	if e != nil || got.Status != "no_effect" || got.Reason != "task_inactive" || got.Sequence != 1 || got.InstanceState != "unchanged" {
		t.Fatalf("no-effect receipt incorrect: %+v %v", got, e)
	}
	rootActionCount(t, f, 0, 1, 1)
	f.state(t, "active", 1, 2, 1, 1)
}
func rootTaskConditionSetup(t *testing.T) (rootTaskFixture, string) {
	t.Helper()
	base := rootCaptureSetup(t)
	f := base.recordFixture
	rootTaskKeepDispatchPrivate(t, f)
	f.ownRecord = f.otherRecord
	g := rootCatalogCondition(t, f)
	condition := ""
	for i := range g.Nodes {
		if g.Nodes[i].Kind == "condition" {
			condition = g.Nodes[i].ID
			g.Nodes[i].Condition = json.RawMessage(`{"operator":"or","children":[{"operator":"and","children":[{"fieldId":"` + f.secret + `","operator":"eq","value":"private beta"},{"fieldId":"` + f.public + `","operator":"neq","value":"blocked"}]},{"fieldId":"` + f.public + `","operator":"eq","value":"never"}]}`)
		}
	}
	h := rootCatalogReady(t, f, g)
	instance := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var version string
	if e := f.owner.QueryRow(f.ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version); e != nil {
		t.Fatal(e)
	}
	projection := rootProjection{recordFixture: f, head: h, instance: instance, version: version, first: g.Nodes[1].ID, second: g.Nodes[1].ID}
	c, p := rootRecoveryAccept(t, projection)
	task := projection.task(t, projection.first, projection.actor, 1)
	r, b := projection.receipt(t, c, "active", "", task)
	projection.apply(t, c, p, r, b, true)
	return rootTaskFixture{rootProjection: projection, task: task}, condition
}
func TestRootWorkflowActionEvaluatesPinnedConditionsOnActualHiddenTypedFields(t *testing.T) {
	f, condition := rootTaskConditionSetup(t)
	req := rootActionRequest(t, f, "agree")
	// Publish a newer opposite condition; the existing instance must keep its old graph.
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT graph_json FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, f.head.FlowID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var graph flowgraph.Graph
	if json.Unmarshal(raw, &graph) != nil {
		t.Fatal("fixture graph")
	}
	for i := range graph.Nodes {
		if bytes.Equal(bytes.TrimSpace(graph.Nodes[i].Condition), []byte("null")) {
			graph.Nodes[i].Condition = nil
		}
		if graph.Nodes[i].ID == condition {
			graph.Nodes[i].Condition = json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + f.public + `","operator":"eq","value":"never"}]}`)
		}
	}
	newer := rootCatalogPut(t, f.recordFixture, f.head.FlowID, f.head.Revision, graph)
	rootCatalogDeploy(t, f.recordFixture, newer)
	r := rootAction(t, f, req)
	c, p := rootActionRead(t, f, r)
	if c.DefinitionVersion != 1 || c.VersionID != f.version || len(p.Routes) != 1 || !p.Routes[condition] {
		t.Fatal("action used candidate/current graph or user-visible subset instead of pinned true condition")
	}
	if p.Start != nil {
		t.Fatal("agree resupplied start roster")
	}
}
func TestRootWorkflowActionFenceProtectsOnlyTargetAndDoesNotImplicitlySave(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	rootAction(t, f, req)
	same := EditRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, OperationID: recordOperationID(t, f.recordFixture), ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{f.public: "not implicitly saved"}}
	_, e := f.service.Edit(f.ctx, f.principal, same, applications.Metadata{RequestID: "pending-edit"})
	var domain *appstructure.Error
	if !errors.As(e, &domain) || domain.Code != "APPLICATION_RECORD_FENCED" {
		t.Fatalf("pending action failed to protect record: %v", e)
	}
	same.RecordID = f.otherRecord
	same.OperationID = recordOperationID(t, f.recordFixture)
	if r, e := f.service.Edit(f.ctx, f.principal, same, applications.Metadata{RequestID: "other-record-edit"}); e != nil || r.RecordVersion != 2 {
		t.Fatal("different record blocked by this fence", e)
	}
	var actual string
	var version int64
	relation := rootCaptureRelation(rootEvidenceStoreFixture{recordFixture: f.recordFixture})
	if e = f.owner.QueryRow(f.ctx, "SELECT "+rootCaptureColumn(f.public)+",record_version FROM "+relation+" WHERE id=$1", f.ownRecord).Scan(&actual, &version); e != nil || actual != "alpha" || version != 1 {
		t.Fatal("approve performed an implicit data save", e)
	}
}
func TestRootWorkflowActionInvalidOrNonWorkflowOperationsNeverReturnStatus(t *testing.T) {
	f := rootTaskSetup(t, false)
	req := rootActionRequest(t, f, "agree")
	for _, action := range []string{"", "start", "withdraw", "return", "Agree"} {
		bad := req
		bad.Action = action
		got, e := f.service.AcceptWorkflowTask(f.ctx, f.principal, bad, applications.Metadata{RequestID: "unsupported-action"})
		if !errors.Is(e, applications.ErrInvalid) || got.CommandID != "" {
			t.Fatalf("unsupported %q accepted: %v", action, e)
		}
	}
	op := recordOperationID(t, f.recordFixture)
	if _, e := f.service.Create(f.ctx, f.principal, CreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, ExpectedSchemaVersion: 1, Values: map[string]any{f.public: "ordinary"}}, applications.Metadata{RequestID: "ordinary-create"}); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{op, recordOperationID(t, f.recordFixture)} {
		if got, e := f.service.WorkflowOperation(f.ctx, f.principal, id); !errors.Is(e, applications.ErrMissing) || got.CommandID != "" {
			t.Fatalf("non-workflow operation read as approval: %+v %v", got, e)
		}
	}
	rootActionCount(t, f, 0, 0, 0)
}
