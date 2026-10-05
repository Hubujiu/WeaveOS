package appquery

import (
	"errors"
	"testing"
)

func TestRootStoredValueSQLPreservesPhysicalValues(t *testing.T) {
	const id = "12345678-1234-4234-8234-123456789abc"
	const col = `r."f_12345678123442348234123456789abc"`
	for _, kind := range []FieldKind{"text", "multiline", "boolean", "multi_select", "number", "money", "date", "datetime", "single_select", "member", "department"} {
		t.Run(string(kind), func(t *testing.T) {
			want := col
			switch kind {
			case "number", "money", "single_select", "member", "department":
				want += "::text"
			case "date":
				want = "to_char(" + col + ",'YYYY-MM-DD')"
			case "datetime":
				want = "to_char(" + col + ` AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')`
			}
			got, e := StoredValueSQL(Field{ID: id, Kind: kind})
			if e != nil || got != want {
				t.Fatalf("physical expression=%q want%q error=%v", got, want, e)
			}
		})
	}
}
func TestRootStoredValueSQLRejectsUntrustedIdentifiersAndKinds(t *testing.T) {
	for _, field := range []Field{{ID: "x; DROP TABLE auth.users", Kind: "text"}, {ID: "", Kind: "money"}, {ID: "12345678-1234-4234-8234-123456789abc", Kind: "text);SELECT 1"}} {
		if got, e := StoredValueSQL(field); !errors.Is(e, ErrInvalid) || got != "" {
			t.Fatalf("unsafe fragment=%q error=%v", got, e)
		}
	}
}
