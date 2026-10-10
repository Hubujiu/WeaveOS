package apppresets

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
)

const titleID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const amountID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const otherID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

func rootFields() []appquery.Field {
	return []appquery.Field{{ID: titleID, Kind: appquery.Text}, {ID: amountID, Kind: appquery.Number}}
}
func rootPolicy() appaccess.Policy {
	return appaccess.Policy{ActorID: "11111111-1111-4111-8111-111111111111", OwnerID: "11111111-1111-4111-8111-111111111111", AppID: "22222222-2222-4222-8222-222222222222", ViewID: "33333333-3333-4333-8333-333333333333", ResourceExists: true}
}
func rootState() State {
	return State{Name: " 测试方案 ", Filter: json.RawMessage(`null`), Sort: json.RawMessage(`null`), HiddenColumnIDs: []string{}, ColumnOrder: []string{}, ColumnWidths: map[string]int64{}}
}
func rootJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func rootGroup(value string) json.RawMessage {
	return json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + titleID + `","operator":"eq","value":` + value + `}]}`)
}

// ADR14.4 and ADR008 A1/A3: preserve user structure and display state; query
// compiler semantics are independently covered there, not reimplemented here.
func TestRootPresetConfigurationPreservesStructureAndIndependentState(t *testing.T) {
	s := rootState()
	s.Filter = json.RawMessage(`{"operator":"or","children":[{"operator":"and","children":[{"fieldId":"` + titleID + `","operator":"eq","value":"甲"}]},{"operator":"and","children":[{"fieldId":"` + amountID + `","operator":"gte","value":"123.45"}]}]}`)
	s.Sort = json.RawMessage(`{"fieldId":"` + amountID + `","direction":"desc"}`)
	s.HiddenColumnIDs = []string{amountID, "createdBy"}
	s.ColumnOrder = []string{amountID, titleID, "recordVersion"}
	s.ColumnWidths = map[string]int64{amountID: 210, "id": 1, "updatedAt": 9007199254740991}
	raw := rootJSON(t, s)
	decoded, e := DecodeState(raw)
	if e != nil {
		t.Fatal("legal closed configuration rejected", e)
	}
	got, e := Prepare(decoded, rootPolicy(), rootFields())
	if e != nil {
		t.Fatal("legal typed private configuration rejected", e)
	}
	if got.State.Name != "测试方案" || !reflect.DeepEqual(got.State.HiddenColumnIDs, s.HiddenColumnIDs) || !reflect.DeepEqual(got.State.ColumnOrder, s.ColumnOrder) || !reflect.DeepEqual(got.State.ColumnWidths, s.ColumnWidths) {
		t.Fatalf("changed configuration: %+v", got)
	}
	var before, after any
	json.Unmarshal(s.Filter, &before)
	json.Unmarshal(got.State.Filter, &after)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("changed AND/OR meaning: %s", got.State.Filter)
	}
	if !reflect.DeepEqual(got.FieldKinds, map[string]appquery.FieldKind{titleID: appquery.Text, amountID: appquery.Number}) {
		t.Fatalf("wrong references: %v", got.FieldKinds)
	}
	if reason := CheckStored(got.Definition, got.FieldKinds, rootPolicy(), rootFields()); reason != "" {
		t.Fatalf("valid stored configuration invalid: %s", reason)
	}
	// Consumers cannot mutate retained input through a returned slice/map/raw alias.
	got.State.HiddenColumnIDs[0] = "id"
	got.State.ColumnWidths[amountID] = 2
	got.State.Filter[0] = '!'
	if !bytes.Equal(raw, rootJSON(t, s)) || decoded.HiddenColumnIDs[0] != amountID || decoded.ColumnWidths[amountID] != 210 || decoded.Filter[0] != '{' {
		t.Fatal("output aliases caller configuration")
	}
}

func TestRootPresetStrictClosedJSON(t *testing.T) {
	good := string(rootJSON(t, rootState()))
	cases := map[string]string{
		"unknown":          strings.Replace(good, `"name":`, `"ownerId":"x","name":`, 1),
		"case_alias":       strings.Replace(good, `"name":`, `"Name":`, 1),
		"duplicate":        strings.Replace(good, `"name":`, `"name":"first","name":`, 1),
		"nested_duplicate": strings.Replace(good, `"columnWidths":{}`, `"columnWidths":{"id":1,"id":2}`, 1),
		"missing_filter":   strings.Replace(good, `"filter":null,`, "", 1),
		"null_hidden":      strings.Replace(good, `"hiddenColumnIds":[]`, `"hiddenColumnIds":null`, 1),
		"null_order":       strings.Replace(good, `"columnOrder":[]`, `"columnOrder":null`, 1),
		"null_widths":      strings.Replace(good, `"columnWidths":{}`, `"columnWidths":null`, 1),
		"fractional_width": strings.Replace(good, `"columnWidths":{}`, `"columnWidths":{"id":1.5}`, 1),
		"surrogate":        strings.Replace(good, `" 测试方案 "`, `"\ud800"`, 1),
		"nul":              strings.Replace(good, `" 测试方案 "`, `"a\u0000b"`, 1),
		"trailing":         good + `{}`, "array": `[]`, "null": `null`, "bad_utf8": strings.Replace(good, "测试", string([]byte{255}), 1),
		"raw_body_limit": strings.Repeat(" ", 65537) + good,
	}
	for n, raw := range cases {
		t.Run(n, func(t *testing.T) {
			if _, e := DecodeState([]byte(raw)); !errors.Is(e, ErrInvalid) {
				t.Fatalf("must reject closed input: %v", e)
			}
		})
	}
	// Whitespace contributes to raw body capacity, not canonical capacity.
	padded := strings.Repeat(" ", 65536-len(good)) + good
	if _, e := DecodeState([]byte(padded)); e != nil {
		t.Fatal("exact raw 64KiB must remain legal", e)
	}
}

func TestRootPresetRejectsInvalidStateAndUneditableTrees(t *testing.T) {
	cases := map[string]func(*State){
		"empty_name": func(s *State) { s.Name = " \t " }, "long_name": func(s *State) { s.Name = strings.Repeat("中", 101) },
		"hidden_duplicate": func(s *State) { s.HiddenColumnIDs = []string{titleID, titleID} }, "order_duplicate": func(s *State) { s.ColumnOrder = []string{titleID, titleID} },
		"unknown_column": func(s *State) { s.ColumnOrder = []string{otherID} }, "aux_column": func(s *State) { s.HiddenColumnIDs = []string{"selection"} },
		"invented_system": func(s *State) { s.ColumnOrder = []string{"updatedBy"} }, "all_business_hidden": func(s *State) { s.HiddenColumnIDs = []string{titleID, amountID} },
		"zero_width": func(s *State) { s.ColumnWidths[titleID] = 0 }, "unsafe_width": func(s *State) { s.ColumnWidths[titleID] = 9007199254740992 },
		"negative_width": func(s *State) { s.ColumnWidths[titleID] = -1 }, "unknown_width": func(s *State) { s.ColumnWidths[otherID] = 80 },
		"text_sort":      func(s *State) { s.Sort = json.RawMessage(`{"fieldId":"` + titleID + `","direction":"asc"}`) },
		"system_id_sort": func(s *State) { s.Sort = json.RawMessage(`{"fieldId":"id","direction":"asc"}`) },
		"empty_group":    func(s *State) { s.Filter = json.RawMessage(`{"operator":"and","children":[]}`) },
		"bare_or_leaves": func(s *State) { s.Filter = bytes.Replace(rootGroup(`"x"`), []byte(`"and"`), []byte(`"or"`), 1) },
		"nested_and": func(s *State) {
			s.Filter = json.RawMessage(`{"operator":"and","children":[` + string(rootGroup(`"x"`)) + `]}`)
		},
		"unknown_condition":          func(s *State) { s.Filter = bytes.ReplaceAll(rootGroup(`"x"`), []byte(titleID), []byte(otherID)) },
		"numeric_not_decimal_string": func(s *State) { s.Filter = bytes.ReplaceAll(rootGroup(`12`), []byte(titleID), []byte(amountID)) },
		"too_many_leaves": func(s *State) {
			leaf := `{"fieldId":"` + titleID + `","operator":"eq","value":"x"}`
			s.Filter = json.RawMessage(`{"operator":"and","children":[` + strings.TrimSuffix(strings.Repeat(leaf+",", 21), ",") + `]}`)
		},
		"canonical_filter_limit": func(s *State) { s.Filter = rootGroup(`"` + strings.Repeat("中", 6000) + `"`) },
	}
	for n, edit := range cases {
		t.Run(n, func(t *testing.T) {
			s := rootState()
			edit(&s)
			if _, e := Prepare(s, rootPolicy(), rootFields()); !errors.Is(e, ErrInvalid) {
				t.Fatalf("invalid accepted or wrong class: %v", e)
			}
		})
	}
	s := rootState()
	s.Name = strings.Repeat("中", 100)
	s.Sort = json.RawMessage(`{"fieldId":"createdAt","direction":"asc"}`)
	if _, e := Prepare(s, rootPolicy(), rootFields()); e != nil {
		t.Fatal("100 Unicode chars and approved system time sort rejected", e)
	}
}

func TestRootPresetLiveCoverageAndDisplayPermissions(t *testing.T) {
	p := rootPolicy()
	p.OwnerID = "other"
	p.Grants = []appaccess.Grant{{AppID: p.AppID, ViewID: p.ViewID, Action: appaccess.Read, Scope: appaccess.All, Fields: []string{titleID}}, {AppID: p.AppID, ViewID: p.ViewID, Action: appaccess.Read, Scope: appaccess.Own, Fields: []string{amountID}}}
	s := rootState()
	s.HiddenColumnIDs = []string{amountID}
	s.ColumnWidths[amountID] = 150
	if _, e := Prepare(s, p, rootFields()); e != nil {
		t.Fatal("own-readable display setting does not require all-row criterion coverage", e)
	}
	s.Sort = json.RawMessage(`{"fieldId":"` + amountID + `","direction":"asc"}`)
	if _, e := Prepare(s, p, rootFields()); !errors.Is(e, ErrPermission) {
		t.Fatalf("own column sorted across all visible rows: %v", e)
	}
	s.Sort = json.RawMessage(`null`)
	p.Grants = p.Grants[:1]
	if _, e := Prepare(s, p, rootFields()); !errors.Is(e, ErrPermission) {
		t.Fatalf("unreadable display reference accepted: %v", e)
	}
	s = rootState()
	p.Grants = nil
	if _, e := Prepare(s, p, rootFields()); !errors.Is(e, ErrPermission) {
		t.Fatalf("no data.read accepted: %v", e)
	}
	p = rootPolicy()
	p.ResourceExists = false
	if _, e := Prepare(s, p, rootFields()); !errors.Is(e, ErrPermission) {
		t.Fatalf("owner bypassed missing resource: %v", e)
	}
}

func TestRootPresetStoredDefinitionInvalidatesWholeWithoutMutation(t *testing.T) {
	s := rootState()
	s.Filter = rootGroup(`"SECRET-OPERAND"`)
	s.ColumnWidths[amountID] = 170
	raw := rootJSON(t, s)
	kinds := map[string]appquery.FieldKind{titleID: appquery.Text, amountID: appquery.Number}
	original := append([]byte(nil), raw...)
	cases := []struct {
		name   string
		fields []appquery.Field
		policy appaccess.Policy
		kinds  map[string]appquery.FieldKind
		reason Reason
	}{
		{"unchanged", rootFields(), rootPolicy(), kinds, ""},
		{"unrelated_field_added", append(rootFields(), appquery.Field{ID: otherID, Kind: appquery.Boolean}), rootPolicy(), kinds, ""},
		{"deleted_criterion", rootFields()[1:], rootPolicy(), kinds, FieldUnavailable},
		{"deleted_display", rootFields()[:1], rootPolicy(), kinds, FieldUnavailable},
		{"retyped", []appquery.Field{{ID: titleID, Kind: appquery.Multiline}, {ID: amountID, Kind: appquery.Number}}, rootPolicy(), kinds, DefinitionChanged},
		{"missing_signature", rootFields(), rootPolicy(), map[string]appquery.FieldKind{}, DefinitionChanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if r := CheckStored(raw, tc.kinds, tc.policy, tc.fields); r != tc.reason {
				t.Fatalf("reason=%s want=%s", r, tc.reason)
			}
			if !bytes.Equal(original, raw) {
				t.Fatal("invalid read rewrote original definition")
			}
		})
	}
	p := rootPolicy()
	p.OwnerID = "other"
	p.Grants = []appaccess.Grant{{AppID: p.AppID, ViewID: p.ViewID, Action: appaccess.Read, Scope: appaccess.All, Fields: []string{amountID}}}
	if r := CheckStored(raw, kinds, p, rootFields()); r != PermissionChanged {
		t.Fatalf("revoked field not wholly invalid: %s", r)
	}
	if r := CheckStored([]byte(`{}`), kinds, rootPolicy(), rootFields()); r != DefinitionChanged {
		t.Fatalf("corrupt definition not fail closed: %s", r)
	}
}
