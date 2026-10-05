// Package workflowevidence holds immutable, content-addressed approval evidence.
// User authorization and collection from real records belong to its caller.
package workflowevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
)

var ErrInvalid = errors.New("invalid workflow evidence")
var ErrTooLarge = errors.New("workflow evidence exceeds resource limits")

const MaxFields = 1600
const MaxFieldBytes = 4 * 1024 * 1024
const MaxManifestBytes = 1024 * 1024
const MaxBundleBytes = 64 * 1024 * 1024
const MaxDepth = 64

type Reference struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Deleted bool   `json:"deleted"`
}
type Field struct {
	Definition appfields.Field `json:"definition"`
	Value      json.RawMessage `json:"value"`
	Reference  *Reference      `json:"reference"`
}
type Header struct {
	AppID         string `json:"appId"`
	TableID       string `json:"tableId"`
	ViewID        string `json:"viewId"`
	RecordID      string `json:"recordId"`
	CreatedBy     string `json:"createdBy"`
	SchemaVersion int64  `json:"schemaVersion"`
	RecordVersion int64  `json:"recordVersion"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}
type FieldRef struct {
	FieldID string
	Hash    [32]byte
}
type Manifest struct {
	Header          Header
	Fields          []FieldRef
	VisibleFieldIDs []string
}
type Blob struct {
	FieldID string
	Hash    [32]byte
	Body    []byte
}
type Bundle struct {
	Manifest []byte
	Hash     [32]byte
	Fields   []Blob
}

const fieldPrefix = "WVFEFL\x00\x01"
const manifestPrefix = "WVFEMF\x00\x01"

// readJSON owns its result, retains numbers exactly, and checks resources before
// callers interpret the value's physical type.
func readJSON(body []byte, limit int) (any, error) {
	if len(body) > limit {
		return nil, ErrTooLarge
	}
	if !utf8.Valid(body) {
		return nil, ErrInvalid
	}
	if !validUnicodeEscapes(body) {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	value, err := readValue(d, 0)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return value, nil
}

// validUnicodeEscapes checks only string escapes; the decoder remains
// responsible for the full JSON grammar.
func validUnicodeEscapes(body []byte) bool {
	inString := false
	for i := 0; i < len(body); i++ {
		if body[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || body[i] != '\\' {
			continue
		}
		i++
		if i >= len(body) {
			return false
		}
		if body[i] != 'u' {
			continue
		}
		code, ok := hexQuad(body, i+1)
		if !ok {
			return false
		}
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return false
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if len(body)-i-1 < 6 || body[i+1] != '\\' || body[i+2] != 'u' {
			return false
		}
		low, ok := hexQuad(body, i+3)
		if !ok || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

func hexQuad(body []byte, start int) (uint16, bool) {
	if len(body)-start < 4 {
		return 0, false
	}
	var code uint16
	for _, c := range body[start : start+4] {
		code <<= 4
		switch {
		case c >= '0' && c <= '9':
			code |= uint16(c - '0')
		case c >= 'a' && c <= 'f':
			code |= uint16(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			code |= uint16(c - 'A' + 10)
		default:
			return 0, false
		}
	}
	return code, true
}

func readValue(d *json.Decoder, depth int) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, ErrInvalid
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return token, nil
	}
	if depth >= MaxDepth {
		return nil, ErrTooLarge
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return nil, ErrInvalid
			}
			key, ok := token.(string)
			if !ok {
				return nil, ErrInvalid
			}
			if _, duplicate := object[key]; duplicate {
				return nil, ErrInvalid
			}
			value, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, ErrInvalid
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for d.More() {
			value, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, ErrInvalid
		}
		return array, nil
	default:
		return nil, ErrInvalid
	}
}

func canonicalJSON(body []byte, limit int) ([]byte, error) {
	value, err := readJSON(body, limit)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, ErrInvalid
	}
	if len(out) > limit {
		return nil, ErrTooLarge
	}
	return out, nil
}

func strictDecode(body []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	d.UseNumber()
	if d.Decode(target) != nil {
		return ErrInvalid
	}
	if d.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	return nil
}

func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	nonzero := false
	for i := 0; i < len(id); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if id[i] != '-' {
				return false
			}
			continue
		}
		c := id[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
		nonzero = nonzero || c != '0'
	}
	return nonzero
}

// Check all lengths before scanning any scalar or interpreting metadata.
func directStrings(values []string, limit int) error {
	remaining := limit
	for _, value := range values {
		if len(value) > remaining {
			return ErrTooLarge
		}
		remaining -= len(value)
	}
	for _, value := range values {
		if !validText(value) {
			return ErrInvalid
		}
	}
	return nil
}

func validText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func utcTime(value string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", ErrInvalid
	}
	utc := t.UTC()
	if utc.Year() < 0 || utc.Year() > 9999 {
		return "", ErrInvalid
	}
	return utc.Format(time.RFC3339Nano), nil
}

// Decimal validation is lexical: no floating point, rounding or expansion.
func validDecimal(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	digits := value
	if digits[0] == '-' {
		digits = digits[1:]
	}
	if len(digits) == 0 {
		return false
	}
	parts := strings.Split(digits, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return false
	}
	if len(parts[0]) > 1 && parts[0][0] == '0' {
		return false
	}
	if len(parts) == 2 && (len(parts[1]) == 0 || len(parts[1]) > 18) {
		return false
	}
	count := 0
	leading := true
	for _, part := range parts {
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return false
			}
			if part[i] != '0' {
				leading = false
			}
			if !leading {
				count++
			}
		}
	}
	return count <= 38
}

func physicalValue(kind string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch kind {
	case "text", "multiline":
		text, ok := value.(string)
		if !ok || !validText(text) {
			return nil, ErrInvalid
		}
	case "number", "money":
		text, ok := value.(string)
		if !ok || !validDecimal(text) {
			return nil, ErrInvalid
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return nil, ErrInvalid
		}
	case "member", "department", "single_select":
		id, ok := value.(string)
		if !ok || !validID(id) {
			return nil, ErrInvalid
		}
	case "multi_select":
		ids, ok := value.([]any)
		if !ok {
			return nil, ErrInvalid
		}
		for _, item := range ids {
			id, ok := item.(string)
			if !ok || !validID(id) {
				return nil, ErrInvalid
			}
		}
	case "date":
		text, ok := value.(string)
		if !ok || len(text) != 10 {
			return nil, ErrInvalid
		}
		t, err := time.Parse("2006-01-02", text)
		if err != nil || t.Format("2006-01-02") != text {
			return nil, ErrInvalid
		}
	case "datetime":
		text, ok := value.(string)
		if !ok {
			return nil, ErrInvalid
		}
		return utcTime(text)
	default:
		return nil, ErrInvalid
	}
	return value, nil
}

func normalizedField(input Field) (Field, error) {
	direct := []string{input.Definition.ID, input.Definition.Name, input.Definition.Kind}
	if input.Definition.Presentation.HelpText != nil {
		direct = append(direct, *input.Definition.Presentation.HelpText)
	}
	if input.Definition.Presentation.DisplayTimeZone != nil {
		direct = append(direct, *input.Definition.Presentation.DisplayTimeZone)
	}
	if input.Reference != nil {
		direct = append(direct, input.Reference.ID, input.Reference.Label)
	}
	if err := directStrings(direct, MaxFieldBytes); err != nil {
		return Field{}, err
	}
	// Parse every RawMessage before applying configuration or physical type rules.
	config, err := readJSON(input.Definition.Config, MaxFieldBytes)
	if err != nil {
		return Field{}, err
	}
	defaultValue, err := readJSON(input.Definition.Default, MaxFieldBytes)
	if err != nil {
		return Field{}, err
	}
	value, err := readJSON(input.Value, MaxFieldBytes)
	if err != nil {
		return Field{}, err
	}
	if !validID(input.Definition.ID) {
		return Field{}, ErrInvalid
	}
	if input.Definition.Kind == "single_select" || input.Definition.Kind == "multi_select" {
		object, ok := config.(map[string]any)
		if !ok {
			return Field{}, ErrInvalid
		}
		options, ok := object["options"].([]any)
		if !ok {
			return Field{}, ErrInvalid
		}
		for _, item := range options {
			option, ok := item.(map[string]any)
			if !ok {
				return Field{}, ErrInvalid
			}
			id, ok := option["id"].(string)
			if !ok || !validID(id) {
				return Field{}, ErrInvalid
			}
		}
	}
	definition := input.Definition
	definition.Config, err = json.Marshal(config)
	if err != nil {
		return Field{}, ErrInvalid
	}
	definition.Default = json.RawMessage("null")
	definitions, err := appfields.NormalizeFields([]appfields.Field{definition})
	if err != nil {
		return Field{}, ErrInvalid
	}
	definition = definitions[0]
	defaultValue, err = physicalValue(definition.Kind, defaultValue)
	if err != nil {
		return Field{}, err
	}
	value, err = physicalValue(definition.Kind, value)
	if err != nil {
		return Field{}, err
	}
	definition.Default, err = json.Marshal(defaultValue)
	if err != nil {
		return Field{}, ErrInvalid
	}
	out := Field{Definition: definition}
	out.Value, err = json.Marshal(value)
	if err != nil {
		return Field{}, ErrInvalid
	}
	if (definition.Kind == "member" || definition.Kind == "department") && value != nil {
		reference := input.Reference
		if reference == nil || !validID(reference.ID) || reference.ID != value ||
			strings.TrimSpace(reference.Label) == "" || !validText(reference.Label) {
			return Field{}, ErrInvalid
		}
		copy := *reference
		out.Reference = &copy
	} else if input.Reference != nil {
		return Field{}, ErrInvalid
	}
	return out, nil
}

func encode(prefix string, value any, limit int) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, ErrInvalid
	}
	canonical, err := canonicalJSON(body, limit)
	if err != nil {
		return nil, err
	}
	if len(canonical) > limit-len(prefix) {
		return nil, ErrTooLarge
	}
	out := make([]byte, 0, len(prefix)+len(canonical))
	out = append(out, prefix...)
	out = append(out, canonical...)
	return out, nil
}

func decodeBody(body []byte, prefix string, limit int) ([]byte, error) {
	if len(body) > limit {
		return nil, ErrTooLarge
	}
	if len(body) < len(prefix) || string(body[:len(prefix)]) != prefix {
		return nil, ErrInvalid
	}
	return canonicalJSON(body[len(prefix):], limit-len(prefix))
}

func EncodeField(input Field) ([]byte, error) {
	normalized, err := normalizedField(input)
	if err != nil {
		return nil, err
	}
	return encode(fieldPrefix, normalized, MaxFieldBytes)
}

func DecodeField(body []byte) (Field, error) {
	canonical, err := decodeBody(body, fieldPrefix, MaxFieldBytes)
	if err != nil {
		return Field{}, err
	}
	var field Field
	if err = strictDecode(canonical, &field); err != nil {
		return Field{}, err
	}
	out, err := normalizedField(field)
	if err != nil {
		return Field{}, err
	}
	encoded, err := encode(fieldPrefix, out, MaxFieldBytes)
	if err != nil {
		return Field{}, err
	}
	if !bytes.Equal(body, encoded) {
		return Field{}, ErrInvalid
	}
	return out, nil
}

type wireFieldRef struct {
	FieldID string `json:"fieldId"`
	Hash    string `json:"hash"`
}
type wireManifest struct {
	Header          Header         `json:"header"`
	Fields          []wireFieldRef `json:"fields"`
	VisibleFieldIDs []string       `json:"visibleFieldIds"`
}

func normalizedHeader(header Header) (Header, error) {
	if err := directStrings([]string{header.AppID, header.TableID, header.ViewID, header.RecordID,
		header.CreatedBy, header.CreatedAt, header.UpdatedAt}, MaxManifestBytes); err != nil {
		return Header{}, err
	}
	for _, id := range []string{header.AppID, header.TableID, header.ViewID, header.RecordID, header.CreatedBy} {
		if !validID(id) {
			return Header{}, ErrInvalid
		}
	}
	const maxVersion = 9007199254740991
	if header.SchemaVersion < 1 || header.SchemaVersion > maxVersion ||
		header.RecordVersion < 1 || header.RecordVersion > maxVersion {
		return Header{}, ErrInvalid
	}
	var err error
	header.CreatedAt, err = utcTime(header.CreatedAt)
	if err != nil {
		return Header{}, err
	}
	header.UpdatedAt, err = utcTime(header.UpdatedAt)
	if err != nil {
		return Header{}, err
	}
	return header, nil
}

func EncodeManifest(input Manifest) ([]byte, error) {
	if len(input.Fields) > MaxFields || len(input.VisibleFieldIDs) > MaxFields {
		return nil, ErrTooLarge
	}
	header, err := normalizedHeader(input.Header)
	if err != nil {
		return nil, err
	}
	fields := append([]FieldRef{}, input.Fields...)
	sort.Slice(fields, func(i, j int) bool { return fields[i].FieldID < fields[j].FieldID })
	visible := append([]string{}, input.VisibleFieldIDs...)
	sort.Strings(visible)
	wire := wireManifest{Header: header, Fields: make([]wireFieldRef, 0, len(fields)), VisibleFieldIDs: visible}
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if !validID(field.FieldID) || seen[field.FieldID] || field.Hash == ([32]byte{}) {
			return nil, ErrInvalid
		}
		seen[field.FieldID] = true
		wire.Fields = append(wire.Fields, wireFieldRef{FieldID: field.FieldID, Hash: hex.EncodeToString(field.Hash[:])})
	}
	for i, id := range visible {
		if !validID(id) || !seen[id] || i > 0 && id == visible[i-1] {
			return nil, ErrInvalid
		}
	}
	return encode(manifestPrefix, wire, MaxManifestBytes)
}

func DecodeManifest(body []byte) (Manifest, error) {
	canonical, err := decodeBody(body, manifestPrefix, MaxManifestBytes)
	if err != nil {
		return Manifest{}, err
	}
	var wire wireManifest
	if err = strictDecode(canonical, &wire); err != nil {
		return Manifest{}, err
	}
	if len(wire.Fields) > MaxFields || len(wire.VisibleFieldIDs) > MaxFields {
		return Manifest{}, ErrTooLarge
	}
	out := Manifest{Header: wire.Header, Fields: make([]FieldRef, 0, len(wire.Fields)),
		VisibleFieldIDs: append([]string{}, wire.VisibleFieldIDs...)}
	for _, field := range wire.Fields {
		if len(field.Hash) != 64 {
			return Manifest{}, ErrInvalid
		}
		hash, err := hex.DecodeString(field.Hash)
		if err != nil || len(hash) != 32 {
			return Manifest{}, ErrInvalid
		}
		ref := FieldRef{FieldID: field.FieldID}
		copy(ref.Hash[:], hash)
		out.Fields = append(out.Fields, ref)
	}
	encoded, err := EncodeManifest(out)
	if err != nil {
		return Manifest{}, err
	}
	if !bytes.Equal(body, encoded) {
		return Manifest{}, ErrInvalid
	}
	return out, nil
}

func Build(header Header, fields []Field, visible []string) (Bundle, error) {
	if len(fields) > MaxFields {
		return Bundle{}, ErrTooLarge
	}
	manifest := Manifest{Header: header, Fields: make([]FieldRef, 0, len(fields)), VisibleFieldIDs: visible}
	blobs := make([]Blob, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	total := 0
	for _, field := range fields {
		id := field.Definition.ID
		if !validID(id) || seen[id] {
			return Bundle{}, ErrInvalid
		}
		seen[id] = true
		body, err := EncodeField(field)
		if err != nil {
			return Bundle{}, err
		}
		if len(body) > MaxBundleBytes-total {
			return Bundle{}, ErrTooLarge
		}
		total += len(body)
		hash := sha256.Sum256(body)
		blobs = append(blobs, Blob{FieldID: id, Hash: hash, Body: body})
		manifest.Fields = append(manifest.Fields, FieldRef{FieldID: id, Hash: hash})
	}
	body, err := EncodeManifest(manifest)
	if err != nil {
		return Bundle{}, err
	}
	if len(body) > MaxBundleBytes-total {
		return Bundle{}, ErrTooLarge
	}
	sort.Slice(blobs, func(i, j int) bool { return blobs[i].FieldID < blobs[j].FieldID })
	return Bundle{Manifest: body, Hash: sha256.Sum256(body), Fields: blobs}, nil
}
