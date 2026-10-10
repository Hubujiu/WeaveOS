package apptemplates

import (
	"bytes"
	"encoding/json"
	"sort"
)

type References struct {
	UserIDs       []string `json:"userIds"`
	DepartmentIDs []string `json:"departmentIds"`
}
type Binding struct {
	Kind     string `json:"kind"`
	SourceID string `json:"sourceId"`
	TargetID string `json:"targetId"`
}

func ExternalReferences(in Manifest) (References, error) {
	m, e := NormalizeManifest(in)
	if e != nil {
		return References{}, e
	}
	return references(&m)
}
func references(m *Manifest) (References, error) {
	users, depts := map[string]bool{}, map[string]bool{}
	e := visitExternal(m, func(kind string, id *string) error {
		if !validID(*id) {
			return ErrInvalid
		}
		if kind == "user" {
			users[*id] = true
		} else {
			depts[*id] = true
		}
		return nil
	})
	if e != nil {
		return References{}, e
	}
	out := References{UserIDs: []string{}, DepartmentIDs: []string{}}
	for id := range users {
		out.UserIDs = append(out.UserIDs, id)
	}
	for id := range depts {
		out.DepartmentIDs = append(out.DepartmentIDs, id)
	}
	sort.Strings(out.UserIDs)
	sort.Strings(out.DepartmentIDs)
	return out, nil
}
func MapExternalReferences(in Manifest, bindings []Binding) (Manifest, error) {
	m, e := NormalizeManifest(in)
	if e != nil {
		return Manifest{}, e
	}
	refs, e := references(&m)
	if e != nil {
		return Manifest{}, e
	}
	wanted := map[string]map[string]bool{"user": {}, "department": {}}
	for _, id := range refs.UserIDs {
		wanted["user"][id] = true
	}
	for _, id := range refs.DepartmentIDs {
		wanted["department"][id] = true
	}
	targets := map[string]map[string]string{"user": {}, "department": {}}
	if len(bindings) != len(refs.UserIDs)+len(refs.DepartmentIDs) {
		return Manifest{}, ErrInvalid
	}
	for _, b := range bindings {
		if !wanted[b.Kind][b.SourceID] || !validID(b.TargetID) || targets[b.Kind][b.SourceID] != "" {
			return Manifest{}, ErrInvalid
		}
		targets[b.Kind][b.SourceID] = b.TargetID
	}
	e = visitExternal(&m, func(kind string, id *string) error {
		target := targets[kind][*id]
		if target == "" {
			return ErrInvalid
		}
		*id = target
		return nil
	})
	if e != nil {
		return Manifest{}, e
	}
	return NormalizeManifest(m)
}
func externalKind(kind string) string {
	switch kind {
	case "member":
		return "user"
	case "department":
		return "department"
	}
	return ""
}

// Only a successfully normalized, detached manifest reaches this visitor.
// Never recursively replace arbitrary strings: text may legitimately equal an ID.
func visitExternal(m *Manifest, visit func(string, *string) error) error {
	fields := map[string]map[string]string{}
	for ti := range m.Tables {
		table := &m.Tables[ti]
		fields[table.ID] = map[string]string{}
		for fi := range table.Fields {
			field := &table.Fields[fi]
			kind := externalKind(field.Kind)
			fields[table.ID][field.ID] = kind
			if kind != "" && !bytes.Equal(bytes.TrimSpace(field.Default), []byte("null")) {
				var id string
				if json.Unmarshal(field.Default, &id) != nil {
					return ErrInvalid
				}
				if e := visit(kind, &id); e != nil {
					return e
				}
				field.Default, _ = json.Marshal(id)
			}
		}
	}
	for gi := range m.PermissionGroups {
		for ui := range m.PermissionGroups[gi].MemberIDs {
			if e := visit("user", &m.PermissionGroups[gi].MemberIDs[ui]); e != nil {
				return e
			}
		}
	}
	for wi := range m.Workflows {
		w := &m.Workflows[wi]
		for ni := range w.Graph.Nodes {
			n := &w.Graph.Nodes[ni]
			if n.Approval != nil {
				for ui := range n.Approval.AssigneeIDs {
					if e := visit("user", &n.Approval.AssigneeIDs[ui]); e != nil {
						return e
					}
				}
			}
			if n.Kind == "condition" {
				raw, e := visitCondition(n.Condition, fields[w.TableID], visit)
				if e != nil {
					return e
				}
				n.Condition = raw
			}
		}
		for ti := range w.Triggers {
			raw, e := visitCondition(w.Triggers[ti].Condition, fields[w.TableID], visit)
			if e != nil {
				return e
			}
			w.Triggers[ti].Condition = raw
		}
	}
	return nil
}
func visitCondition(raw json.RawMessage, fields map[string]string, visit func(string, *string) error) (json.RawMessage, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return raw, nil
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return nil, ErrInvalid
	}
	var walk func(map[string]json.RawMessage) error
	walk = func(node map[string]json.RawMessage) error {
		if children, ok := node["children"]; ok {
			var nodes []map[string]json.RawMessage
			if json.Unmarshal(children, &nodes) != nil {
				return ErrInvalid
			}
			for _, child := range nodes {
				if e := walk(child); e != nil {
					return e
				}
			}
			node["children"], _ = json.Marshal(nodes)
			return nil
		}
		var field string
		if json.Unmarshal(node["fieldId"], &field) != nil {
			return ErrInvalid
		}
		kind := fields[field]
		value := node["value"]
		if kind == "" || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil
		}
		var id string
		if json.Unmarshal(value, &id) != nil {
			return ErrInvalid
		}
		if e := visit(kind, &id); e != nil {
			return e
		}
		node["value"], _ = json.Marshal(id)
		return nil
	}
	if e := walk(root); e != nil {
		return nil, e
	}
	b, e := json.Marshal(root)
	return b, e
}
