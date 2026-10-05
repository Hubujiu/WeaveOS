package workflowevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"os"
	"reflect"
	"strings"
	"testing"
)

func rootEvidenceID(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }
func rootEvidenceHeader() Header {
	return Header{AppID: rootEvidenceID(101), TableID: rootEvidenceID(102), ViewID: rootEvidenceID(103), RecordID: rootEvidenceID(104), CreatedBy: rootEvidenceID(105), SchemaVersion: 7, RecordVersion: 11, CreatedAt: "2026-10-05T10:00:00Z", UpdatedAt: "2026-10-05T10:01:00Z"}
}
func rootEvidenceMoney() Field {
	return Field{Definition: appfields.Field{ID: rootEvidenceID(1), Name: "Amount", Kind: "money", Default: json.RawMessage("null"), Config: json.RawMessage(`{"scale":2,"precision":18,"roundingMode":"HALF_UP","roundingPlaces":2}`)}, Value: json.RawMessage(`"12.35"`)}
}
func rootEvidenceText(n int, value string) Field {
	raw, _ := json.Marshal(value)
	return Field{Definition: appfields.Field{ID: rootEvidenceID(n), Name: "Text", Kind: "text", Default: json.RawMessage("null"), Config: json.RawMessage(`{"maxLength":null}`)}, Value: raw}
}
func rootEvidenceReference() Field {
	f := rootEvidenceText(3, rootEvidenceID(200))
	f.Definition.Kind = "member"
	f.Definition.Config = json.RawMessage(`{}`)
	f.Reference = &Reference{ID: rootEvidenceID(200), Label: "Former member", Deleted: true}
	return f
}

type rootEvidenceVector struct {
	FieldJSON    string `json:"fieldJSON"`
	FieldSHA     string `json:"fieldSHA256"`
	ManifestJSON string `json:"manifestJSON"`
	ManifestSHA  string `json:"manifestSHA256"`
}

func rootEvidenceGolden(t testing.TB) ([]byte, []byte) {
	t.Helper()
	raw, e := os.ReadFile("testdata/root-evidence-vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var v rootEvidenceVector
	if e = json.Unmarshal(raw, &v); e != nil {
		t.Fatal(e)
	}
	if v.FieldSHA != "e7d8bf7caf8d2d9732d1133133d77182750a91942c6855fed907b2ce70defc86" || v.ManifestSHA != "826bda2b5039c347c39e9ba6f7ff50dff9871f8890cf83e81b87c833e658fd4c" {
		t.Fatal("independent golden identities changed")
	}
	field := append([]byte("WVFEFL\x00\x01"), v.FieldJSON...)
	manifest := append([]byte("WVFEMF\x00\x01"), v.ManifestJSON...)
	for _, x := range []struct {
		body []byte
		hash string
	}{{field, v.FieldSHA}, {manifest, v.ManifestSHA}} {
		h := sha256.Sum256(x.body)
		if hex.EncodeToString(h[:]) != x.hash {
			t.Fatal("independent fixture hash mismatch")
		}
	}
	return field, manifest
}
func rootEvidenceWantInvalid(t *testing.T, e error) {
	t.Helper()
	if !errors.Is(e, ErrInvalid) {
		t.Fatalf("want invalid evidence, got %v", e)
	}
}

func TestRootEvidenceIndependentGoldenBytes(t *testing.T) {
	wantField, wantManifest := rootEvidenceGolden(t)
	input := rootEvidenceMoney()
	before, _ := json.Marshal(input)
	field, e := EncodeField(input)
	if e != nil || !bytes.Equal(field, wantField) {
		t.Fatalf("field golden mismatch: %v\n%s", e, field)
	}
	after, _ := json.Marshal(input)
	if !bytes.Equal(before, after) {
		t.Fatal("EncodeField mutated its caller")
	}
	decoded, e := DecodeField(field)
	if e != nil || string(decoded.Value) != `"12.35"` {
		t.Fatalf("stored exact decimal lost: %s %v", decoded.Value, e)
	}
	hash := sha256.Sum256(field)
	m := Manifest{Header: rootEvidenceHeader(), Fields: []FieldRef{{FieldID: input.Definition.ID, Hash: hash}}, VisibleFieldIDs: []string{input.Definition.ID}}
	manifest, e := EncodeManifest(m)
	if e != nil || !bytes.Equal(manifest, wantManifest) {
		t.Fatalf("manifest golden mismatch: %v\n%s", e, manifest)
	}
	dm, e := DecodeManifest(manifest)
	if e != nil || !reflect.DeepEqual(dm, m) {
		t.Fatalf("manifest roundtrip: %+v %v", dm, e)
	}
	b, e := Build(m.Header, []Field{input}, m.VisibleFieldIDs)
	if e != nil || !bytes.Equal(b.Manifest, wantManifest) || b.Hash != sha256.Sum256(wantManifest) || len(b.Fields) != 1 || b.Fields[0].FieldID != input.Definition.ID || b.Fields[0].Hash != hash || !bytes.Equal(b.Fields[0].Body, wantField) {
		t.Fatalf("bundle does not bind exact independent bytes: %v", e)
	}
}
func TestRootEvidenceDeterministicOrderingWithoutMutation(t *testing.T) {
	fields := []Field{rootEvidenceReference(), rootEvidenceMoney(), rootEvidenceText(2, "Café 李")}
	visible := []string{fields[0].Definition.ID, fields[1].Definition.ID}
	before, _ := json.Marshal(fields)
	prior := append([]string(nil), visible...)
	a, e := Build(rootEvidenceHeader(), fields, visible)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := json.Marshal(fields)
	if !bytes.Equal(before, after) || !reflect.DeepEqual(prior, visible) {
		t.Fatal("Build sorted or rewrote caller storage")
	}
	b, e := Build(rootEvidenceHeader(), []Field{fields[1], fields[2], fields[0]}, []string{visible[1], visible[0]})
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("input ordering changed canonical evidence: %v", e)
	}
	for i := 1; i < len(a.Fields); i++ {
		if a.Fields[i-1].FieldID >= a.Fields[i].FieldID {
			t.Fatal("bundle fields not canonically ordered")
		}
	}
}
func TestRootEvidenceEveryBindingChangesManifestHash(t *testing.T) {
	base := rootEvidenceHeader()
	f := rootEvidenceMoney()
	a, e := Build(base, []Field{f}, []string{f.Definition.ID})
	if e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*Header){"app": func(h *Header) { h.AppID = rootEvidenceID(301) }, "table": func(h *Header) { h.TableID = rootEvidenceID(302) }, "view": func(h *Header) { h.ViewID = rootEvidenceID(303) }, "record": func(h *Header) { h.RecordID = rootEvidenceID(304) }, "creator": func(h *Header) { h.CreatedBy = rootEvidenceID(305) }, "schema": func(h *Header) { h.SchemaVersion++ }, "record-version": func(h *Header) { h.RecordVersion++ }, "created-at": func(h *Header) { h.CreatedAt = "2026-10-05T09:00:00Z" }, "updated-at": func(h *Header) { h.UpdatedAt = "2026-10-05T11:00:00Z" }} {
		t.Run(name, func(t *testing.T) {
			h := base
			change(&h)
			b, e := Build(h, []Field{f}, []string{f.Definition.ID})
			if e != nil || b.Hash == a.Hash || b.Fields[0].Hash != a.Fields[0].Hash {
				t.Fatalf("identity not bound or unchanged field not reused: %v", e)
			}
		})
	}
	b, e := Build(base, []Field{f}, []string{})
	if e != nil || b.Hash == a.Hash || b.Fields[0].Hash != a.Fields[0].Hash {
		t.Fatal("visibility was not bound independently of field content")
	}
}
func TestRootEvidenceChangedFieldRetainsUnchangedContent(t *testing.T) {
	first := rootEvidenceText(1, "unchanged large value")
	second := rootEvidenceReference()
	fields := []Field{first, second}
	a, e := Build(rootEvidenceHeader(), fields, []string{first.Definition.ID, second.Definition.ID})
	if e != nil {
		t.Fatal(e)
	}
	changed := second
	ref := *second.Reference
	ref.Label = "Renamed later"
	ref.Deleted = false
	changed.Reference = &ref
	b, e := Build(rootEvidenceHeader(), []Field{first, changed}, []string{first.Definition.ID, second.Definition.ID})
	if e != nil {
		t.Fatal(e)
	}
	if a.Hash == b.Hash || a.Fields[0].Hash != b.Fields[0].Hash || a.Fields[1].Hash == b.Fields[1].Hash {
		t.Fatal("reference display changes with the same recordVersion were lost")
	}
	prior, e := DecodeField(a.Fields[1].Body)
	if e != nil || prior.Reference.Label != "Former member" || !prior.Reference.Deleted {
		t.Fatal("old evidence followed current reference display")
	}
	named := first
	named.Definition.Name = "Renamed field"
	c, e := Build(rootEvidenceHeader(), []Field{named, second}, []string{named.Definition.ID, second.Definition.ID})
	if e != nil || c.Fields[0].Hash == a.Fields[0].Hash || c.Hash == a.Hash {
		t.Fatal("historical field name not bound")
	}
}
func TestRootEvidenceExactTypesAndPresentation(t *testing.T) {
	cases := []struct {
		name  string
		field Field
		want  string
	}{}
	number := rootEvidenceMoney()
	number.Definition.Kind = "number"
	number.Definition.Config = json.RawMessage(`{"precision":38,"scale":18,"roundingPlaces":18,"roundingMode":"TOWARD_ZERO"}`)
	number.Value = json.RawMessage(`"99999999999999999999.123456789012345678"`)
	cases = append(cases, struct {
		name  string
		field Field
		want  string
	}{"38-digit", number, string(number.Value)})
	stamp := rootEvidenceText(2, "2026-10-05T18:01:49.123+08:00")
	stamp.Definition.Kind = "datetime"
	stamp.Definition.Config = json.RawMessage(`{"precision":"minute"}`)
	zone := "Asia/Shanghai"
	stamp.Definition.Presentation.DisplayTimeZone = &zone
	cases = append(cases, struct {
		name  string
		field Field
		want  string
	}{"datetime", stamp, `"2026-10-05T10:01:49.123Z"`})
	boolean := rootEvidenceText(2, "unused")
	boolean.Definition.Kind = "boolean"
	boolean.Definition.Config = json.RawMessage(`{}`)
	boolean.Value = json.RawMessage(`false`)
	cases = append(cases, struct {
		name  string
		field Field
		want  string
	}{"boolean", boolean, `false`})
	null := rootEvidenceText(2, "")
	null.Value = json.RawMessage(`null`)
	cases = append(cases, struct {
		name  string
		field Field
		want  string
	}{"null", null, `null`})
	changedRule := rootEvidenceMoney()
	changedRule.Definition.Config = json.RawMessage(`{"precision":18,"scale":2,"roundingPlaces":0,"roundingMode":"TOWARD_ZERO"}`)
	cases = append(cases, struct {
		name  string
		field Field
		want  string
	}{"do-not-reapply-rounding", changedRule, `"12.35"`})
	restricted := rootEvidenceText(2, "existing longer value")
	restricted.Definition.Config = json.RawMessage(`{"maxLength":1}`)
	cases = append(cases, struct {
		name  string
		field Field
		want  string
	}{"do-not-truncate-existing-text", restricted, `"existing longer value"`})
	requiredNull := null
	requiredNull.Definition.Required = true
	cases = append(cases, struct {
		name  string
		field Field
		want  string
	}{"preserve-stored-null", requiredNull, `null`})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, e := EncodeField(c.field)
			if e != nil {
				t.Fatal(e)
			}
			got, e := DecodeField(raw)
			if e != nil || string(got.Value) != c.want {
				t.Fatalf("stored type value changed meaning: %s %v", got.Value, e)
			}
			if c.name == "datetime" && (got.Definition.Presentation.DisplayTimeZone == nil || *got.Definition.Presentation.DisplayTimeZone != zone) {
				t.Fatal("historical presentation zone lost")
			}
		})
	}
}
func TestRootEvidenceInvalidFieldsAndReferences(t *testing.T) {
	for name, change := range map[string]func(*Field){
		"zero-id":        func(f *Field) { f.Definition.ID = "00000000-0000-0000-0000-000000000000" },
		"bad-id":         func(f *Field) { f.Definition.ID = "not-an-id" },
		"empty-name":     func(f *Field) { f.Definition.Name = " " },
		"unknown-kind":   func(f *Field) { f.Definition.Kind = "javascript" },
		"missing-value":  func(f *Field) { f.Value = nil },
		"bad-value-type": func(f *Field) { f.Value = json.RawMessage(`123.45`) },
		"extra-json":     func(f *Field) { f.Value = json.RawMessage(`"1" "2"`) },
		"duplicate-config-key": func(f *Field) {
			f.Definition.Config = json.RawMessage(`{"precision":18,"precision":19,"scale":2,"roundingPlaces":2,"roundingMode":"HALF_UP"}`)
		},
		"foreign-reference": func(f *Field) { f.Reference = &Reference{ID: rootEvidenceID(2), Label: "not a money reference"} },
		"huge-decimal":      func(f *Field) { f.Value = json.RawMessage(`"` + strings.Repeat("1", 200) + `"`) },
	} {
		t.Run(name, func(t *testing.T) {
			f := rootEvidenceMoney()
			change(&f)
			_, e := EncodeField(f)
			rootEvidenceWantInvalid(t, e)
		})
	}
	for _, name := range []string{"missing-reference", "wrong-reference", "empty-label", "null-with-reference"} {
		t.Run(name, func(t *testing.T) {
			f := rootEvidenceReference()
			switch name {
			case "missing-reference":
				f.Reference = nil
			case "wrong-reference":
				f.Reference.ID = rootEvidenceID(201)
			case "empty-label":
				f.Reference.Label = ""
			case "null-with-reference":
				f.Value = json.RawMessage(`null`)
			}
			_, e := EncodeField(f)
			rootEvidenceWantInvalid(t, e)
		})
	}
}
func TestRootEvidenceInvalidManifestBindings(t *testing.T) {
	raw, _ := rootEvidenceGolden(t)
	hash := sha256.Sum256(raw)
	base := Manifest{Header: rootEvidenceHeader(), Fields: []FieldRef{{FieldID: rootEvidenceID(1), Hash: hash}}, VisibleFieldIDs: []string{rootEvidenceID(1)}}
	for name, change := range map[string]func(*Manifest){
		"foreign-visible-field": func(m *Manifest) { m.VisibleFieldIDs = []string{rootEvidenceID(2)} },
		"duplicate-visible":     func(m *Manifest) { m.VisibleFieldIDs = append(m.VisibleFieldIDs, m.VisibleFieldIDs[0]) },
		"duplicate-field":       func(m *Manifest) { m.Fields = append(m.Fields, m.Fields[0]) },
		"zero-hash":             func(m *Manifest) { m.Fields[0].Hash = [32]byte{} },
		"negative-schema":       func(m *Manifest) { m.Header.SchemaVersion = -1 },
		"unsafe-version":        func(m *Manifest) { m.Header.RecordVersion = 9007199254740992 },
		"invalid-app":           func(m *Manifest) { m.Header.AppID = "wrong" },
		"invalid-time":          func(m *Manifest) { m.Header.UpdatedAt = "not-a-date" },
	} {
		t.Run(name, func(t *testing.T) {
			m := base
			m.Fields = append([]FieldRef(nil), base.Fields...)
			m.VisibleFieldIDs = append([]string(nil), base.VisibleFieldIDs...)
			change(&m)
			_, e := EncodeManifest(m)
			rootEvidenceWantInvalid(t, e)
		})
	}
	_, e := Build(rootEvidenceHeader(), []Field{rootEvidenceMoney(), rootEvidenceMoney()}, nil)
	rootEvidenceWantInvalid(t, e)
}
func TestRootEvidenceEmptyManifestAndEquivalentTime(t *testing.T) {
	h := rootEvidenceHeader()
	a, e := Build(h, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	m, e := DecodeManifest(a.Manifest)
	if e != nil || len(m.Fields) != 0 || len(m.VisibleFieldIDs) != 0 {
		t.Fatal("valid empty form evidence rejected")
	}
	h.CreatedAt = "2026-10-05T18:00:00+08:00"
	h.UpdatedAt = "2026-10-05T18:01:00+08:00"
	b, e := Build(h, []Field{}, []string{})
	if e != nil || !bytes.Equal(a.Manifest, b.Manifest) || a.Hash != b.Hash {
		t.Fatal("equivalent instants or empty collections changed canonical identity")
	}
}
func TestRootEvidenceDecoderRequiresCanonicalOwnedBytes(t *testing.T) {
	field, manifest := rootEvidenceGolden(t)
	for _, kind := range []string{"field", "manifest"} {
		t.Run(kind, func(t *testing.T) {
			original := field
			if kind == "manifest" {
				original = manifest
			}
			decode := func(raw []byte) error {
				if kind == "field" {
					_, e := DecodeField(raw)
					return e
				}
				_, e := DecodeManifest(raw)
				return e
			}
			badPrefix := bytes.Clone(original)
			badPrefix[0] ^= 1
			badVersion := bytes.Clone(original)
			badVersion[7] = 2
			unknown := bytes.Replace(original, []byte(`{`), []byte(`{"unexpected":1,`), 1)
			duplicate := bytes.Replace(original, []byte(`{`), []byte(`{"`+map[string]string{"field": "reference", "manifest": "visibleFieldIds"}[kind]+`":null,`), 1)
			for _, raw := range [][]byte{nil, original[:7], original[:len(original)-1], append(bytes.Clone(original), ' '), badPrefix, badVersion, unknown, duplicate, append(bytes.Clone(original), []byte(`{}`)...)} {
				rootEvidenceWantInvalid(t, decode(raw))
			}
			spaced := append(bytes.Clone(original[:8]), []byte("\n"+string(original[8:]))...)
			rootEvidenceWantInvalid(t, decode(spaced))
		})
	}
	before := bytes.Clone(field)
	got, e := DecodeField(field)
	if e != nil {
		t.Fatal(e)
	}
	got.Value[1] = '9'
	got.Definition.Config[1] = 'x'
	if !bytes.Equal(before, field) {
		t.Fatal("returned RawMessage aliases original body")
	}
	fresh, e := DecodeField(field)
	if e != nil || string(fresh.Value) != `"12.35"` {
		t.Fatal("decoded mutation corrupted durable input")
	}
}
func TestRootEvidenceResourceBounds(t *testing.T) {
	if MaxFields != 1600 || MaxFieldBytes != 4*1024*1024 || MaxManifestBytes != 1024*1024 || MaxBundleBytes != 64*1024*1024 || MaxDepth != 64 {
		t.Fatal("Root resource contract changed")
	}
	_, e := DecodeField(make([]byte, MaxFieldBytes+1))
	if !errors.Is(e, ErrTooLarge) {
		t.Fatalf("oversized body not bounded first: %v", e)
	}
	_, e = DecodeManifest(make([]byte, MaxManifestBytes+1))
	if !errors.Is(e, ErrTooLarge) {
		t.Fatalf("oversized manifest not bounded first: %v", e)
	}
	refs := make([]FieldRef, MaxFields+1)
	for i := range refs {
		refs[i] = FieldRef{FieldID: rootEvidenceID(i + 1), Hash: sha256.Sum256([]byte(fmt.Sprint(i)))}
	}
	_, e = EncodeManifest(Manifest{Header: rootEvidenceHeader(), Fields: refs})
	if !errors.Is(e, ErrTooLarge) {
		t.Fatalf("too many fields: %v", e)
	}
	_, e = EncodeField(rootEvidenceText(1, strings.Repeat("a", MaxFieldBytes)))
	if !errors.Is(e, ErrTooLarge) {
		t.Fatalf("encoded field over budget: %v", e)
	}
	field, _ := rootEvidenceGolden(t)
	deep := strings.Repeat("[", MaxDepth+1) + "0" + strings.Repeat("]", MaxDepth+1)
	nested := bytes.Replace(field, []byte(`"12.35"`), []byte(deep), 1)
	_, e = DecodeField(nested)
	if !errors.Is(e, ErrTooLarge) {
		t.Fatalf("nesting limit not checked before semantic decode: %v", e)
	}
}
func TestRootEvidenceTotalBundleBudgetReturnsNoPartialResult(t *testing.T) {
	shared, _ := json.Marshal(strings.Repeat("a", 2*1024*1024))
	fields := make([]Field, 33)
	for i := range fields {
		fields[i] = rootEvidenceText(i+1, "")
		fields[i].Value = shared
	}
	got, e := Build(rootEvidenceHeader(), fields, nil)
	if !errors.Is(e, ErrTooLarge) || got.Hash != ([32]byte{}) || len(got.Manifest) != 0 || len(got.Fields) != 0 {
		t.Fatalf("aggregate budget did not reject atomically: %v", e)
	}
}
func TestRootEvidenceSelectLabelsAndOrderingAreHistoricalContent(t *testing.T) {
	a, b := rootEvidenceID(301), rootEvidenceID(302)
	f := rootEvidenceText(4, "")
	f.Definition.Kind = "multi_select"
	f.Definition.Config = json.RawMessage(fmt.Sprintf(`{"options":[{"id":%q,"label":"First"},{"id":%q,"label":"Second"}]}`, a, b))
	f.Value = json.RawMessage(fmt.Sprintf(`[%q,%q]`, b, a))
	raw, e := EncodeField(f)
	if e != nil {
		t.Fatal(e)
	}
	got, e := DecodeField(raw)
	if e != nil || string(got.Value) != fmt.Sprintf(`[%q,%q]`, b, a) {
		t.Fatalf("selection changed its stored array order: %s %v", got.Value, e)
	}
	newer := f
	newer.Definition.Config = bytes.Replace(f.Definition.Config, []byte("First"), []byte("Renamed"), 1)
	next, e := EncodeField(newer)
	if e != nil || bytes.Equal(raw, next) {
		t.Fatal("option rename did not change content identity")
	}
	old, e := DecodeField(raw)
	if e != nil || !bytes.Contains(old.Definition.Config, []byte("First")) || bytes.Contains(old.Definition.Config, []byte("Renamed")) {
		t.Fatal("old option label was rewritten")
	}
}
func TestRootEvidenceFieldStorageReusesOnlyIdenticalContent(t *testing.T) {
	fields := make([]Field, 32)
	for i := range fields {
		fields[i] = rootEvidenceText(i+1, strings.Repeat("x", 256))
	}
	unique := map[[32]byte][]byte{}
	var first Bundle
	for version := 0; version < 16; version++ {
		h := rootEvidenceHeader()
		h.RecordVersion = int64(version + 1)
		fields[0] = rootEvidenceText(1, fmt.Sprintf("changed-%d", version))
		bundle, e := Build(h, fields, nil)
		if e != nil {
			t.Fatal(e)
		}
		if version == 0 {
			first = bundle
		}
		for _, blob := range bundle.Fields {
			if old, ok := unique[blob.Hash]; ok && !bytes.Equal(old, blob.Body) {
				t.Fatal("one content hash has different canonical bytes")
			}
			unique[blob.Hash] = bytes.Clone(blob.Body)
		}
	}
	if len(unique) != 31+16 {
		t.Fatalf("unchanged field content did not share: %d", len(unique))
	}
	original, e := DecodeField(first.Fields[0].Body)
	if e != nil || string(original.Value) != `"changed-0"` {
		t.Fatal("old version no longer reconstructs its original field")
	}
	t.Logf("synthetic 32 fields x16 versions: whole-record field copies=512; distinct reusable field blobs=%d; manifest references still=512; not a database size or latency measurement", len(unique))
}
func FuzzRootEvidenceFieldDecode(f *testing.F) {
	field, _ := rootEvidenceGolden(f)
	f.Add(field)
	f.Add([]byte("WVFEFL\x00\x01{}"))
	f.Fuzz(func(t *testing.T, b []byte) {
		v, e := DecodeField(b)
		if e == nil {
			out, e := EncodeField(v)
			if e != nil || !bytes.Equal(out, b) {
				t.Fatal("accepted noncanonical field bytes")
			}
		}
	})
}
func FuzzRootEvidenceManifestDecode(f *testing.F) {
	_, manifest := rootEvidenceGolden(f)
	f.Add(manifest)
	f.Add([]byte("WVFEMF\x00\x01{}"))
	f.Fuzz(func(t *testing.T, b []byte) {
		v, e := DecodeManifest(b)
		if e == nil {
			out, e := EncodeManifest(v)
			if e != nil || !bytes.Equal(out, b) {
				t.Fatal("accepted noncanonical manifest bytes")
			}
		}
	})
}
