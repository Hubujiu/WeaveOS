package apprecordservice

import (
	"reflect"
	"testing"
)

func rootLifecycleFacts() workflowLifecycleFacts {
	return workflowLifecycleFacts{ActorID: "alice", InitiatorID: "alice", InstanceID: "instance-a", State: "active", AllowWithdraw: true, CanRead: true, Tasks: []workflowLifecycleTask{{InstanceID: "instance-a", ActorID: "bob", NodeID: "finance", Closed: true, ClosedAction: "agree", ClosedOutcome: "success"}, {InstanceID: "instance-a", ActorID: "alice", NodeID: "manager"}}}
}
func TestRootWorkflowLifecycleWithdrawQualification(t *testing.T) {
	f := rootLifecycleFacts()
	if !workflowLifecyclePolicy(f).Withdraw {
		t.Fatal("original initiator with configured withdrawal denied")
	}
	for _, change := range []func(*workflowLifecycleFacts){func(f *workflowLifecycleFacts) { f.ActorID = "bob" }, func(f *workflowLifecycleFacts) { f.AllowWithdraw = false }, func(f *workflowLifecycleFacts) { f.CanRead = false }, func(f *workflowLifecycleFacts) { f.State = "approved" }} {
		g := f
		change(&g)
		if workflowLifecyclePolicy(g).Withdraw {
			t.Fatalf("unauthorized withdrawal %+v", g)
		}
	}
	f.Tasks = nil
	if !workflowLifecyclePolicy(f).Withdraw {
		t.Fatal("initiator need not be a task approver")
	}
}
func TestRootWorkflowLifecycleCurrentApproverActualVisitedTargets(t *testing.T) {
	f := rootLifecycleFacts()
	f.Tasks = append(f.Tasks, workflowLifecycleTask{InstanceID: "instance-a", ActorID: "bob", NodeID: "finance", Closed: true}, workflowLifecycleTask{InstanceID: "instance-b", ActorID: "alice", NodeID: "not-visited"})
	got := workflowLifecyclePolicy(f)
	if !reflect.DeepEqual(got.ReturnTargets, []string{"finance", "manager"}) {
		t.Fatalf("actual visited sorted unique targets: %+v", got)
	}
}
func TestRootWorkflowLifecycleHistoricalApproverOnlyOwnSuccessfulNode(t *testing.T) {
	f := rootLifecycleFacts()
	f.ActorID = "bob"
	got := workflowLifecyclePolicy(f)
	if !reflect.DeepEqual(got.ReturnTargets, []string{"finance"}) {
		t.Fatalf("historical success must allow only own node: %+v", got)
	}
	for _, changes := range []workflowLifecycleTask{{InstanceID: "instance-a", ActorID: "bob", NodeID: "finance", Closed: true, ClosedAction: "reject", ClosedOutcome: "success"}, {InstanceID: "instance-a", ActorID: "bob", NodeID: "finance", Closed: true, ClosedAction: "agree", ClosedOutcome: "no_effect"}, {InstanceID: "instance-a", ActorID: "bob", NodeID: "finance", Closed: true}} {
		f.Tasks = []workflowLifecycleTask{changes}
		if len(workflowLifecyclePolicy(f).ReturnTargets) != 0 {
			t.Fatalf("nonapproval creates historical right %+v", changes)
		}
	}
}
func TestRootWorkflowLifecycleRevokedOrTerminalNeverReturns(t *testing.T) {
	f := rootLifecycleFacts()
	if len(workflowLifecyclePolicy(f).ReturnTargets) == 0 {
		t.Fatal("active baseline absent")
	}
	for _, state := range []string{"starting", "approved", "rejected", "withdrawn", ""} {
		g := f
		g.State = state
		r := workflowLifecyclePolicy(g)
		if r.Withdraw || len(r.ReturnTargets) != 0 {
			t.Fatalf("terminal or unstarted allowed %+v", g)
		}
	}
	f.CanRead = false
	r := workflowLifecyclePolicy(f)
	if r.Withdraw || len(r.ReturnTargets) != 0 {
		t.Fatal("revoked access retained capabilities")
	}
}
func TestRootWorkflowLifecycleForeignActorAndInstanceCannotGrant(t *testing.T) {
	f := rootLifecycleFacts()
	if len(workflowLifecyclePolicy(f).ReturnTargets) == 0 {
		t.Fatal("active baseline absent")
	}
	f.ActorID = "outsider"
	if len(workflowLifecyclePolicy(f).ReturnTargets) != 0 {
		t.Fatal("outsider gets return")
	}
	f.Tasks = append(f.Tasks, workflowLifecycleTask{InstanceID: "instance-b", ActorID: "outsider", NodeID: "finance"})
	if len(workflowLifecyclePolicy(f).ReturnTargets) != 0 {
		t.Fatal("foreign instance grants return")
	}
}
func TestRootWorkflowLifecycleDoesNotMutateFacts(t *testing.T) {
	f := rootLifecycleFacts()
	copyTasks := append([]workflowLifecycleTask(nil), f.Tasks...)
	r := workflowLifecyclePolicy(f)
	if !reflect.DeepEqual(r.ReturnTargets, []string{"finance", "manager"}) {
		t.Fatal("missing baseline")
	}
	r.ReturnTargets[0] = "changed"
	if !reflect.DeepEqual(copyTasks, f.Tasks) {
		t.Fatal("input mutated")
	}
	if !reflect.DeepEqual(workflowLifecyclePolicy(f).ReturnTargets, []string{"finance", "manager"}) {
		t.Fatal("output aliases input or retained shared state")
	}
}
