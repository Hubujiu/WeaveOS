package apprecordservice

import (
	"reflect"
	"testing"
)

// Expected cases come from FLOW-08/10/14, not existing service output.
func rootRoundFacts() workflowRoundFacts {
	identity := workflowRoundIdentity{"app", "table", "record", "flow", "latest-instance"}
	return workflowRoundFacts{Target: identity, Latest: identity, ActorID: "starter", InitiatorID: "starter", State: "rejected", CanRead: true, CanEdit: true, DefinitionApprovers: []string{"configured-but-never-assigned", "second-approver"}}
}
func TestRootRoundPolicyConfirmedRoleStateMatrix(t *testing.T) {
	cases := []struct {
		state, actor string
		want         workflowRoundCapabilities
	}{
		{"rejected", "starter", workflowRoundCapabilities{Rework: true, Resubmit: true}},
		{"withdrawn", "starter", workflowRoundCapabilities{Rework: true, Resubmit: true}},
		{"completed", "starter", workflowRoundCapabilities{}},
		{"rejected", "configured-but-never-assigned", workflowRoundCapabilities{Review: true}},
		{"withdrawn", "configured-but-never-assigned", workflowRoundCapabilities{Review: true}},
		{"completed", "configured-but-never-assigned", workflowRoundCapabilities{Review: true}},
		{"completed", "second-approver", workflowRoundCapabilities{Review: true}},
		{"completed", "app-owner-without-flow-role", workflowRoundCapabilities{}},
		{"rejected", "app-owner-without-flow-role", workflowRoundCapabilities{}},
	}
	for _, tc := range cases {
		t.Run(tc.state+"/"+tc.actor, func(t *testing.T) {
			f := rootRoundFacts()
			f.State = tc.state
			f.ActorID = tc.actor
			if got := workflowRoundPolicy(f); got != tc.want {
				t.Fatalf("FLOW08/10: got %+v want %+v", got, tc.want)
			}
		})
	}
}
func TestRootRoundPolicyOldOrForeignRoundReadOnly(t *testing.T) {
	changes := []struct {
		name   string
		change func(*workflowRoundFacts)
	}{
		{"old-instance", func(f *workflowRoundFacts) { f.Target.InstanceID = "old" }},
		{"other-flow", func(f *workflowRoundFacts) { f.Latest.FlowID = "other" }},
		{"other-record", func(f *workflowRoundFacts) { f.Latest.RecordID = "other" }},
		{"other-table", func(f *workflowRoundFacts) { f.Latest.TableID = "other" }},
		{"other-app", func(f *workflowRoundFacts) { f.Latest.AppID = "other" }},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			f := rootRoundFacts()
			f.DefinitionApprovers = append(f.DefinitionApprovers, f.ActorID)
			tc.change(&f)
			if got := workflowRoundPolicy(f); got != (workflowRoundCapabilities{}) {
				t.Fatalf("FLOW14 old/foreign round writable: %+v", got)
			}
		})
	}
}
func TestRootRoundPolicyCurrentPermissionsIntersectRoles(t *testing.T) {
	f := rootRoundFacts()
	f.DefinitionApprovers = append(f.DefinitionApprovers, f.ActorID)
	if got := workflowRoundPolicy(f); got != (workflowRoundCapabilities{true, true, true}) {
		t.Fatalf("combined roles lost %+v", got)
	}
	f.CanEdit = false
	if got := workflowRoundPolicy(f); got != (workflowRoundCapabilities{false, true, true}) {
		t.Fatalf("edit revocation changed independent read-only start eligibility %+v", got)
	}
	f.CanRead = false
	if got := workflowRoundPolicy(f); got != (workflowRoundCapabilities{}) {
		t.Fatalf("read revocation did not deny all %+v", got)
	}
}
func TestRootRoundPolicyOnlyBusinessTerminalStates(t *testing.T) {
	for _, state := range []string{"starting", "active", "no_effect", "", "unknown", "COMPLETED"} {
		t.Run(state, func(t *testing.T) {
			f := rootRoundFacts()
			f.State = state
			f.DefinitionApprovers = append(f.DefinitionApprovers, f.ActorID)
			if got := workflowRoundPolicy(f); got != (workflowRoundCapabilities{}) {
				t.Fatalf("non-business-terminal state accepted %+v", got)
			}
		})
	}
}
func TestRootRoundPolicyMissingIdentityFailsClosed(t *testing.T) {
	for _, name := range []string{"AppID", "TableID", "RecordID", "FlowID", "InstanceID", "ActorID", "InitiatorID"} {
		t.Run(name, func(t *testing.T) {
			f := rootRoundFacts()
			if name == "ActorID" || name == "InitiatorID" {
				reflect.ValueOf(&f).Elem().FieldByName(name).SetString("")
			} else {
				reflect.ValueOf(&f.Target).Elem().FieldByName(name).SetString("")
				reflect.ValueOf(&f.Latest).Elem().FieldByName(name).SetString("")
			}
			if got := workflowRoundPolicy(f); got != (workflowRoundCapabilities{}) {
				t.Fatalf("missing %s granted %+v", name, got)
			}
		})
	}
}
func TestRootRoundPolicyDoesNotMutateTrustedFacts(t *testing.T) {
	f := rootRoundFacts()
	before := f
	before.DefinitionApprovers = append([]string(nil), f.DefinitionApprovers...)
	workflowRoundPolicy(f)
	if !reflect.DeepEqual(f, before) {
		t.Fatal("policy mutated facts")
	}
}
