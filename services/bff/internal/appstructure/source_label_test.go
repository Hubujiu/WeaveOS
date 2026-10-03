package appstructure

import (
	"context"
	"strings"
	"testing"
)

// Independent oracle: accepted FR002/Q13, auth DDL and HTTP max254 Unicode.
func TestReferenceSourcePreserves254CharacterAuthoritativeAccount(t *testing.T) {
	f := setup(t)
	c := context.Background()
	account := strings.Repeat("测", 254)
	id := uuid(t, f.owner)
	if _, e := f.owner.Exec(c, "INSERT INTO auth.users(id,account) VALUES($1,$2)", id, account); e != nil {
		t.Fatal("source hook narrowed existing254-character account contract", e)
	}
	var label, status string
	if e := f.runtime.QueryRow(c, "SELECT label,status FROM applications.member_sources WHERE id=$1", id).Scan(&label, &status); e != nil || label != account || status != "active" {
		t.Fatal("long authoritative label was truncated", len([]rune(label)), status, e)
	}
	if _, e := f.owner.Exec(c, "DELETE FROM auth.users WHERE id=$1", id); e != nil {
		t.Fatal(e)
	}
	if e := f.runtime.QueryRow(c, "SELECT label,status FROM applications.member_sources WHERE id=$1", id).Scan(&label, &status); e != nil || label != account || status != "deleted" {
		t.Fatal("long source tombstone lost label", len([]rune(label)), status, e)
	}
}
