package appaccess

const History Action = "data.history"

// HistoryFields is the current read AND history field projection, never an
// authorization from a historical task or evidence manifest.
func (p Policy) HistoryFields(createdBy string, fieldIDs []string) []string {
	readable := p.ReadFields(createdBy, fieldIDs)
	out := make([]string, 0, len(readable))
	for _, id := range readable {
		scope := p.FieldScope(History, id)
		if scope == All || scope == Own && createdBy == p.ActorID {
			out = append(out, id)
		}
	}
	return out
}

func (p Policy) HistoryScope() Scope { return p.grants(History, "", false) }
