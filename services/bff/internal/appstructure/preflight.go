package appstructure

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type tokenContext struct {
	KeyID, Actor, App, Table, View, Plan    string
	Schema, ViewVersion, Data, Dependencies int64
	Expires                                 int64
}

func (a *Application) now() time.Time {
	if a.Now != nil {
		return a.Now().UTC()
	}
	return time.Now().UTC()
}
func revisions(c context.Context, tx pgx.Tx, id string) (int64, int64, error) {
	var data, dep int64
	e := tx.QueryRow(c, "SELECT data_revision,dependency_revision FROM applications.logical_tables WHERE id=$1", id).Scan(&data, &dep)
	return data, dep, e
}
func (a *Application) tokenFacts(c context.Context, tx pgx.Tx, p session.Principal, d Definition, in Input) (tokenContext, error) {
	data, dep, e := revisions(c, tx, d.Table.ID)
	raw, _ := json.Marshal(struct {
		Fields   []appfields.Field
		Layout   []appfields.LayoutNode
		Mappings []OptionMapping
	}{in.Fields, in.Layout, in.OptionMappings})
	hash := sha256.Sum256(raw)
	return tokenContext{KeyID: a.ConfirmationKeyID, Actor: p.UserID, App: d.AppID, Table: d.Table.ID, View: d.Form.ID, Plan: hex.EncodeToString(hash[:]), Schema: d.Table.SchemaVersion, ViewVersion: d.Form.ViewVersion, Data: data, Dependencies: dep}, e
}
func (a *Application) confirmation(c context.Context, tx pgx.Tx, p session.Principal, d Definition, in Input) (*Confirmation, error) {
	if len(a.ConfirmationKey) < 32 || a.ConfirmationKeyID == "" {
		return nil, ErrUnavailable
	}
	facts, e := a.tokenFacts(c, tx, p, d, in)
	if e != nil {
		return nil, e
	}
	expires := a.now().Add(10 * time.Minute)
	facts.Expires = expires.Unix()
	raw, _ := json.Marshal(facts)
	mac := hmac.New(sha256.New, a.ConfirmationKey)
	mac.Write(raw)
	return &Confirmation{base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), expires.Format(time.RFC3339)}, nil
}
func (a *Application) verifyConfirmation(c context.Context, tx pgx.Tx, p session.Principal, d Definition, in Input) error {
	if in.ConfirmationToken == nil {
		return &Error{"APPLICATION_SCHEMA_CONFIRMATION_REQUIRED", map[string]any{"impacts": []Impact{}}}
	}
	if len(a.ConfirmationKey) < 32 || a.ConfirmationKeyID == "" {
		return ErrUnavailable
	}
	parts := strings.Split(*in.ConfirmationToken, ".")
	if len(parts) != 2 {
		return invalid()
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return invalid()
	}
	signed, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return invalid()
	}
	mac := hmac.New(sha256.New, a.ConfirmationKey)
	mac.Write(raw)
	if !hmac.Equal(signed, mac.Sum(nil)) {
		return invalid()
	}
	var saved tokenContext
	if json.Unmarshal(raw, &saved) != nil || saved.Actor != p.UserID || saved.App != d.AppID || saved.Table != d.Table.ID || saved.View != d.Form.ID || saved.KeyID != a.ConfirmationKeyID {
		return invalid()
	}
	if saved.Expires <= a.now().Unix() {
		return &Error{"APPLICATION_SCHEMA_CONFIRMATION_STALE", map[string]string{"reason": "expired"}}
	}
	expected, e := a.tokenFacts(c, tx, p, d, in)
	if e != nil {
		return e
	}
	expected.Expires = saved.Expires
	if !jsonEqual(expected, saved) {
		return &Error{"APPLICATION_SCHEMA_CONFIRMATION_STALE", map[string]string{"reason": "context_changed"}}
	}
	return nil
}
func (a *Application) inspect(c context.Context, tx pgx.Tx, p session.Principal, d Definition, in Input, sign bool) (Preflight, error) {
	if e := validateMappings(d, in); e != nil {
		return Preflight{}, e
	}
	plan := planFor(d, in)
	data, dep, e := revisions(c, tx, d.Table.ID)
	out := Preflight{AppID: d.AppID, TableID: d.Table.ID, ViewID: d.Form.ID, SchemaVersion: d.Table.SchemaVersion, ViewVersion: d.Form.ViewVersion, DataRevision: data, DependencyRevision: dep, Plan: plan, Impacts: []Impact{}, Dependencies: []Dependency{}, BlockingIssues: []Issue{}, SaveAllowed: true}
	if e != nil {
		return out, e
	}
	if d.Table.SchemaVersion != in.ExpectedSchemaVersion {
		return out, &Error{"APPLICATION_SCHEMA_CONFLICT", map[string]any{"currentSchemaVersion": d.Table.SchemaVersion}}
	}
	if d.Form.ViewVersion != in.ExpectedViewVersion {
		return out, &Error{"APPLICATION_VIEW_CONFLICT", map[string]any{"currentViewVersion": d.Form.ViewVersion}}
	}
	newFields := map[string]appfields.Field{}
	for _, f := range in.Fields {
		newFields[f.ID] = f
	}
	protected := []string{}
	removed := []string{}
	for _, ch := range plan.SchemaChanges {
		if ch.Kind == "remove" || ch.Kind == "change_type" || ch.Kind == "change_config" {
			protected = append(protected, ch.FieldID)
		}
		if ch.Kind == "remove" {
			removed = append(removed, ch.FieldID)
		}
	}
	if len(protected) > 0 {
		out.Dependencies, e = a.dependencies(c, tx, d, protected, removed)
		if e != nil {
			return out, e
		}
	}
	issue := func(code, id string) {
		for _, v := range out.BlockingIssues {
			if v.Code == code && len(v.FieldIDs) == 1 && v.FieldIDs[0] == id {
				return
			}
		}
		out.BlockingIssues = append(out.BlockingIssues, Issue{code, []string{id}})
	}
	var hasRows bool
	if d.Table.SchemaReady {
		if e = tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM "+tablePhysical(d.Table.ID)+")").Scan(&hasRows); e != nil {
			return out, e
		}
	}
	oldFields := map[string]appfields.Field{}
	for _, f := range d.Fields {
		oldFields[f.ID] = f
	}
	for _, f := range in.Fields {
		if _, exists := oldFields[f.ID]; !exists && hasRows && f.Required && string(f.Default) == "null" {
			issue("APPLICATION_SCHEMA_REQUIRED_BACKFILL", f.ID)
		}
	}
	for _, old := range d.Fields {
		next, exists := newFields[old.ID]
		if !d.Table.SchemaReady {
			continue
		}
		col := columnPhysical(old.ID)
		if !exists {
			var count int64
			if e = tx.QueryRow(c, "SELECT count("+col+") FROM "+tablePhysical(d.Table.ID)).Scan(&count); e != nil {
				return out, e
			}
			if count > 0 {
				out.Impacts = append(out.Impacts, Impact{old.ID, "column_removal", count, nil})
			}
			continue
		}
		if (old.Kind == "member" || old.Kind == "department" || next.Kind == "member" || next.Kind == "department") && old.Kind != next.Kind {
			issue("APPLICATION_SCHEMA_CONVERSION_FAILED", old.ID)
			continue
		}
		if jsonEqual(old, next) && len(in.OptionMappings) == 0 {
			continue
		}
		expression := "to_jsonb(" + col + ")"
		if old.Kind == "number" || old.Kind == "money" {
			expression = "to_jsonb(" + col + "::text)"
		}
		rows, err := tx.Query(c, "SELECT "+expression+" FROM "+tablePhysical(d.Table.ID))
		if err != nil {
			return out, err
		}
		mappingCounts := map[string]int64{}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				break
			}
			if raw == nil {
				raw = []byte("null")
			}
			if next.Required && bytes.Equal(raw, []byte("null")) {
				issue("APPLICATION_SCHEMA_REQUIRED_BACKFILL", old.ID)
				continue
			}
			mapped, counts, problem := mapSelection(old, next, raw, in.OptionMappings)
			if problem != "" {
				issue(problem, old.ID)
				continue
			}
			for id, count := range counts {
				mappingCounts[id] += count
			}
			if old.Kind != next.Kind && next.Kind == "text" && old.Kind != "multi_select" {
				if old.Kind == "datetime" {
					mapped, err = appfields.NormalizeValue(old, mapped)
					if err != nil {
						issue("APPLICATION_SCHEMA_CONVERSION_FAILED", old.ID)
						continue
					}
				}
				var value any
				json.Unmarshal(mapped, &value)
				if s, ok := value.(string); ok {
					mapped, _ = json.Marshal(s)
				} else {
					mapped, _ = json.Marshal(string(mapped))
				}
			}
			if old.Kind == "text" && next.Kind == "boolean" {
				var text string
				json.Unmarshal(mapped, &text)
				if text == "true" || text == "false" {
					mapped = []byte(text)
				}
			}
			if _, err = appfields.NormalizeValue(next, mapped); err != nil {
				issue("APPLICATION_SCHEMA_CONVERSION_FAILED", old.ID)
			}
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return out, e
		}
		for id, count := range mappingCounts {
			x := id
			out.Impacts = append(out.Impacts, Impact{old.ID, "option_mapping", count, &x})
		}
	}
	out.SaveAllowed = len(out.Dependencies) == 0 && len(out.BlockingIssues) == 0
	if sign && out.SaveAllowed && len(out.Impacts) > 0 {
		out.Confirmation, e = a.confirmation(c, tx, p, d, in)
	}
	return out, e
}
func mapSelection(old, next appfields.Field, raw []byte, mappings []OptionMapping) ([]byte, map[string]int64, string) {
	counts := map[string]int64{}
	if old.Kind != "single_select" && old.Kind != "multi_select" {
		return raw, counts, ""
	}
	if next.Kind != "single_select" && next.Kind != "multi_select" {
		return raw, counts, ""
	}
	if string(raw) == "null" {
		return raw, counts, ""
	}
	var ids []string
	if old.Kind == "single_select" {
		var id string
		if json.Unmarshal(raw, &id) != nil {
			return raw, counts, "APPLICATION_SCHEMA_CONVERSION_FAILED"
		}
		ids = []string{id}
	} else if json.Unmarshal(raw, &ids) != nil {
		return raw, counts, "APPLICATION_SCHEMA_CONVERSION_FAILED"
	}
	distinct := map[string]bool{}
	for _, id := range ids {
		distinct[id] = true
	}
	if old.Kind == "multi_select" && next.Kind == "single_select" && len(distinct) > 1 {
		return raw, counts, "APPLICATION_SCHEMA_CONVERSION_FAILED"
	}
	var cfg struct {
		Options []appfields.Option `json:"options"`
	}
	json.Unmarshal(next.Config, &cfg)
	allowed := map[string]bool{}
	for _, o := range cfg.Options {
		allowed[o.ID] = true
	}
	out := []string{}
	for id := range distinct {
		mapped := id
		clear := false
		found := false
		for _, m := range mappings {
			if m.FieldID == old.ID && m.FromOptionID == id {
				if found {
					return raw, counts, "APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED"
				}
				found = true
				counts[id]++
				if m.ToOptionID == nil {
					clear = true
				} else {
					mapped = *m.ToOptionID
				}
			}
		}
		if !allowed[id] && !found {
			return raw, counts, "APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED"
		}
		if clear {
			continue
		}
		if !allowed[mapped] {
			return raw, counts, "APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED"
		}
		out = append(out, mapped)
	}
	if len(out) == 0 {
		if next.Required {
			return raw, counts, "APPLICATION_SCHEMA_REQUIRED_BACKFILL"
		}
		return []byte("null"), counts, ""
	}
	var result []byte
	if next.Kind == "single_select" {
		result, _ = json.Marshal(out[0])
	} else {
		result, _ = json.Marshal(out)
	}
	return result, counts, ""
}
