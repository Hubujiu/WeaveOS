package appmeta

import (
	"context"
	"fmt"
)

func invalid(reason string) error { return fmt.Errorf("%w: %s", ErrInvalid, reason) }

func index[T any](values []T, idOf func(T) ID, kind string) (map[ID]T, error) {
	result := make(map[ID]T, len(values))
	for _, value := range values {
		id := idOf(value)
		if id == "" {
			return nil, invalid("missing " + kind + " ID")
		}
		if _, exists := result[id]; exists {
			return nil, invalid("duplicate " + kind + " ID")
		}
		result[id] = value
	}
	return result, nil
}

// ValidateCatalog verifies references only. It does not choose group cardinality,
// visibility, ordering, lifecycle rules, or grants for the application center.
func ValidateCatalog(c Catalog) error {
	apps, err := index(c.Applications, func(a Application) ID { return a.ID }, "application")
	if err != nil {
		return err
	}
	groups, err := index(c.Groups, func(g ApplicationGroup) ID { return g.ID }, "application group")
	if err != nil {
		return err
	}
	seen := make(map[ApplicationGroupLink]bool, len(c.Links))
	for _, link := range c.Links {
		if _, ok := apps[link.ApplicationID]; !ok {
			return invalid("application group link has absent application")
		}
		if _, ok := groups[link.GroupID]; !ok {
			return invalid("application group link has absent group")
		}
		if seen[link] {
			return invalid("duplicate application group link")
		}
		seen[link] = true
	}
	return nil
}

// ValidateStructure validates one coherent application's ownership and references.
// Its iterative directory traversal examines every component, including detached
// cycles, in O(G+T+V) expected time and O(G+T+V) auxiliary memory.
func ValidateStructure(s Structure) error {
	appID := s.Application.ID
	if appID == "" {
		return invalid("missing application ID")
	}
	groups, err := index(s.Groups, func(g Group) ID { return g.ID }, "directory")
	if err != nil {
		return err
	}
	tables, err := index(s.Tables, func(t TableDefinition) ID { return t.ID }, "table")
	if err != nil {
		return err
	}
	if _, err := index(s.Views, func(v View) ID { return v.ID }, "view"); err != nil {
		return err
	}
	for _, g := range s.Groups {
		if g.ApplicationID != appID {
			return invalid("directory belongs to another application")
		}
		if g.ParentID != "" {
			if _, ok := groups[g.ParentID]; !ok {
				return invalid("directory parent is absent")
			}
		}
	}
	// Each path is completed before starting the next; state 1 therefore indicates
	// a back edge in the current path, and state 2 means a completed component.
	state := make(map[ID]uint8, len(groups))
	path := make([]ID, 0)
	for _, g := range s.Groups {
		path = path[:0]
		for current := g.ID; current != "" && state[current] != 2; current = groups[current].ParentID {
			if state[current] == 1 {
				return invalid("directory cycle")
			}
			state[current] = 1
			path = append(path, current)
		}
		for _, id := range path {
			state[id] = 2
		}
	}
	checkGroup := func(id ID) error {
		if id != "" {
			if _, ok := groups[id]; !ok {
				return invalid("table or view directory is absent")
			}
		}
		return nil
	}
	for _, table := range s.Tables {
		if table.ApplicationID != appID {
			return invalid("table belongs to another application")
		}
		if err := checkGroup(table.GroupID); err != nil {
			return err
		}
	}
	for _, view := range s.Views {
		if view.ApplicationID != appID {
			return invalid("view belongs to another application")
		}
		if _, ok := tables[view.TableID]; !ok {
			return invalid("view table is absent")
		}
		if err := checkGroup(view.GroupID); err != nil {
			return err
		}
	}
	return nil
}

// ReadStructure is a validation boundary, not an authorization or SQL adapter.
// A failed read/validation never returns partially usable application metadata.
func ReadStructure(ctx context.Context, reader Reader, appID ID) (Structure, error) {
	if ctx == nil || reader == nil || appID == "" {
		return Structure{}, invalid("missing reader, context or application ID")
	}
	if err := ctx.Err(); err != nil {
		return Structure{}, err
	}
	s, err := reader.LoadStructure(ctx, appID)
	if err != nil {
		return Structure{}, err
	}
	if err := ctx.Err(); err != nil {
		return Structure{}, err
	}
	if s.Application.ID != appID {
		return Structure{}, invalid("stored application does not match requested owner")
	}
	if err := ValidateStructure(s); err != nil {
		return Structure{}, err
	}
	return s, nil
}
