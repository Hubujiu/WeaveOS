package appschema

import (
	"fmt"
	"regexp"
	"strings"
)

var internalID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// BuildPlan is pure: it performs no database calls and emits no DDL.
func BuildPlan(tableID string, before, after []Field) (Plan, error) {
	if !internalID.MatchString(tableID) {
		return Plan{}, fmt.Errorf("%w: internal table ID", ErrInvalid)
	}
	old, err := validateFields(before)
	if err != nil {
		return Plan{}, err
	}
	current, err := validateFields(after)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{TableID: tableID, TableName: physicalID("t_", tableID)}
	for _, input := range after {
		field := current[input.ID]
		p.Fields = append(p.Fields, field)
		column := physicalID("f_", field.ID)
		previous, exists := old[field.ID]
		if !exists {
			f := field
			p.Changes = append(p.Changes, Change{Operation: AddColumn, Column: column, After: &f})
			continue
		}
		for _, changed := range []struct {
			op  Operation
			yes bool
		}{{AlterType, previous.Type != field.Type}, {AlterDefault, !sameValue(previous.Default, field.Default)}, {AlterRequired, previous.Required != field.Required}} {
			if changed.yes {
				b, a := previous, field
				p.Changes = append(p.Changes, Change{Operation: changed.op, Column: column, Before: &b, After: &a})
			}
		}
	}
	for _, input := range before {
		if _, exists := current[input.ID]; !exists {
			f := old[input.ID]
			p.Changes = append(p.Changes, Change{Operation: DropColumn, Column: physicalID("f_", f.ID), Before: &f})
		}
	}
	return p, nil
}

func physicalID(prefix, id string) string { return prefix + strings.ReplaceAll(id, "-", "") }
func validateFields(fields []Field) (map[string]Field, error) {
	result := make(map[string]Field, len(fields))
	for _, f := range fields {
		if !internalID.MatchString(f.ID) {
			return nil, fmt.Errorf("%w: internal field ID", ErrInvalid)
		}
		if _, exists := result[f.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate field ID", ErrInvalid)
		}
		if f.Type != Text && f.Type != Boolean {
			return nil, ErrUnsupportedType
		}
		if f.Default != nil {
			if err := validateValue(*f.Default, f.Type); err != nil {
				return nil, err
			}
			copy := *f.Default
			f.Default = &copy
		}
		result[f.ID] = f
	}
	return result, nil
}
func validateValue(v Value, typ Type) error {
	if v.Type != typ {
		return fmt.Errorf("%w: default/backfill type mismatch", ErrInvalid)
	}
	switch typ {
	case Text:
		if strings.ContainsRune(v.Text, 0) || v.Boolean {
			return fmt.Errorf("%w: text value", ErrInvalid)
		}
	case Boolean:
		if v.Text != "" {
			return fmt.Errorf("%w: boolean value", ErrInvalid)
		}
	default:
		return ErrUnsupportedType
	}
	return nil
}
func sameValue(a, b *Value) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
