package appschema

// BuildPlan is pure: it performs no database calls and emits no DDL.
func BuildPlan(tableID string, before, after []Field) (Plan, error) {
	return Plan{}, ErrNotImplemented
}
