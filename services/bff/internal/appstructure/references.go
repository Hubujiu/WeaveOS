package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5"
	"sort"
)

type CurrentSources struct{}

func (CurrentSources) Validate(c context.Context, tx pgx.Tx, kind string, ids []string) error {
	query := ""
	switch kind {
	case "member":
		query = "SELECT id::text FROM auth.users WHERE status='active' AND id=ANY($1::uuid[]) ORDER BY id"
	case "department":
		query = "SELECT id::text FROM personnel.departments WHERE id=ANY($1::uuid[]) ORDER BY id"
	default:
		return ErrUnavailable
	}
	var readOnly string
	if e := tx.QueryRow(c, "SHOW transaction_read_only").Scan(&readOnly); e != nil {
		return e
	}
	if readOnly == "off" {
		query += " FOR SHARE"
	}
	rows, e := tx.Query(c, query, ids)
	if e != nil {
		return e
	}
	seen := map[string]bool{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			break
		}
		seen[id] = true
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if !seen[id] {
			return applications.ErrResourceInvalid
		}
	}
	return nil
}

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
