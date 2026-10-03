package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

// ResolveRecordAccess is injected by the V015 policy consumer. Facts and Tx
// originate from live Session/resource reads, never from HTTP JSON.
type ResolveRecordAccess func(context.Context, pgx.Tx, applications.RecordContext) (RecordAccess, error)
type FieldAccess struct {
	Read    string `json:"read"`
	Create  bool   `json:"create"`
	Edit    string `json:"edit"`
	History string `json:"-"`
}
type RecordAccess struct {
	MenuEnter, Create   bool
	Read, Edit, History string
	Fields              map[string]FieldAccess
}
type RuntimeInput struct {
	Decimal       *appfields.DecimalConfig `json:"decimal,omitempty"`
	TimePrecision string                   `json:"timePrecision,omitempty"`
	Options       *[]appfields.Option      `json:"options,omitempty"`
	ReferenceKind string                   `json:"referenceKind,omitempty"`
}
type RuntimeQuery struct {
	Operators       []string `json:"operators"`
	Sortable        bool     `json:"sortable"`
	QuickSearchable bool     `json:"quickSearchable"`
}
type RuntimeField struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Kind         string                 `json:"kind"`
	Required     bool                   `json:"required"`
	Presentation appfields.Presentation `json:"presentation"`
	Input        RuntimeInput           `json:"input"`
	Default      json.RawMessage        `json:"default,omitempty"`
	Access       FieldAccess            `json:"access"`
	Query        RuntimeQuery           `json:"query"`
}
type RuntimeCapabilities struct {
	Create      bool   `json:"create"`
	Read        string `json:"read"`
	Edit        string `json:"edit"`
	Search      bool   `json:"search"`
	DraftCreate bool   `json:"draftCreate"`
	DraftEdit   bool   `json:"draftEdit"`
}
type RuntimeView struct {
	AppID          string                 `json:"appId"`
	TableID        string                 `json:"tableId"`
	ViewID         string                 `json:"viewId"`
	SchemaVersion  int64                  `json:"schemaVersion"`
	ViewVersion    int64                  `json:"viewVersion"`
	PolicyRevision int64                  `json:"policyRevision"`
	Fields         []RuntimeField         `json:"fields"`
	Layout         []appfields.LayoutNode `json:"layout"`
	Capabilities   RuntimeCapabilities    `json:"capabilities"`
}

func ProjectRuntime(facts applications.RecordContext, access RecordAccess) (RuntimeView, error) {
	var view RuntimeView
	if !access.MenuEnter {
		return view, applications.ErrDenied
	}
	if !validScope(access.Read) || !validScope(access.Edit) {
		return view, ErrUnavailable
	}
	if !access.Create && access.Read == "none" && access.Edit == "none" {
		return view, applications.ErrDenied
	}
	if !facts.SchemaReady {
		return view, &Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
	}
	var fields []appfields.Field
	var layout []appfields.LayoutNode
	if json.Unmarshal(facts.Fields, &fields) != nil || fields == nil || json.Unmarshal(facts.Layout, &layout) != nil || layout == nil {
		return view, ErrUnavailable
	}
	view = RuntimeView{AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID, SchemaVersion: facts.SchemaVersion, ViewVersion: facts.ViewVersion, PolicyRevision: facts.App.PolicyRevision, Fields: []RuntimeField{}, Layout: []appfields.LayoutNode{}, Capabilities: RuntimeCapabilities{Create: access.Create, Read: access.Read, Edit: access.Edit, Search: access.Read != "none", DraftCreate: access.Create, DraftEdit: access.Edit != "none"}}
	allowed := map[string]bool{}
	for _, f := range fields {
		mask, ok := access.Fields[f.ID]
		if !ok {
			continue
		}
		if !validScope(mask.Read) || !validScope(mask.Edit) {
			return RuntimeView{}, ErrUnavailable
		}
		if mask.Read == "none" && !mask.Create && mask.Edit == "none" {
			continue
		}
		allowed[f.ID] = true
		out := RuntimeField{ID: f.ID, Name: f.Name, Kind: f.Kind, Required: f.Required, Presentation: f.Presentation, Access: mask, Query: RuntimeQuery{Operators: []string{}}}
		if mask.Create {
			out.Default = append(json.RawMessage{}, f.Default...)
		}
		switch f.Kind {
		case "number", "money":
			var cfg appfields.DecimalConfig
			if json.Unmarshal(f.Config, &cfg) != nil {
				return RuntimeView{}, ErrUnavailable
			}
			out.Input.Decimal = &cfg
		case "datetime":
			var cfg struct {
				Precision string `json:"precision"`
			}
			if json.Unmarshal(f.Config, &cfg) != nil {
				return RuntimeView{}, ErrUnavailable
			}
			out.Input.TimePrecision = cfg.Precision
		case "single_select", "multi_select":
			var cfg struct {
				Options []appfields.Option `json:"options"`
			}
			if json.Unmarshal(f.Config, &cfg) != nil || cfg.Options == nil {
				return RuntimeView{}, ErrUnavailable
			}
			out.Input.Options = &cfg.Options
		case "member", "department":
			out.Input.ReferenceKind = f.Kind
		}
		if access.Read != "none" && scopeCovers(mask.Read, access.Read) {
			out.Query.Operators = []string{"eq", "neq"}
			switch f.Kind {
			case "number", "money", "date", "datetime":
				out.Query.Operators = append(out.Query.Operators, "gt", "gte", "lt", "lte")
				out.Query.Sortable = true
			case "text", "multiline":
				out.Query.QuickSearchable = true
			}
		}
		view.Fields = append(view.Fields, out)
	}
	var prune func([]appfields.LayoutNode) []appfields.LayoutNode
	prune = func(nodes []appfields.LayoutNode) []appfields.LayoutNode {
		out := []appfields.LayoutNode{}
		for _, n := range nodes {
			if n.Kind == "field" && !allowed[n.FieldID] {
				continue
			}
			if n.Kind == "system_field" && access.Read == "none" {
				continue
			}
			if n.Kind == "group" {
				n.Children = prune(n.Children)
				if len(n.Children) == 0 {
					continue
				}
			}
			out = append(out, n)
		}
		return out
	}
	view.Layout = prune(layout)
	return view, nil
}
func validScope(v string) bool           { return v == "none" || v == "own" || v == "all" }
func scopeCovers(have, want string) bool { return have == "all" || have == "own" && want == "own" }
func fullRecordAccess(facts applications.RecordContext) (RecordAccess, error) {
	var fields []appfields.Field
	if json.Unmarshal(facts.Fields, &fields) != nil {
		return RecordAccess{}, ErrUnavailable
	}
	a := RecordAccess{MenuEnter: true, Create: true, Read: "all", Edit: "all", History: "all", Fields: map[string]FieldAccess{}}
	for _, f := range fields {
		a.Fields[f.ID] = FieldAccess{Read: "all", Create: true, Edit: "all", History: "all"}
	}
	return a, nil
}
func (a *Application) recordAccess(c context.Context, tx pgx.Tx, facts applications.RecordContext) (RecordAccess, error) {
	if facts.Actor.BootstrapAdmin || facts.Actor.ID == facts.App.OwnerUserID {
		return fullRecordAccess(facts)
	}
	if a.RecordAccess == nil {
		return RecordAccess{}, ErrUnavailable
	}
	return a.RecordAccess(c, tx, facts)
}
func (a *Application) runtime(c context.Context, p session.Principal, app, view string) (RuntimeView, error) {
	tx, facts, e := (&applications.Application{Pool: a.Pool}).BeginRecordRead(c, p, app, view)
	if e != nil {
		return RuntimeView{}, e
	}
	defer tx.Rollback(context.Background())
	access, e := a.recordAccess(c, tx, facts)
	if e != nil {
		return RuntimeView{}, e
	}
	out, e := ProjectRuntime(facts, access)
	if e == nil {
		e = tx.Commit(c)
	}
	return out, e
}
