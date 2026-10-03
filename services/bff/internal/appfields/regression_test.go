package appfields

import (
	"encoding/json"
	"testing"
)

func TestRequiredConfigAndMalformedDecimalKeysCannotNormalize(t *testing.T) {
	for _, raw := range []string{`{"precision":"38","scale":2,"roundingMode":"HALF_UP"}`, `{"roundingMode":null}`, `{"precision":38,"scale":"2","roundingMode":"HALF_UP"}`} {
		for i := 0; i < 100; i++ {
			f := Field{ID: "00000000-0000-4000-8000-000000000011", Name: "n", Kind: "money", Default: json.RawMessage(`null`), Config: json.RawMessage(raw)}
			if _, e := NormalizeFields([]Field{f}); e == nil {
				t.Fatalf("malformed exact configuration accepted %s", raw)
			}
		}
	}
	f := Field{ID: "00000000-0000-4000-8000-000000000011", Name: "t", Kind: "text", Default: json.RawMessage(`null`), Config: json.RawMessage(`{}`)}
	if _, e := NormalizeFields([]Field{f}); e == nil {
		t.Fatal("text maxLength is required and nullable")
	}
}
