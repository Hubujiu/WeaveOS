// Package appaccess evaluates trusted, live form-view data grants. Loading and
// locking those facts is the caller's responsibility; request JSON is never a
// source of Policy or Grant values.
package appaccess

type Action string

const (
	Create Action = "data.create"
	Read   Action = "data.read"
	Edit   Action = "data.edit"
)

type Scope uint8

const (
	None Scope = iota
	Own
	All
)

// Grant is one complete persisted tuple after enabled group membership has
// been verified by the caller. Empty Fields is meaningful for defaults-only
// Create, but never makes a row visible for Read.
type Grant struct {
	AppID, ViewID string
	Action        Action
	Scope         Scope
	Fields        []string
}

type Policy struct {
	ActorID, AppID, OwnerID, ViewID string
	ResourceExists                  bool
	BootstrapAdmin                  bool
	Grants                          []Grant
}

func (p Policy) valid() bool {
	return p.ActorID != "" && p.AppID != "" && p.ViewID != "" && p.ResourceExists
}

func (p Policy) full() bool {
	return p.valid() && (p.BootstrapAdmin || p.ActorID == p.OwnerID)
}

func (p Policy) grants(action Action, fieldID string, requireField bool) Scope {
	if !p.valid() {
		return None
	}
	if p.full() {
		return All
	}
	var scope Scope
	for _, grant := range p.Grants {
		if grant.AppID != p.AppID || grant.ViewID != p.ViewID || grant.Action != action ||
			grant.Scope != All && grant.Scope != Own || action == Create && grant.Scope != All {
			continue
		}
		if requireField && !has(grant.Fields, fieldID) || action != Create && len(grant.Fields) == 0 {
			continue
		}
		if grant.Scope > scope {
			scope = grant.Scope
		}
	}
	return scope
}

func has(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// VisibleScope is derived only from read grants with a nonempty field mask.
func (p Policy) VisibleScope() Scope { return p.grants(Read, "", false) }

// FieldScope never combines a field from one action with another action's
// scope. Own uses immutable createdBy supplied by the row loader.
func (p Policy) FieldScope(action Action, fieldID string) Scope {
	if fieldID == "" || action != Read && action != Edit && action != Create && action != History {
		return None
	}
	return p.grants(action, fieldID, true)
}

func (p Policy) ReadFields(createdBy string, fieldIDs []string) []string {
	seen := make(map[string]bool, len(fieldIDs))
	var out []string
	for _, id := range fieldIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		scope := p.FieldScope(Read, id)
		if scope == All || scope == Own && createdBy == p.ActorID {
			out = append(out, id)
		}
	}
	return out
}

// CoversRead rejects an entire filter/sort when any named field cannot be
// read across the full visible row scope. The caller must apply this before
// compiling a SQL predicate that touches the field.
func (p Policy) CoversRead(fields []string) bool {
	visible := p.VisibleScope()
	if visible == None {
		return false
	}
	for _, id := range fields {
		if id == "" || p.FieldScope(Read, id) < visible {
			return false
		}
	}
	return true
}

func (p Policy) CanCreate(fieldIDs []string) bool {
	if p.grants(Create, "", false) != All {
		return false
	}
	for _, id := range fieldIDs {
		if id == "" || p.FieldScope(Create, id) != All {
			return false
		}
	}
	return true
}

func (p Policy) CanEdit(createdBy string, fieldIDs []string) bool {
	if !p.valid() {
		return false
	}
	// An empty PATCH remains an authorized edit action, not a way to mint an
	// operation without any live record permission.
	if len(fieldIDs) == 0 {
		scope := p.grants(Edit, "", false)
		return scope == All || scope == Own && createdBy == p.ActorID
	}
	for _, id := range fieldIDs {
		if id == "" {
			return false
		}
		scope := p.FieldScope(Edit, id)
		if scope != All && !(scope == Own && createdBy == p.ActorID) {
			return false
		}
	}
	return true
}
