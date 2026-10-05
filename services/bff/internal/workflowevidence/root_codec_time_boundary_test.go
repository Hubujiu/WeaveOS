package workflowevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestRootEvidenceUTCConversionCannotProduceUndecodableYears(t *testing.T) {
	for _, stamp := range []string{"0000-01-01T00:00:00+01:00", "9999-12-31T23:59:59-01:00"} {
		t.Run(stamp, func(t *testing.T) {
			f := rootEvidenceText(1, stamp)
			f.Definition.Kind = "datetime"
			f.Definition.Config = json.RawMessage(`{"precision":"second"}`)
			if _, e := EncodeField(f); !errors.Is(e, ErrInvalid) {
				t.Fatalf("UTC conversion encoded a non-RFC3339 year: %v", e)
			}
			h := rootEvidenceHeader()
			h.CreatedAt = stamp
			if _, e := EncodeManifest(Manifest{Header: h}); !errors.Is(e, ErrInvalid) {
				t.Fatalf("header UTC conversion encoded an undecodable year: %v", e)
			}
		})
	}
	for _, stamp := range []string{"0000-01-01T01:00:00+01:00", "9999-12-31T22:59:59-01:00"} {
		f := rootEvidenceText(1, stamp)
		f.Definition.Kind = "datetime"
		f.Definition.Config = json.RawMessage(`{"precision":"second"}`)
		raw, e := EncodeField(f)
		if e != nil {
			t.Fatal(e)
		}
		decoded, e := DecodeField(raw)
		if e != nil {
			t.Fatal(e)
		}
		again, e := EncodeField(decoded)
		if e != nil || !bytes.Equal(raw, again) {
			t.Fatalf("representable boundary did not roundtrip: %v", e)
		}
	}
}
