package applications

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
)

// RecordPolicy maps only server-loaded live facts; callers separately require menu.
func RecordPolicy(facts RecordContext) (appaccess.Policy, bool) {
	p := appaccess.Policy{ActorID: facts.Actor.ID, AppID: facts.App.ID, OwnerID: facts.App.OwnerUserID, ViewID: facts.ViewID, ResourceExists: appfields.ValidID(facts.TableID), BootstrapAdmin: facts.Actor.BootstrapAdmin}
	menu := p.BootstrapAdmin || p.ActorID == p.OwnerID
	for _, g := range facts.Grants {
		if g.ResourceKind != "form" || g.ResourceID != facts.ViewID {
			continue
		}
		if g.Action == "menu.enter" {
			menu = true
			continue
		}
		var scope appaccess.Scope
		switch g.RowScope {
		case "all":
			scope = appaccess.All
		case "own":
			scope = appaccess.Own
		default:
			return p, false
		}
		p.Grants = append(p.Grants, appaccess.Grant{AppID: facts.App.ID, ViewID: facts.ViewID, Action: appaccess.Action(g.Action), Scope: scope, Fields: g.Fields})
	}
	return p, menu
}
