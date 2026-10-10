package apppresets

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DecodeState is a closed DTO decoder, including RawMessage children. The
// bounded token pass prevents encoding/json's last-key-wins and case aliases.
func DecodeState(raw []byte) (State, error) {
	if len(raw) > 65536 || !unicodeJSON(raw) {
		return State{}, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if walkJSON(d, 0) != nil {
		return State{}, ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return State{}, ErrInvalid
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || len(object) != 6 {
		return State{}, ErrInvalid
	}
	for _, key := range []string{"name", "filter", "sort", "hiddenColumnIds", "columnOrder", "columnWidths"} {
		v, ok := object[key]
		if !ok {
			return State{}, ErrInvalid
		}
		if key != "filter" && key != "sort" && bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return State{}, ErrInvalid
		}
	}
	var state State
	if json.Unmarshal(raw, &state) != nil || state.HiddenColumnIDs == nil || state.ColumnOrder == nil || state.ColumnWidths == nil || strings.ContainsRune(state.Name, 0) {
		return State{}, ErrInvalid
	}
	return state, nil
}
func walkJSON(d *json.Decoder, depth int) error {
	if depth > 12 {
		return ErrInvalid
	}
	token, e := d.Token()
	if e != nil {
		return ErrInvalid
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			s, ok := key.(string)
			if e != nil || !ok || keys[s] {
				return ErrInvalid
			}
			keys[s] = true
			if walkJSON(d, depth+1) != nil {
				return ErrInvalid
			}
		}
		token, e = d.Token()
		if e != nil || token != json.Delim('}') {
			return ErrInvalid
		}
	case '[':
		for d.More() {
			if walkJSON(d, depth+1) != nil {
				return ErrInvalid
			}
		}
		token, e = d.Token()
		if e != nil || token != json.Delim(']') {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

// Reject unmatched escaped UTF-16 before the standard decoder can replace it.
// Literal U+FFFD remains a legitimate Unicode character.
func unicodeJSON(raw []byte) bool {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return false
	}
	inString := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil || n == 0 {
			return false
		}
		i += 4
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		} else if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
	}
	return true
}

// DecodeRequest preserves original request identity independently of live
// authorization; all permissions and schema CAS are checked by the service.
func DecodeRequest(raw []byte, update bool) (Request, error) {
	if len(raw) > 65536 || !unicodeJSON(raw) {
		return Request{}, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if walkJSON(d, 0) != nil {
		return Request{}, ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return Request{}, ErrInvalid
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return Request{}, ErrInvalid
	}
	expected := 8
	if update {
		expected = 9
	}
	if len(object) != expected {
		return Request{}, ErrInvalid
	}
	var request Request
	if json.Unmarshal(object["operationId"], &request.OperationID) != nil || !validID(request.OperationID) || json.Unmarshal(object["expectedSchemaVersion"], &request.ExpectedSchemaVersion) != nil || request.ExpectedSchemaVersion < 1 || request.ExpectedSchemaVersion > maxVersion {
		return Request{}, ErrInvalid
	}
	delete(object, "operationId")
	delete(object, "expectedSchemaVersion")
	if update {
		if json.Unmarshal(object["expectedVersion"], &request.ExpectedVersion) != nil || request.ExpectedVersion < 1 || request.ExpectedVersion > maxVersion {
			return Request{}, ErrInvalid
		}
		delete(object, "expectedVersion")
	}
	stateRaw, e := json.Marshal(object)
	if e != nil {
		return Request{}, ErrInvalid
	}
	request.State, e = DecodeState(stateRaw)
	if e != nil {
		return Request{}, e
	}
	return request, nil
}
