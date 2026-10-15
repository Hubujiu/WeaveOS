package flowgraph

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidNodeName checks optional display metadata without changing its text.
// An empty string means the caller should use the existing node-kind label.
func ValidNodeName(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= 100 && !strings.ContainsRune(value, 0) && (value == "" || strings.TrimSpace(value) != "")
}

// DecodeNodeNameJSON refuses encoding/json's lossy replacement of unpaired
// UTF-16 escapes. It accepts literal replacement characters and escaped slashes.
func DecodeNodeNameJSON(raw []byte) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' || !utf8.Valid(raw) || !json.Valid(raw) {
		return "", ErrInvalid
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return "", ErrInvalid
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return "", ErrInvalid
		}
		n, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil {
			return "", ErrInvalid
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return "", ErrInvalid
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return "", ErrInvalid
			}
			low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff {
				return "", ErrInvalid
			}
			i += 6
		}
	}
	var value string
	if json.Unmarshal(raw, &value) != nil || !ValidNodeName(value) {
		return "", ErrInvalid
	}
	return value, nil
}
