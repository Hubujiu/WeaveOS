package workflowevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRootEvidencePresentationCannotSilentlyReplaceInvalidUTF8(t *testing.T) {
	for _, text := range []string{string([]byte{0xff}), "bad\x00help"} {
		f := rootEvidenceMoney()
		f.Definition.Presentation.HelpText = &text
		if _, e := EncodeField(f); !errors.Is(e, ErrInvalid) {
			t.Fatalf("invalid metadata text was replaced or persisted: %v", e)
		}
	}
}
func TestRootEvidenceRejectsUnpairedSurrogatesWithoutRejectingRealReplacementRune(t *testing.T) {
	for _, value := range []json.RawMessage{json.RawMessage(`"\ud800"`), json.RawMessage(`"\udc00"`), json.RawMessage(`"\ud800\u0041"`)} {
		f := rootEvidenceText(1, "")
		f.Value = value
		if _, e := EncodeField(f); !errors.Is(e, ErrInvalid) {
			t.Fatalf("unpaired Unicode escape silently changed: %s %v", value, e)
		}
	}
	for _, pair := range []struct{ escaped, literal string }{{`"\ud83d\ude00"`, "😀"}, {`"\ufffd"`, "�"}, {`"\\ud800"`, `\ud800`}} {
		a := rootEvidenceText(1, "")
		a.Value = json.RawMessage(pair.escaped)
		b := rootEvidenceText(1, pair.literal)
		x, e := EncodeField(a)
		if e != nil {
			t.Fatal(e)
		}
		y, e := EncodeField(b)
		if e != nil || !bytes.Equal(x, y) {
			t.Fatalf("valid equivalent Unicode or escaped backslash was changed: %s %v", pair.escaped, e)
		}
	}
}
func TestRootEvidenceDirectScalarBoundsPrecedeExpensiveInterpretation(t *testing.T) {
	big := strings.Repeat("a", MaxFieldBytes+1)
	for name, mutate := range map[string]func(*Field){
		"name":            func(f *Field) { f.Definition.Name = big },
		"help":            func(f *Field) { f.Definition.Presentation.HelpText = &big },
		"zone":            func(f *Field) { f.Definition.Presentation.DisplayTimeZone = &big },
		"reference-label": func(f *Field) { f.Reference = &Reference{ID: rootEvidenceID(200), Label: big} },
	} {
		t.Run(name, func(t *testing.T) {
			f := rootEvidenceMoney()
			mutate(&f)
			if _, e := EncodeField(f); !errors.Is(e, ErrTooLarge) {
				t.Fatalf("scalar interpreted before byte budget: %v", e)
			}
		})
	}
	h := rootEvidenceHeader()
	h.CreatedAt = strings.Repeat("a", MaxManifestBytes+1)
	if _, e := EncodeManifest(Manifest{Header: h}); !errors.Is(e, ErrTooLarge) {
		t.Fatalf("unbounded timestamp parsing: %v", e)
	}
}
