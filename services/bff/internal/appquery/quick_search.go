package appquery

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
)

type QuickPlan struct {
	Predicate        string
	Arguments        []any
	Canonical        json.RawMessage
	ReferencedFields []string
}

func CompileQuickSearch(json.RawMessage, []Field, appaccess.Policy, int) (QuickPlan, error) {
	return QuickPlan{}, ErrInvalid
}
