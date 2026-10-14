package appstructure

import (
	"strings"
	"unicode/utf8"
)

// ValidWorkflowActionReason checks audit text without modifying its bytes.
func ValidWorkflowActionReason(reason string) bool {
	return utf8.ValidString(reason) && utf8.RuneCountInString(reason) <= 500 && strings.TrimSpace(reason) != "" && !strings.ContainsRune(reason, 0)
}
