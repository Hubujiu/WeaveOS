package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/jackc/pgx/v5"
	"sort"
)

// The source owner supplies this adapter. Historical IDs never receive a cascade FK.
func (a *Application) references(c context.Context, tx pgx.Tx, app, view string, in Input) error {
	old := map[string]appfields.Field{}
	if view != "" {
		d, e := definition(c, tx, app, view, false)
		if e != nil {
			return e
		}
		for _, f := range d.Fields {
			old[f.ID] = f
		}
	}
	groups := map[string][]string{}
	for _, f := range in.Fields {
		if f.Kind != "member" && f.Kind != "department" || string(f.Default) == "null" {
			continue
		}
		previous, ok := old[f.ID]
		if ok && previous.Kind == f.Kind && jsonEqual(previous.Default, f.Default) {
			continue
		}
		var id string
		if json.Unmarshal(f.Default, &id) != nil {
			return invalid()
		}
		groups[f.Kind] = append(groups[f.Kind], id)
	}
	for _, kind := range []string{"member", "department"} {
		ids := groups[kind]
		if len(ids) == 0 {
			continue
		}
		if a.References == nil {
			return ErrUnavailable
		}
		sort.Strings(ids)
		if e := a.References.Validate(c, tx, kind, ids); e != nil {
			return e
		}
	}
	return nil
}
