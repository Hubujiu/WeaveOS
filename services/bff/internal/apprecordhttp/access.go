package apprecordhttp

import (
	"context"
	"encoding/json"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/jackc/pgx/v5"
)

func scope(s appaccess.Scope) string {
	switch s {
	case appaccess.All:
		return "all"
	case appaccess.Own:
		return "own"
	default:
		return "none"
	}
}

// ResolveAccess adapts registered same-RR facts to the single appaccess
// evaluator. History uses the same scope relation on its own complete tuples.
// No resource/grant is obtained from JSON or inherited from a parent menu.
func ResolveAccess(_ context.Context, tx pgx.Tx, facts applications.RecordContext) (appstructure.RecordAccess, error) {
	if tx == nil || !appfields.ValidID(facts.Actor.ID) || !appfields.ValidID(facts.App.ID) || !appfields.ValidID(facts.ViewID) || !appfields.ValidID(facts.TableID) {
		return appstructure.RecordAccess{}, appstructure.ErrUnavailable
	}
	p := appaccess.Policy{ActorID: facts.Actor.ID, AppID: facts.App.ID, OwnerID: facts.App.OwnerUserID, ViewID: facts.ViewID, ResourceExists: true, BootstrapAdmin: facts.Actor.BootstrapAdmin}
	history := p
	menu := p.ActorID == p.OwnerID || p.BootstrapAdmin
	for _, g := range facts.Grants {
		if g.ResourceKind != "form" || g.ResourceID != facts.ViewID {
			continue
		}
		if g.Action == "menu.enter" {
			menu = true
			continue
		}
		var row appaccess.Scope
		switch g.RowScope {
		case "all":
			row = appaccess.All
		case "own":
			row = appaccess.Own
		default:
			return appstructure.RecordAccess{}, appstructure.ErrUnavailable
		}
		grant := appaccess.Grant{AppID: facts.App.ID, ViewID: facts.ViewID, Action: appaccess.Action(g.Action), Scope: row, Fields: g.Fields}
		if g.Action == "data.history" {
			grant.Action = appaccess.Read
			history.Grants = append(history.Grants, grant)
		} else {
			p.Grants = append(p.Grants, grant)
		}
	}
	var fields []appfields.Field
	if json.Unmarshal(facts.Fields, &fields) != nil || fields == nil {
		return appstructure.RecordAccess{}, appstructure.ErrUnavailable
	}
	a := appstructure.RecordAccess{MenuEnter: menu, Create: p.CanCreate(nil), Read: scope(p.VisibleScope()), Edit: "none", History: scope(history.VisibleScope()), Fields: map[string]appstructure.FieldAccess{}}
	edit := appaccess.None
	for _, f := range fields {
		if !appfields.ValidID(f.ID) {
			return appstructure.RecordAccess{}, appstructure.ErrUnavailable
		}
		e := p.FieldScope(appaccess.Edit, f.ID)
		if e > edit {
			edit = e
		}
		a.Fields[f.ID] = appstructure.FieldAccess{Read: scope(p.FieldScope(appaccess.Read, f.ID)), Create: p.FieldScope(appaccess.Create, f.ID) == appaccess.All, Edit: scope(e), History: scope(history.FieldScope(appaccess.Read, f.ID))}
	}
	a.Edit = scope(edit)
	return a, nil
}
