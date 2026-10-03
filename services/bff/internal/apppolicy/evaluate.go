package apppolicy

import "strings"

// CanCreate checks only application creation, never existing-app management.
func CanCreate(actor TrustedActor) bool {
	return nonblank(actor.ID) && (actor.BootstrapAdmin || actor.CreateApp)
}

// Allows checks a menu primitive (empty field) or one data cell. Authorization
// does not establish task eligibility, schema validity, or business-state legality.
func Allows(ctx TrustedContext, resource Resource, action Action, row RowFact, field string) bool {
	if !validOperation(ctx, resource, action, row) {
		return false
	}
	if action == MenuEnter {
		if field != "" {
			return false
		}
	} else if !nonblank(field) {
		return false
	}
	if full(ctx) {
		return true
	}
	for _, group := range ctx.Groups {
		if !effectiveGroup(ctx, group) {
			continue
		}
		for _, grant := range group.Grants {
			if !matchingTuple(ctx, resource, action, row, grant) {
				continue
			}
			if action == MenuEnter || contains(grant.Fields, field) {
				return true
			}
		}
	}
	return false
}

// AllowedFields returns authorized requested fields, unique and in request order.
// Callers must check every changed field before an atomic multi-field write.
func AllowedFields(ctx TrustedContext, resource Resource, action Action, row RowFact, requested []string) []string {
	if (action != DataRead && action != DataEdit) || !validOperation(ctx, resource, action, row) {
		return nil
	}
	var allowed map[string]struct{}
	all := full(ctx)
	if !all {
		allowed = make(map[string]struct{})
		for _, group := range ctx.Groups {
			if !effectiveGroup(ctx, group) {
				continue
			}
			for _, grant := range group.Grants {
				if !matchingTuple(ctx, resource, action, row, grant) {
					continue
				}
				for _, field := range grant.Fields {
					if nonblank(field) {
						allowed[field] = struct{}{}
					}
				}
			}
		}
	}
	seen := make(map[string]struct{}, len(requested))
	var result []string
	for _, field := range requested {
		if !nonblank(field) {
			continue
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		if _, exists := allowed[field]; all || exists {
			result = append(result, field)
		}
	}
	return result
}

func validOperation(ctx TrustedContext, resource Resource, action Action, row RowFact) bool {
	if !nonblank(ctx.Actor.ID) || !ctx.Application.Exists || !nonblank(ctx.Application.ID) ||
		!resource.Exists || !nonblank(resource.Ref.ID) || !nonblank(resource.Ref.Kind) ||
		resource.Ref.ApplicationID != ctx.Application.ID {
		return false
	}
	switch action {
	case MenuEnter:
		return true
	case DataRead, DataEdit:
		return row.Exists && row.ApplicationID == ctx.Application.ID
	default:
		return false
	}
}

func full(ctx TrustedContext) bool {
	return ctx.Actor.BootstrapAdmin || ctx.Actor.ID == ctx.Application.OwnerID
}

func effectiveGroup(ctx TrustedContext, group PermissionGroup) bool {
	return group.Enabled && nonblank(group.ID) && group.ApplicationID == ctx.Application.ID &&
		contains(group.MemberIDs, ctx.Actor.ID)
}

func matchingTuple(ctx TrustedContext, resource Resource, action Action, row RowFact, grant Grant) bool {
	if grant.Resource != resource.Ref || grant.Action != action {
		return false
	}
	if action == MenuEnter {
		return grant.Rows == AllRows && len(grant.Fields) == 0
	}
	switch grant.Rows {
	case AllRows:
		return true
	case OwnRows:
		return row.CreatedBy == ctx.Actor.ID
	default:
		return false
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func nonblank(value string) bool { return strings.TrimSpace(value) != "" }
