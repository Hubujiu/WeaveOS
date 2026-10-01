package personnel

import (
	"context"
	"github.com/jackc/pgx/v5"
	"testing"
)

func queryRevisionsForTest(t *testing.T, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) QueryRevisions {
	t.Helper()
	var r QueryRevisions
	if err := db.QueryRow(context.Background(), `SELECT max(revision) FILTER(WHERE scope='people'),max(revision) FILTER(WHERE scope='configuration'),max(revision) FILTER(WHERE scope='activity') FROM personnel.query_revisions`).Scan(&r.People, &r.Configuration, &r.Activity); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestQ36RevisionActualWriterCoverageAndNoops(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	target := newTarget(t, f)
	check := func(name, scope, sql string, args ...any) {
		t.Helper()
		before := queryRevisionsForTest(t, f.owner)
		if _, err := f.owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		after := queryRevisionsForTest(t, f.owner)
		expected := map[string]bool{"people": scope == "people", "configuration": scope == "configuration", "activity": scope == "activity"}
		changed := map[string]bool{"people": after.People > before.People, "configuration": after.Configuration > before.Configuration, "activity": after.Activity > before.Activity}
		for k, want := range expected {
			if changed[k] != want {
				t.Errorf("%s: %s revision changed=%v want %v; before=%+v after=%+v", name, k, changed[k], want, before, after)
			}
		}
	}
	check("visible account", "people", "UPDATE auth.users SET account=account||'-renamed' WHERE id=$1", target)
	check("account no-op", "", "UPDATE auth.users SET account=account WHERE id=$1", target)
	check("auth version invisible", "", "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", target)
	check("visible status", "people", "UPDATE auth.users SET status='disabled' WHERE id=$1", target)
	check("configuration initialization zero", "", "INSERT INTO personnel.member_configuration(user_id) VALUES($1) ON CONFLICT DO NOTHING", target)
	check("configuration actual version", "people", "UPDATE personnel.member_configuration SET version=version+1 WHERE user_id=$1", target)
	check("identity name", "configuration", "UPDATE personnel.identities SET name=name||'-renamed' WHERE id=$1", f.i2)
	check("identity version only", "", "UPDATE personnel.identities SET version=version+1 WHERE id=$1", f.i2)
	check("identity no-op", "", "UPDATE personnel.identities SET name=name WHERE id=$1", f.i2)
	check("template description", "configuration", "UPDATE personnel.permission_templates SET description='new' WHERE id=$1", f.template)
	check("membership insert", "people", "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", target, f.i2)
	check("membership no-op", "", "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2) ON CONFLICT DO NOTHING", target, f.i2)
	check("membership delete", "people", "DELETE FROM personnel.member_identities WHERE user_id=$1", target)
	check("catalog visible name", "configuration", "UPDATE personnel.permission_catalog SET name=name||'-changed' WHERE code='app.test.A'")
	check("catalog enabled", "configuration", "UPDATE personnel.permission_catalog SET enabled=false WHERE code='app.test.A'")
	check("invitation event", "activity", `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,request_id) VALUES('invitation_created','success',$1,'q36-revision-invitation')`, f.actor.UserID)
	check("audit private metadata", "", "UPDATE auth.authentication_events SET user_agent='synthetic' WHERE request_id='q36-revision-invitation'")
	check("login excluded", "", `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,request_id) VALUES('login','success',$1,'q36-revision-login')`, f.actor.UserID)
	check("hot archive delete", "activity", "DELETE FROM auth.authentication_events WHERE request_id='q36-revision-invitation'")
	check("login delete excluded", "", "DELETE FROM auth.authentication_events WHERE request_id='q36-revision-login'")
	before := queryRevisionsForTest(t, f.owner)
	id := newTarget(t, f)
	after := queryRevisionsForTest(t, f.owner)
	if after.People <= before.People {
		t.Error("new registration/user must advance people revision")
	}
	check("delete visible user", "people", "DELETE FROM auth.users WHERE id=$1", id)
	before = queryRevisionsForTest(t, f.owner)
	d, err := f.app.CreateDraft(ctx, f.actor, DraftCreateInput{Kind: DraftIdentity, Payload: []byte(`{"name":"","description":"","templateIds":[],"permissionCodes":[]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.app.DeleteDraft(ctx, f.actor, d.ID, d.Version); err != nil {
		t.Fatal(err)
	}
	if queryRevisionsForTest(t, f.owner) != before {
		t.Error("draft CRUD or initial auth configuration changed business revisions")
	}
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before = queryRevisionsForTest(t, f.owner)
	if _, err = tx.Exec(ctx, "UPDATE auth.users SET account=account||'-rolledback' WHERE id=$1", target); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if queryRevisionsForTest(t, f.owner) != before {
		t.Error("rolled-back mutation changed revision")
	}
}
