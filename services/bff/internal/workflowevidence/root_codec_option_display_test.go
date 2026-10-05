package workflowevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func rootEvidenceSelectedOptions() Field {
	a, b := rootEvidenceID(201), rootEvidenceID(202)
	f := rootEvidenceText(4, "")
	f.Definition.Kind = "multi_select"
	f.Definition.Config = json.RawMessage(fmt.Sprintf(`{"options":[{"id":%q,"label":"Current label"}]}`, a))
	f.Value = json.RawMessage(fmt.Sprintf(`[%q,%q,%q]`, b, a, b))
	f.OptionDisplays = []Reference{{ID: b, Label: "Removed label", Deleted: true}, {ID: a, Label: "Current label", Deleted: false}}
	return f
}
func TestRootEvidenceSelectedOptionDisplaysPreserveRemovedLabels(t *testing.T) {
	f := rootEvidenceSelectedOptions()
	before := append([]Reference(nil), f.OptionDisplays...)
	raw, e := EncodeField(f)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(f.OptionDisplays, before) {
		t.Fatal("caller display order mutated")
	}
	got, e := DecodeField(raw)
	if e != nil {
		t.Fatal(e)
	}
	want := []Reference{before[1], before[0]}
	if !reflect.DeepEqual(got.OptionDisplays, want) || !bytes.Equal(got.Value, f.Value) {
		t.Fatalf("selected option display/value lost: %+v %s", got.OptionDisplays, got.Value)
	}
	if bytes.Contains(got.Definition.Config, []byte("Removed label")) {
		t.Fatal("tombstone was inserted into active configuration")
	}
	got.OptionDisplays[0].Label = "caller change"
	again, e := DecodeField(raw)
	if e != nil || again.OptionDisplays[0].Label != "Current label" {
		t.Fatal("decoded display aliases input or another result")
	}
	newer := f
	newer.OptionDisplays = append([]Reference(nil), f.OptionDisplays...)
	newer.OptionDisplays[0].Label = "Another old label"
	next, e := EncodeField(newer)
	if e != nil || bytes.Equal(next, raw) {
		t.Fatal("historical display change did not change content identity")
	}
}
func TestRootEvidenceSelectedOptionDisplaysBindExactSelectedIDs(t *testing.T) {
	for _, kind := range []string{"missing", "extra", "duplicate", "invalid-id", "blank-label", "nul-label", "invalid-utf8", "active-marked-deleted", "active-label-mismatch", "removed-marked-active"} {
		t.Run(kind, func(t *testing.T) {
			f := rootEvidenceSelectedOptions()
			switch kind {
			case "missing":
				f.OptionDisplays = f.OptionDisplays[:1]
			case "extra":
				f.OptionDisplays = append(f.OptionDisplays, Reference{ID: rootEvidenceID(203), Label: "not selected", Deleted: true})
			case "duplicate":
				f.OptionDisplays = append(f.OptionDisplays, f.OptionDisplays[0])
			case "invalid-id":
				f.OptionDisplays[0].ID = "bad"
			case "blank-label":
				f.OptionDisplays[0].Label = " "
			case "nul-label":
				f.OptionDisplays[0].Label = "old\x00label"
			case "invalid-utf8":
				f.OptionDisplays[0].Label = string([]byte{0xff})
			case "active-marked-deleted":
				f.OptionDisplays[1].Deleted = true
			case "active-label-mismatch":
				f.OptionDisplays[1].Label = "wrong current label"
			case "removed-marked-active":
				f.OptionDisplays[0].Deleted = false
			}
			if _, e := EncodeField(f); !errors.Is(e, ErrInvalid) {
				t.Fatalf("%s was not rejected: %v", kind, e)
			}
		})
	}
}
func TestRootEvidenceSelectedOptionDisplaysSingleAndLegacyCompatibility(t *testing.T) {
	f := rootEvidenceSelectedOptions()
	f.Definition.Kind = "single_select"
	f.Value = json.RawMessage(fmt.Sprintf(`%q`, rootEvidenceID(202)))
	f.OptionDisplays = f.OptionDisplays[:1]
	raw, e := EncodeField(f)
	if e != nil {
		t.Fatal(e)
	}
	got, e := DecodeField(raw)
	if e != nil || len(got.OptionDisplays) != 1 || !got.OptionDisplays[0].Deleted {
		t.Fatal("single tombstone lost")
	}
	// Already encoded internal v1 fields without display metadata retain their
	// exact format. Real capture must always populate selected displays.
	f.OptionDisplays = nil
	legacy, e := EncodeField(f)
	if e != nil || bytes.Contains(legacy, []byte("optionDisplays")) {
		t.Fatal("legacy omission changed")
	}
	decoded, e := DecodeField(legacy)
	if e != nil {
		t.Fatal(e)
	}
	same, e := EncodeField(decoded)
	if e != nil || !bytes.Equal(same, legacy) {
		t.Fatal("legacy bytes do not roundtrip")
	}
	f.OptionDisplays = []Reference{{ID: rootEvidenceID(202), Label: "Removed label", Deleted: true}}
	f.Value = json.RawMessage("null")
	if _, e := EncodeField(f); !errors.Is(e, ErrInvalid) {
		t.Fatal("null selection accepted attached display")
	}
	text := rootEvidenceText(3, "value")
	text.OptionDisplays = f.OptionDisplays
	if _, e := EncodeField(text); !errors.Is(e, ErrInvalid) {
		t.Fatal("non-selection accepted option metadata")
	}
}
func TestRootEvidenceSelectedOptionDisplayBudgetPrecedesInterpretation(t *testing.T) {
	f := rootEvidenceSelectedOptions()
	f.OptionDisplays[0].Label = strings.Repeat("x", MaxFieldBytes+1)
	if _, e := EncodeField(f); !errors.Is(e, ErrTooLarge) {
		t.Fatalf("unbounded label=%v", e)
	}
	f = rootEvidenceSelectedOptions()
	f.OptionDisplays = make([]Reference, MaxFieldBytes/36+1)
	if _, e := EncodeField(f); !errors.Is(e, ErrTooLarge) {
		t.Fatalf("unbounded display count=%v", e)
	}
}
