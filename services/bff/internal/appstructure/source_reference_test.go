package appstructure

import (
	"context"
	"testing"
)

func TestActiveAuthoritativeReferenceDefaultsAndInactiveRejection(t *testing.T) {
	f := setup(t)
	view, _ := newForm(t, f)
	f.service.Application.References = CurrentSources{}
	member := field(t, f, "member", f.actor, map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, member)), 200)
	var disabled string
	e := f.owner.QueryRow(context.Background(), "INSERT INTO auth.users(account,status) VALUES('inactive-source-'||gen_random_uuid(),'disabled') RETURNING id::text").Scan(&disabled)
	if e != nil {
		t.Fatal(e)
	}
	member["default"] = disabled
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, member), 400, "APPLICATION_RESOURCE_INVALID")
	member["default"] = f.actor
	department := field(t, f, "department", uuid(t, f.owner), map[string]any{})
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, member, department), 400, "APPLICATION_RESOURCE_INVALID")
}

func TestUnauthorizedDefinitionWriteCannotInspectReferenceAvailability(t *testing.T) {
	f, owner := setup(t), setup(t)
	view, _ := newForm(t, owner)
	untrusted := *f
	untrusted.app = owner.app
	member := field(t, f, "member", uuid(t, f.owner), map[string]any{})
	for _, source := range []ReferenceValidator{nil, CurrentSources{}} {
		f.service.Application.References = source
		expectError(t, &untrusted, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, member), 403, "APPLICATION_FORBIDDEN")
	}
}
