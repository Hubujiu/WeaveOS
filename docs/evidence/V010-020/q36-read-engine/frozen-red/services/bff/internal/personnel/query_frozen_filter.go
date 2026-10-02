package personnel

import "encoding/json"

func compileFrozenFilter(view string, raw json.RawMessage, first int, bounds []QueryRange) (FilterPlan, error) {
	return FilterPlan{}, ErrNotImplemented
}
