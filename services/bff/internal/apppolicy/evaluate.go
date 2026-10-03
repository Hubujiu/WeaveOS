package apppolicy

// CanCreate checks only application creation, never existing-app management.
func CanCreate(actor TrustedActor) bool { return false }

// Allows checks a menu primitive (empty field) or one data cell. Authorization
// does not establish task eligibility, schema validity, or business-state legality.
func Allows(ctx TrustedContext, resource Resource, action Action, row RowFact, field string) bool {
	return false
}

// AllowedFields returns authorized requested fields, unique and in request order.
// Callers must check every changed field before an atomic multi-field write.
func AllowedFields(ctx TrustedContext, resource Resource, action Action, row RowFact, requested []string) []string {
	return nil
}
