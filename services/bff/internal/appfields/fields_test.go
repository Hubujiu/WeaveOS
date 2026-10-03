package appfields

import (
	"encoding/json"
	"testing"
)

func TestDecimalIndependentOracle(t *testing.T) {
	cases := []struct {
		input, mode string
		places      int
		want        string
	}{
		{"-1.25", "HALF_UP", 1, "-1.30"}, {"-1.25", "HALF_EVEN", 1, "-1.20"}, {"1.35", "HALF_EVEN", 1, "1.40"},
		{"-149", "TOWARD_ZERO", -2, "-100.00"}, {"-149", "FLOOR", -2, "-200.00"}, {"-149", "CEILING", -2, "-100.00"},
		{"150", "HALF_UP", -2, "200.00"}, {"-0.001", "CEILING", 2, "0.00"}, {"9.999", "HALF_EVEN", 2, "10.00"},
		{"100000000000000001.25", "HALF_EVEN", 1, "100000000000000001.20"},
		{"99999999999999999999.999999999999999999", "TOWARD_ZERO", 18, "99999999999999999999.999999999999999999"},
		{"1500000000000000000", "HALF_UP", -18, "2000000000000000000.00"},
	}
	for _, c := range cases {
		t.Run(c.input+c.mode, func(t *testing.T) {
			scale := 2
			if c.places == 18 {
				scale = 18
			}
			got, e := RoundDecimal(c.input, DecimalConfig{38, scale, c.places, c.mode})
			if e != nil || got != c.want {
				t.Fatalf("exact oracle want %s got %s error %v", c.want, got, e)
			}
		})
	}
	for _, s := range []string{"NaN", "Infinity", "1e3", "01", "+2", " 2", "99.995"} {
		if _, e := RoundDecimal(s, DecimalConfig{4, 2, 2, "HALF_UP"}); e == nil {
			t.Errorf("must reject invalid/nonfinite/post-round overflow %s", s)
		}
	}
}
func TestFieldDefaultsAndWireTypes(t *testing.T) {
	for _, kind := range []string{"number", "money"} {
		f := Field{ID: "10000000-0000-4000-8000-000000000001", Name: "金额", Kind: kind, Default: json.RawMessage("null"), Config: json.RawMessage("{}")}
		fs, e := NormalizeFields([]Field{f})
		if e != nil {
			t.Fatal("normalize omitted defaults", e)
		}
		var c DecimalConfig
		json.Unmarshal(fs[0].Config, &c)
		scale := 0
		if kind == "money" {
			scale = 2
		}
		if c != (DecimalConfig{38, scale, scale, "HALF_UP"}) {
			t.Fatalf("default config %#v", c)
		}
		if _, e = NormalizeValue(fs[0], json.RawMessage("1.23")); e == nil {
			t.Fatal("JSON float must fail")
		}
	}
}
func TestDatesOffsetsAndPrecision(t *testing.T) {
	cases := []struct{ kind, cfg, input, want string }{{"date", "{}", `"2024-02-29"`, `"2024-02-29"`}, {"datetime", `{"precision":"minute"}`, `"2026-10-03T10:23:59.999+02:00"`, `"2026-10-03T08:23:00Z"`}, {"datetime", `{"precision":"millisecond"}`, `"1969-12-31T23:59:59.999999Z"`, `"1969-12-31T23:59:59.999Z"`}}
	for _, c := range cases {
		v, e := NormalizeValue(Field{Kind: c.kind, Config: json.RawMessage(c.cfg)}, json.RawMessage(c.input))
		if e != nil || string(v) != c.want {
			t.Errorf("time oracle %s got %s %v", c.want, v, e)
		}
	}
	for _, s := range []string{`"2023-02-29"`, `"2026-10-03T08:23:00"`} {
		kind := "date"
		if len(s) > 15 {
			kind = "datetime"
		}
		if _, e := NormalizeValue(Field{Kind: kind, Config: json.RawMessage(`{"precision":"second"}`)}, json.RawMessage(s)); e == nil {
			t.Error("invalid date/offset accepted")
		}
	}
}
func TestGridSpanAndFieldIdentity(t *testing.T) {
	f := Field{ID: "10000000-0000-4000-8000-000000000001"}
	nodes := []LayoutNode{{ID: "20000000-0000-4000-8000-000000000001", Kind: "group", Span: 6, Children: []LayoutNode{{ID: "20000000-0000-4000-8000-000000000002", Kind: "field", FieldID: f.ID}}}}
	got, e := NormalizeLayout(nodes, []Field{f})
	if e != nil || got[0].Span != 6 || got[0].Children[0].Span != 12 {
		t.Fatalf("12 column grid %+v %v", got, e)
	}
	nodes[0].Span = 13
	if _, e = NormalizeLayout(nodes, []Field{f}); e == nil {
		t.Fatal("invalid span accepted")
	}
}
