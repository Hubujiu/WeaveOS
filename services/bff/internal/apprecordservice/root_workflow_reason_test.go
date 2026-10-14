package apprecordservice

import (
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
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
