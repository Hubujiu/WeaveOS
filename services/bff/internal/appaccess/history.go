package appaccess

const History Action = "data.history"

// HistoryFields is the current read AND history field projection, never an
// authorization from a historical task or evidence manifest.
func (p Policy) HistoryFields(createdBy string, fieldIDs []string) []string { return nil }

func (p Policy) HistoryScope() Scope { return None }
