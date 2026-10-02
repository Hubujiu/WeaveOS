package personnel

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Reject malformed Unicode before encoding/json can silently replace it. The
// persisted canonical form preserves valid user strings, including whitespace.
func draftJSONUnicode(raw []byte) bool {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return false
	}
	inString := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '"' {
			inString = !inString
			continue
		}
		if !inString || c != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n == 0 {
			return false
		} // PostgreSQL jsonb object constraint cannot accept NUL.
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff || utf16.DecodeRune(rune(n), rune(low)) == utf8.RuneError {
				return false
			}
			i += 6
		} else if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
	}
	return true
}

// Explicit fields are mandatory even for incomplete forms. Null is permitted
// only by the individual field schema, never used as a missing-field default.
func draftObject(raw []byte, fields ...string) (map[string]json.RawMessage, error) {
	if !draftJSONUnicode(raw) {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return nil, ErrInvalid
	}
	allowed := map[string]bool{}
	for _, name := range fields {
		allowed[name] = true
	}
	values := map[string]json.RawMessage{}
	for d.More() {
		token, e = d.Token()
		name, ok := token.(string)
		if e != nil || !ok || !allowed[name] || values[name] != nil {
			return nil, ErrInvalid
		}
		var value json.RawMessage
		if e = d.Decode(&value); e != nil {
			return nil, ErrInvalid
		}
		values[name] = value
	}
	if _, e = d.Token(); e != nil {
		return nil, ErrInvalid
	}
	if _, e = d.Token(); e != io.EOF || len(values) != len(fields) {
		return nil, ErrInvalid
	}
	return values, nil
}
func draftString(raw json.RawMessage, max int) (string, error) {
	var value string
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &value) != nil || utf8.RuneCountInString(value) > max {
		return "", ErrInvalid
	}
	return value, nil
}
func draftNullableID(raw json.RawMessage) (*string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, e := draftString(raw, 36)
	if e != nil || !validID(value) {
		return nil, ErrInvalid
	}
	return &value, nil
}
func draftStrings(raw json.RawMessage, ids bool) ([]string, error) {
	var values []json.RawMessage
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &values) != nil {
		return nil, ErrInvalid
	}
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		max := 160
		if ids {
			max = 36
		}
		value, e := draftString(raw, max)
		if e != nil || ids && !validID(value) {
			return nil, ErrInvalid
		}
		key := value
		if seen[key] {
			return nil, ErrInvalid
		}
		seen[key] = true
		result = append(result, value)
	}
	return result, nil
}
func canonicalDraftPayload(kind DraftKind, raw json.RawMessage) (json.RawMessage, error) {
	var fields []string
	switch kind {
	case DraftMemberIdentities:
		fields = []string{"identityIds"}
	case DraftMemberGroups:
		fields = []string{"operation", "departmentId", "sourceDepartmentId"}
	case DraftDepartment:
		fields = []string{"name", "parentId"}
	case DraftIdentity:
		fields = []string{"name", "description", "templateIds", "permissionCodes"}
	case DraftTemplate:
		fields = []string{"name", "description", "permissionCodes"}
	default:
		return nil, ErrInvalid
	}
	values, e := draftObject(raw, fields...)
	if e != nil {
		return nil, e
	}
	out := map[string]any{}
	for _, name := range fields {
		switch name {
		case "identityIds", "templateIds", "permissionCodes":
			out[name], e = draftStrings(values[name], name != "permissionCodes")
		case "departmentId", "sourceDepartmentId", "parentId":
			out[name], e = draftNullableID(values[name])
		case "name":
			out[name], e = draftString(values[name], 100)
		case "description":
			out[name], e = draftString(values[name], 1000)
		case "operation":
			var operation string
			operation, e = draftString(values[name], 6)
			if operation != "add" && operation != "remove" && operation != "move" {
				e = ErrInvalid
			}
			out[name] = operation
		}
		if e != nil {
			return nil, e
		}
	}
	// encoding/json sorts object keys; UTF-8 strings and array order are preserved.
	// No HTML-dependent byte expansion. JSON-required escapes remain canonical.
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if e = enc.Encode(out); e != nil {
		return nil, ErrInvalid
	}
	canonical := bytes.TrimSuffix(b.Bytes(), []byte("\n"))
	if len(canonical) > 65536 {
		return nil, ErrInvalid
	}
	return json.RawMessage(canonical), nil
}
func validDraftKind(kind DraftKind) bool {
	switch kind {
	case DraftMemberIdentities, DraftMemberGroups, DraftDepartment, DraftIdentity, DraftTemplate:
		return true
	}
	return false
}
func draftCreatePayload(in DraftCreateInput) (json.RawMessage, error) {
	if !validDraftKind(in.Kind) {
		return nil, ErrInvalid
	}
	member := in.Kind == DraftMemberIdentities || in.Kind == DraftMemberGroups
	if in.TargetID == nil {
		if member || in.BaseVersion != nil {
			return nil, ErrInvalid
		}
	} else {
		if !validID(*in.TargetID) || in.BaseVersion == nil {
			return nil, ErrInvalid
		}
		min := int64(1)
		if member {
			min = 0
		}
		if *in.BaseVersion < min || *in.BaseVersion > maxSafeVersion {
			return nil, ErrInvalid
		}
	}
	return canonicalDraftPayload(in.Kind, in.Payload)
}
