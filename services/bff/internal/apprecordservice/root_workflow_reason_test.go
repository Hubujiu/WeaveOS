package apprecordservice

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/jackc/pgx/v5"
)

func reasonStored(t *testing.T, f rootTaskFixture, command, expected string) {
	t.Helper()
	var exists bool
	if e := f.owner.QueryRow(f.ctx, "SELECT to_regclass('applications.workflow_action_reasons') IS NOT NULL").Scan(&exists); e != nil {
		t.Fatal(e)
	}
	if !exists {
		t.Fatal("required immutable action reason storage is absent")
	}
	var got string
	if e := f.runtime.QueryRow(f.ctx, "SELECT reason FROM applications.workflow_action_reasons WHERE command_id=$1", command).Scan(&got); e != nil || got != expected {
		t.Fatalf("original reason missing or changed: %v", e)
	}
}
func TestRootWorkflowReasonNewActionsRejectMissingOrMalformedWithoutWrites(t *testing.T) {
	for _, action := range []string{"reject", "return", "agree", "withdraw"} {
		texts := []string{"", "\u3000\t", strings.Repeat("🦊", 501), "a\x00b"}
		if action == "agree" || action == "withdraw" {
			texts = []string{"unrequested reason"}
		}
		for i, reason := range texts {
			t.Run(action+"/"+string(rune('a'+i)), func(t *testing.T) {
				f := rootTaskSetup(t, false)
				var r WorkflowOperationResult
				var err error
				if action == "agree" || action == "reject" {
					q := rootActionRequest(t, f, action)
					q.Reason = reason
					r, err = f.service.AcceptWorkflowTask(f.ctx, f.principal, q, applications.Metadata{RequestID: "reason-invalid"})
				} else {
					q := rootLifecycleActionReq(t, f, action)
					q.Reason = reason
					r, err = f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, q, applications.Metadata{RequestID: "reason-invalid"})
				}
				if !errors.Is(err, applications.ErrInvalid) || r.CommandID != "" {
					t.Fatalf("invalid new reason accepted: %v", err)
				}
				rootActionCount(t, f, 0, 0, 0)
			})
		}
	}
}
func TestRootWorkflowReasonOriginalRejectAndReturnPersistAndReplay(t *testing.T) {
	for _, action := range []string{"reject", "return"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			const original = "  请补充票据\n保留原文 🦊  "
			var accepted, replay WorkflowOperationResult
			var err error
			if action == "reject" {
				q := rootActionRequest(t, f, action)
				q.Reason = original
				accepted = rootAction(t, f, q)
				reasonStored(t, f, accepted.CommandID, original)
				replay = rootAction(t, f, q)
				q.Reason = "different"
				_, err = f.service.AcceptWorkflowTask(f.ctx, f.principal, q, applications.Metadata{RequestID: "reason-conflict"})
			} else {
				q := rootLifecycleActionReq(t, f, action)
				q.Reason = original
				accepted = rootLifecycleAccept(t, f, q)
				reasonStored(t, f, accepted.CommandID, original)
				replay = rootLifecycleAccept(t, f, q)
				q.Reason = "different"
				_, err = f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, q, applications.Metadata{RequestID: "reason-conflict"})
			}
			if replay != accepted || !errors.Is(err, applications.ErrOperationConflict) {
				t.Fatal("original reason replay or conflict changed", err)
			}
			reasonStored(t, f, accepted.CommandID, original)
		})
	}
}
func TestRootWorkflowReasonLostCommitRecoversOriginalText(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootActionRequest(t, f, "reject")
	q.Reason = "original lost-commit reason"
	var dropped atomic.Bool
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, &dropped, nil)
	r, e := faulty.AcceptWorkflowTask(f.ctx, f.principal, q, applications.Metadata{RequestID: "reason-lost-commit"})
	if !dropped.Load() || !errors.Is(e, applications.ErrUnconfirmed) || r.CommandID != "" {
		t.Fatal("lost commit classification changed", e)
	}
	r = rootAction(t, f, q)
	reasonStored(t, f, r.CommandID, q.Reason)
	rootActionCount(t, f, 1, 1, 1)
}
func TestRootWorkflowReasonActualRollbackHasNoPartialAudit(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootActionRequest(t, f, "reject")
	q.Reason = "rollback audit"
	abort := &abortRecordBeforeCommit{}
	faulty := *f.service
	faulty.Pool = recordCommitFaultPool(t, nil, abort)
	r, e := faulty.AcceptWorkflowTask(f.ctx, f.principal, q, applications.Metadata{RequestID: "reason-rollback"})
	if !abort.injected.Load() || !errors.Is(e, pgx.ErrTxCommitRollback) || r.CommandID != "" {
		t.Fatal("expected actual rollback", e)
	}
	rootActionCount(t, f, 0, 0, 0)
	var exists bool
	if e = f.owner.QueryRow(f.ctx, "SELECT to_regclass('applications.workflow_action_reasons') IS NOT NULL").Scan(&exists); e != nil || !exists {
		t.Fatal("reason storage missing", e)
	}
	r = rootAction(t, f, q)
	reasonStored(t, f, r.CommandID, q.Reason)
}
func TestRootWorkflowReasonHistoryKeepsOriginalAndRechecksAuthority(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.reference)
	q := rootActionRequest(t, f, "reject")
	q.Reason = "请补充原票据"
	accepted := rootAction(t, f, q)
	c, p := rootActionRead(t, f, accepted)
	r, b := f.receipt(t, c, "rejected", "")
	f.apply(t, c, p, r, b, true)
	request := WorkflowEventRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, EventID: c.CommandID}
	got := eventRead(t, f, f.principal, request)
	raw, _ := json.Marshal(got.Event)
	var event map[string]any
	_ = json.Unmarshal(raw, &event)
	if event["actionReason"] != q.Reason {
		t.Fatal("authorized history omitted original human reason")
	}
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.history')", f.app)
	eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.history'", f.app)
	denied, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, request)
	if e == nil || denied.Event.ID != "" {
		t.Fatal("revoked history leaked reason", e)
	}
}

func TestRootWorkflowReasonListsKeepOriginalAfterConfirmation(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.reference)
	q := rootActionRequest(t, f, "reject")
	q.Reason = "  原文\n保留 🦊  "
	accepted := rootAction(t, f, q)
	c, p := rootActionRead(t, f, accepted)
	r, b := f.receipt(t, c, "rejected", "")
	f.apply(t, c, p, r, b, true)
	t.Run("record history", func(t *testing.T) {
		page := historyRead(t, f, f.principal, historyRequest(f, 20))
		for _, event := range page.Items {
			if event.ID == c.CommandID {
				if event.ActionReason == nil || *event.ActionReason != q.Reason {
					t.Fatal("history list omitted original reason")
				}
				return
			}
		}
		t.Fatal("confirmed event missing")
	})
	t.Run("personal approved", func(t *testing.T) {
		page := approvedRead(t, f, WorkflowApprovalsRequest{Page: 1})
		for _, item := range page.Items {
			if item.Event.ID == c.CommandID {
				if item.Event.ActionReason == nil || *item.Event.ActionReason != q.Reason {
					t.Fatal("personal approved omitted original reason")
				}
				return
			}
		}
		t.Fatal("own confirmed event missing")
	})
}

func TestRootWorkflowReasonRuntimeRoleIsAppendOnly(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := rootActionRequest(t, f, "reject")
	q.Reason = "immutable audit"
	a := rootAction(t, f, q)
	for _, statement := range []string{
		"UPDATE applications.workflow_action_reasons SET reason='changed' WHERE command_id=$1",
		"DELETE FROM applications.workflow_action_reasons WHERE command_id=$1",
	} {
		_, e := f.runtime.Exec(f.ctx, statement, a.CommandID)
		var pg interface{ SQLState() string }
		if !errors.As(e, &pg) || pg.SQLState() != "42501" {
			t.Fatalf("runtime changed immutable audit: %v", e)
		}
	}
	reasonStored(t, f, a.CommandID, q.Reason)
}

// Legacy request field order comes from the pre-P25 f9b9ef24 request structs.
// Seed the original ledger/operation without an audit row, rather than deleting
// a newly stored human reason to pretend it is legacy data.
func TestRootWorkflowReasonLegacyConfirmedOriginalRequestsRemainReplayable(t *testing.T) {
	for _, action := range []string{"reject", "return"} {
		t.Run(action, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			op := recordOperationID(t, f.recordFixture)
			taskRequest := WorkflowTaskActionRequest{WorkflowTaskRequest: f.request(), OperationID: op, Action: action, BasisToken: "expired-legacy-basis"}
			lifecycleRequest := WorkflowLifecycleActionRequest{WorkflowLifecycleRequest: rootLifecycleRequest(f), OperationID: op, Action: action, BasisToken: "expired-legacy-basis", TargetNodeID: f.task.NodeID}
			var legacy, current []byte
			var err error
			if action == "reject" {
				legacy, err = json.Marshal(struct {
					WorkflowTaskRequest
					OperationID, Action, BasisToken string
				}{f.request(), op, action, "expired-legacy-basis"})
				current, _ = json.Marshal(taskRequest)
			} else {
				legacy, err = json.Marshal(struct {
					WorkflowLifecycleRequest
					OperationID, Action, BasisToken, TargetNodeID string
				}{rootLifecycleRequest(f), op, action, "expired-legacy-basis", f.task.NodeID})
				current, _ = json.Marshal(lifecycleRequest)
			}
			if err != nil || !bytes.Equal(current, legacy) {
				t.Fatal("legacy request encoding changed", err)
			}
			command := fc.Command{ProtocolVersion: 2, CommandID: recordOperationID(t, f.recordFixture), AppID: f.app, TableID: f.table, ViewID: f.view, RecordID: f.ownRecord, InstanceID: f.instance.ID, TaskID: f.task.ID, ActorID: f.actor, Action: action, RecordVersion: 1, SchemaVersion: 1, TaskEpoch: 1, ExpectedSequence: 1, FlowID: f.head.FlowID, VersionID: f.version, DefinitionVersion: 1}
			if action == "return" {
				command.TargetNodeID = f.task.NodeID
			}
			payload := fc.ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("legacy-evidence:" + command.CommandID)), Routes: map[string]bool{}}
			encoded, err := fc.EncodeExecutionPayload(action, payload)
			if err != nil {
				t.Fatal(err)
			}
			command.PayloadHash = sha256.Sum256(encoded)
			tx, err := f.runtime.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			if err = tx.QueryRow(f.ctx, rootAcquireSQL, f.app, f.table, f.view, f.ownRecord, command.CommandID, int64(1), int64(1)).Scan(&command.FenceEpoch); err != nil {
				t.Fatal(err)
			}
			if _, err = (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(f.ctx, tx, command, payload); err != nil {
				t.Fatal(err)
			}
			pending := WorkflowOperationResult{OperationID: op, CommandID: command.CommandID, InstanceID: f.instance.ID, Status: "pending"}
			raw, err := json.Marshal(pending)
			if err != nil {
				t.Fatal(err)
			}
			fingerprint := sha256.Sum256(legacy)
			if _, err = tx.Exec(f.ctx, "INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,$4,$5,$6,202,$7)", f.actor, op, f.app, "workflow.task."+action, fingerprint[:], raw, "/api/v1/application-workflow-operations/"+op); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			state := "rejected"
			var tasks []fc.ExecutionTask
			if action == "return" {
				state = "active"
				tasks = []fc.ExecutionTask{f.rootProjection.task(t, f.task.NodeID, f.actor, 2)}
			}
			receipt, body := f.receipt(t, command, state, "", tasks...)
			f.apply(t, command, payload, receipt, body, true)
			expected, err := f.service.WorkflowOperation(f.ctx, f.principal, op)
			if err != nil || expected.Status != "success" || expected.InstanceState != state {
				t.Fatal("legacy original result unreadable", err)
			}
			var replay WorkflowOperationResult
			if action == "reject" {
				replay, err = f.service.AcceptWorkflowTask(f.ctx, f.principal, taskRequest, applications.Metadata{RequestID: "legacy-replay"})
			} else {
				replay, err = f.service.AcceptWorkflowLifecycle(f.ctx, f.principal, lifecycleRequest, applications.Metadata{RequestID: "legacy-replay"})
			}
			if err != nil || replay != expected {
				t.Fatal("old original request did not replay", err)
			}
			var n int
			if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_action_reasons WHERE command_id=$1", command.CommandID).Scan(&n); err != nil || n != 0 {
				t.Fatal("legacy replay invented human reason", err)
			}
		})
	}
}
