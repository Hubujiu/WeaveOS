package appstructure

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidWorkflowActionReason checks audit text without modifying its bytes.
func ValidWorkflowActionReason(reason string) bool {
	return utf8.ValidString(reason) && utf8.RuneCountInString(reason) <= 500 && strings.TrimSpace(reason) != "" && !strings.ContainsRune(reason, 0)
}

// encoding/json replaces unpaired surrogate escapes. Reject these before
// decoding so an accepted audit reason can never silently change its text.
func validWorkflowReasonUnicodeEscapes(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		value, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		if value < 0xd800 || value > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}
