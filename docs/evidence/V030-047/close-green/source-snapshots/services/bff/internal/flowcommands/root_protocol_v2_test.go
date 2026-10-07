package flowcommands

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

func rootV2Command() Command {
	c := rootCommand()
	c.ProtocolVersion = 2
	c.ViewID = "81000000-0000-4000-8000-000000000008"
	c.FlowID = "82000000-0000-4000-8000-000000000009"
	c.VersionID = "83000000-0000-4000-8000-000000000010"
	c.DefinitionVersion = 3
	c.SchemaVersion = 4
	return c
}
func TestRootV2CanonicalGoldenVectors(t *testing.T) {
	raw, e := os.ReadFile("testdata/root-command-vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Name         string
		Command      Command
		CanonicalHex string
		SHA256       string
	}
	if e = json.Unmarshal(raw, &cases); e != nil {
		t.Fatal(e)
	}
	if len(cases) != 6 {
		t.Fatalf("expected fixed six vectors, got %d", len(cases))
	}
	for _, v := range cases {
		t.Run(v.Name, func(t *testing.T) {
			got, e := CanonicalBytes(v.Command)
			if e != nil {
				t.Fatal(e)
			}
			if hex.EncodeToString(got) != v.CanonicalHex {
				t.Fatalf("canonical bytes differ: %x", got)
			}
			hash, e := Fingerprint(v.Command)
			if e != nil || hex.EncodeToString(hash[:]) != v.SHA256 {
				t.Fatalf("fingerprint %x %v", hash, e)
			}
			if sha256.Sum256(got) != hash {
				t.Fatal("fingerprint is not canonical byte hash")
			}
		})
	}
}
func TestRootV2EveryBindingChangesFingerprint(t *testing.T) {
	base := rootV2Command()
	original, e := Fingerprint(base)
	if e != nil {
		t.Fatal(e)
	}
	edits := []func(*Command){
		func(c *Command) { c.CommandID = otherID }, func(c *Command) { c.AppID = otherID }, func(c *Command) { c.TableID = otherID },
		func(c *Command) { c.ViewID = otherID }, func(c *Command) { c.RecordID = otherID }, func(c *Command) { c.FlowID = otherID },
		func(c *Command) { c.VersionID = otherID }, func(c *Command) { c.InstanceID = otherID }, func(c *Command) { c.TaskID = otherID },
		func(c *Command) { c.ActorID = otherID }, func(c *Command) { c.Action = "reject" }, func(c *Command) { c.DefinitionVersion++ },
		func(c *Command) { c.SchemaVersion++ }, func(c *Command) { c.RecordVersion++ }, func(c *Command) { c.FenceEpoch++ },
		func(c *Command) { c.TaskEpoch++ }, func(c *Command) { c.ExpectedSequence++ }, func(c *Command) { c.PayloadHash[31] = 3 },
	}
	for i, edit := range edits {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c := base
			edit(&c)
			h, e := Fingerprint(c)
			if e != nil || h == original {
				t.Fatalf("binding missing %v", e)
			}
		})
	}
	a := base
	a.Action = "return"
	a.TargetNodeID = otherID
	b := a
	b.TargetNodeID = base.FlowID
	ah, ae := Fingerprint(a)
	bh, be := Fingerprint(b)
	if ae != nil || be != nil || ah == bh {
		t.Fatal("return target not bound")
	}
}
func TestRootV2StrictIdentityAndIntegerBounds(t *testing.T) {
	edits := []func(*Command){
		func(c *Command) { c.ViewID = "" }, func(c *Command) { c.FlowID = "00000000-0000-0000-0000-000000000000" },
		func(c *Command) { c.VersionID = "BBBBBBBB-BBBB-4BBB-8BBB-BBBBBBBBBBBB" }, func(c *Command) { c.ViewID = " " + otherID },
		func(c *Command) { c.DefinitionVersion = 0 }, func(c *Command) { c.DefinitionVersion = 9007199254740992 },
		func(c *Command) { c.SchemaVersion = -1 }, func(c *Command) { c.SchemaVersion = 9007199254740992 },
		func(c *Command) { c.RecordVersion = 0 }, func(c *Command) { c.FenceEpoch = 0 }, func(c *Command) { c.TaskEpoch = 9007199254740992 },
		func(c *Command) { c.ExpectedSequence = -1 }, func(c *Command) { c.ExpectedSequence = 9007199254740992 },
		func(c *Command) { c.PayloadHash = [32]byte{} }, func(c *Command) { c.AppID = "" }, func(c *Command) { c.ActorID = "" },
		func(c *Command) { c.CommandID = "" }, func(c *Command) { c.TableID = "" }, func(c *Command) { c.RecordID = "" }, func(c *Command) { c.InstanceID = "" },
	}
	for i, edit := range edits {
		t.Run(fmt.Sprint(i), func(t *testing.T) { c := rootV2Command(); edit(&c); rootV2Invalid(t, c) })
	}
	c := rootV2Command()
	c.DefinitionVersion = 9007199254740991
	c.SchemaVersion = 9007199254740991
	c.RecordVersion = 9007199254740991
	c.FenceEpoch = 9007199254740991
	c.TaskEpoch = 9007199254740991
	c.ExpectedSequence = 9007199254740991
	if _, e := Fingerprint(c); e != nil {
		t.Fatalf("maximum safe integers should remain exact: %v", e)
	}
}
func rootV2Invalid(t *testing.T, c Command) {
	t.Helper()
	b, e := CanonicalBytes(c)
	if !errors.Is(e, ErrInvalid) || len(b) != 0 {
		t.Fatalf("invalid bytes accepted %x %v", b, e)
	}
	h, e := Fingerprint(c)
	if !errors.Is(e, ErrInvalid) || h != ([32]byte{}) {
		t.Fatalf("invalid hash accepted %x %v", h, e)
	}
}
func TestRootV2ActionSpecificIdentities(t *testing.T) {
	for _, action := range []string{"agree", "reject", "start", "withdraw", "return"} {
		t.Run(action, func(t *testing.T) {
			c := rootV2Command()
			c.Action = action
			if action == "start" || action == "withdraw" {
				c.TaskID = ""
				c.TaskEpoch = 0
			}
			if action == "start" {
				c.ExpectedSequence = 0
			}
			if action == "return" {
				c.TargetNodeID = otherID
			}
			if _, e := Fingerprint(c); e != nil {
				t.Fatal(e)
			}
			bad := c
			if action == "start" || action == "withdraw" {
				bad.TaskID = otherID
			} else {
				bad.TaskID = ""
			}
			rootV2Invalid(t, bad)
			bad = c
			if action == "start" || action == "withdraw" {
				bad.TaskEpoch = 1
			} else {
				bad.TaskEpoch = 0
			}
			rootV2Invalid(t, bad)
			bad = c
			if action == "return" {
				bad.TargetNodeID = ""
			} else {
				bad.TargetNodeID = otherID
			}
			rootV2Invalid(t, bad)
		})
	}
	c := rootV2Command()
	c.Action = "start"
	c.TaskID = ""
	c.TaskEpoch = 0
	c.ExpectedSequence = 1
	rootV2Invalid(t, c)
	for _, action := range []string{"", "complete", "reapprove", "return ", "START"} {
		c := rootV2Command()
		c.Action = action
		rootV2Invalid(t, c)
	}
}
func TestRootV1RejectsV2BindingsWithoutReinterpretingHistory(t *testing.T) {
	c := rootCommand()
	b, e := CanonicalBytes(c)
	if e != nil {
		t.Fatal(e)
	}
	old, e := json.Marshal(c)
	if e != nil || !bytes.Equal(b, old) {
		t.Fatal("legacy canonical JSON changed")
	}
	for _, needle := range [][]byte{[]byte("ViewID"), []byte("FlowID"), []byte("VersionID"), []byte("TargetNodeID"), []byte("SchemaVersion"), []byte("DefinitionVersion")} {
		if bytes.Contains(b, needle) {
			t.Fatalf("new zero field leaked into legacy bytes: %s", needle)
		}
	}
	edits := []func(*Command){func(c *Command) { c.ViewID = otherID }, func(c *Command) { c.FlowID = otherID }, func(c *Command) { c.VersionID = otherID }, func(c *Command) { c.TargetNodeID = otherID }, func(c *Command) { c.DefinitionVersion = 1 }, func(c *Command) { c.SchemaVersion = 1 }}
	for i, edit := range edits {
		t.Run(fmt.Sprint(i), func(t *testing.T) { bad := c; edit(&bad); rootV2Invalid(t, bad) })
	}
}
func TestRootV2ReceiptBindingAndMonotonicReplay(t *testing.T) {
	c := rootV2Command()
	r := rootReceipt(t, c)
	p, e := PlanReceipt(c, r, 11, nil)
	if e != nil || p != (ApplyPlan{Outcome: "success", Sequence: 12}) {
		t.Fatalf("%+v %v", p, e)
	}
	p, e = PlanReceipt(c, r, 50, &r)
	if e != nil || !p.Duplicate || p.Sequence != 12 {
		t.Fatalf("replay %+v %v", p, e)
	}
	for _, change := range []func(*Command){func(c *Command) { c.FlowID = otherID }, func(c *Command) { c.VersionID = otherID }, func(c *Command) { c.SchemaVersion++ }, func(c *Command) { c.ViewID = otherID }} {
		changed := c
		change(&changed)
		if _, e = PlanReceipt(changed, r, 11, nil); !errors.Is(e, ErrConflict) {
			t.Fatalf("cross-context receipt accepted %v", e)
		}
	}
	for _, status := range []string{"unknown", "pending", "timeout", "failed"} {
		bad := r
		bad.Outcome = status
		if _, e = PlanReceipt(c, bad, 11, nil); !errors.Is(e, ErrInvalid) {
			t.Fatalf("uncertain receipt accepted %v", e)
		}
	}
	r.Outcome = "no_effect"
	r.Sequence = 11
	p, e = PlanReceipt(c, r, 11, nil)
	if e != nil || p.Sequence != 11 || p.Outcome != "no_effect" {
		t.Fatal("no-effect sequence changed")
	}
}
func TestRootV2CanonicalBytesHaveIndependentOwnership(t *testing.T) {
	c := rootV2Command()
	a, e := CanonicalBytes(c)
	if e != nil {
		t.Fatal(e)
	}
	want := append([]byte(nil), a...)
	a[0] ^= 0xff
	b, e := CanonicalBytes(c)
	if e != nil || !bytes.Equal(b, want) {
		t.Fatal("caller mutation corrupted later encoding")
	}
	h, e := Fingerprint(c)
	if e != nil || h != sha256.Sum256(want) {
		t.Fatal("caller mutation corrupted identity")
	}
}
func TestRootV2JSONStorageRoundTripPreservesIdentity(t *testing.T) {
	for _, version := range []int{1, 2} {
		c := rootCommand()
		if version == 2 {
			c = rootV2Command()
			c.Action = "return"
			c.TargetNodeID = otherID
		}
		raw, e := json.Marshal(c)
		if e != nil {
			t.Fatal(e)
		}
		var got Command
		if e = json.Unmarshal(raw, &got); e != nil || got != c {
			t.Fatal("storage round trip changed command")
		}
		a, ae := Fingerprint(c)
		b, be := Fingerprint(got)
		if ae != nil || be != nil || a != b {
			t.Fatal("stored identity not reproducible")
		}
	}
}
func TestRootV2UnknownProtocolCannotDowngrade(t *testing.T) {
	for _, v := range []int{-1, 0, 3, 999} {
		c := rootV2Command()
		c.ProtocolVersion = v
		rootV2Invalid(t, c)
	}
}
