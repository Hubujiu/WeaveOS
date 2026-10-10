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
