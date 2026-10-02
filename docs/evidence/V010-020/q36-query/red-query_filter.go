package personnel

import "encoding/json"

// FilterPlan uses only fixed SQL identifiers and separate bind arguments.
// Canonical is normalized input; date bounds in Arguments are absolute instants.
type FilterPlan struct {
	Predicate string
	Arguments []any
	Canonical json.RawMessage
}

func CompileFilter(view string, raw json.RawMessage, firstParameter int) (FilterPlan, error) {
	return FilterPlan{}, ErrNotImplemented
}
