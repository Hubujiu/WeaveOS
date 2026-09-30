package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func rootDepartment(t *testing.T, f *fixture) string {
	t.Helper()
	var id string
	if err := f.owner.QueryRow(context.Background(), "SELECT id::text FROM personnel.departments WHERE is_root").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func cleanDepartment(t *testing.T, f *fixture, id string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), "DELETE FROM personnel.department_members WHERE department_id=$1", id)
		_, _ = f.owner.Exec(context.Background(), "DELETE FROM personnel.departments WHERE id=$1", id)
	})
}
func newTarget(t *testing.T, f *fixture) string {
	t.Helper()
	var id string
	if err := f.owner.QueryRow(context.Background(), "INSERT INTO auth.users(account) VALUES('target-'||gen_random_uuid()) RETURNING id::text").Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.department_members WHERE user_id=$1", id)
		_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.member_identities WHERE user_id=$1", id)
		_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.member_configuration WHERE user_id=$1", id)
		_, _ = f.owner.Exec(ctx, "DELETE FROM auth.users WHERE id=$1", id)
	})
	return id
}
func TestDepartmentHierarchyVersionAndProtectedDeleteQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	root := rootDepartment(t, f)
	meta := RequestMetadata{RequestID: "department-test"}
	child, err := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "研发", ParentID: root}, meta)
	if err != nil {
		t.Fatalf("create department: %v", err)
	}
	cleanDepartment(t, f, child.ID)
	if child.ParentID == nil || *child.ParentID != root || child.Version != 1 || child.IsRoot {
		t.Fatal("explicit parent and nonroot object required")
	}
	renamed, err := f.app.SaveDepartment(ctx, f.actor, child.ID, DepartmentInput{Name: "研发二", Version: 1}, meta)
	if err != nil || renamed.Version != 2 {
		t.Fatalf("versioned rename: %v", err)
	}
	grand, err := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "子部门", ParentID: child.ID}, meta)
	if err != nil {
		t.Fatal(err)
	}
	cleanDepartment(t, f, grand.ID)
	if err := f.app.DeleteDepartment(ctx, f.actor, root, 1, meta); !errors.Is(err, ErrConflict) {
		t.Fatal("enterprise root cannot be deleted")
	}
	if err := f.app.DeleteDepartment(ctx, f.actor, child.ID, 2, meta); !errors.Is(err, ErrConflict) {
		t.Fatal("nonempty department cannot be deleted")
	}
	if err := f.app.DeleteDepartment(ctx, f.actor, grand.ID, 1, meta); err != nil {
		t.Fatal(err)
	}
	if err := f.app.DeleteDepartment(ctx, f.actor, child.ID, 1, meta); !errors.Is(err, ErrConflict) {
		t.Fatal("stale deletion must conflict")
	}
	if err := f.app.DeleteDepartment(ctx, f.actor, child.ID, 2, meta); err != nil {
		t.Fatal(err)
	}
}
func TestMemberExplicitMoveMultiIdentityAndIdempotencyQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	target := newTarget(t, f)
	meta := RequestMetadata{RequestID: "member-move-test"}
	original, err := f.app.GetMember(ctx, f.actor, target)
	if err != nil {
		t.Fatalf("member detail: %v", err)
	}
	if original.Version != 0 || len(original.Identities) != 0 || len(original.Permissions) != 0 {
		t.Fatal("unassigned member starts version 0 without permissions")
	}
	assigned, err := f.app.SetMemberIdentities(ctx, f.actor, target, []string{f.i1, f.i2}, 0, meta)
	if err != nil || assigned.Version != 1 || len(assigned.IdentityIDs) != 2 || len(assigned.Permissions) != 3 {
		t.Fatalf("explicit multi-identity set: %v", err)
	}
	root := rootDepartment(t, f)
	departments := []string{}
	for _, name := range []string{"D1", "D2", "D3"} {
		d, err := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: name, ParentID: root}, meta)
		if err != nil {
			t.Fatal(err)
		}
		departments = append(departments, d.ID)
		cleanDepartment(t, f, d.ID)
	}
	for i, department := range []string{departments[0], departments[2]} {
		if _, err := f.app.ChangeMemberGroups(ctx, f.actor, target, GroupInput{Operation: "add", DepartmentID: department, Version: int64(i + 1)}, meta); err != nil {
			t.Fatal(err)
		}
	}
	moved, err := f.app.ChangeMemberGroups(ctx, f.actor, target, GroupInput{Operation: "move", SourceDepartmentID: departments[0], DepartmentID: departments[1], Version: 3}, meta)
	if err != nil || moved.Version != 4 || len(moved.DepartmentIDs) != 2 || len(moved.IdentityIDs) != 2 || len(moved.Permissions) != 3 {
		t.Fatalf("move must preserve D3 and all authorization: %v", err)
	}
	set := map[string]bool{}
	for _, id := range moved.DepartmentIDs {
		set[id] = true
	}
	if set[departments[0]] || !set[departments[1]] || !set[departments[2]] {
		t.Fatal("only explicit source removed, target added")
	}
	var before int
	if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events WHERE object_type='member' AND object_id=$1", target).Scan(&before); err != nil {
		t.Fatal(err)
	}
	repeat, err := f.app.ChangeMemberGroups(ctx, f.actor, target, GroupInput{Operation: "move", SourceDepartmentID: departments[0], DepartmentID: departments[1], Version: 4}, meta)
	if err != nil || repeat.Version != 4 {
		t.Fatal("repeated already-applied move must not increment version")
	}
	var after int
	_ = f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events WHERE object_type='member' AND object_id=$1", target).Scan(&after)
	if after != before {
		t.Fatal("idempotent relation request must not create change audit")
	}
	list, err := f.app.ListMembers(ctx, f.actor, MemberQuery{PageQuery: PageQuery{Page: 1, PageSize: 20, Search: "target-"}, DepartmentID: departments[1], IdentityID: f.i1})
	if err != nil || list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("server filters and actual total required: %v", err)
	}
	if _, err := f.app.SetMemberIdentities(ctx, f.actor, target, []string{}, 3, meta); !errors.Is(err, ErrConflict) {
		t.Fatal("stale member configuration must conflict")
	}
}
func TestMemberAuditFailureRollsBackRelationAndVersionQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	target := newTarget(t, f)
	_, err := f.app.SetMemberIdentities(ctx, f.actor, target, []string{f.i1}, 0, RequestMetadata{})
	if err == nil || errors.Is(err, ErrNotImplemented) {
		t.Fatalf("must reach actual audit failure: %v", err)
	}
	var relations, versionRows int
	_ = f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.member_identities WHERE user_id=$1", target).Scan(&relations)
	_ = f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.member_configuration WHERE user_id=$1", target).Scan(&versionRows)
	if relations != 0 || versionRows != 0 {
		t.Fatal("audit failure rolls back assignment and first version row")
	}
}
func TestEventsSafeFieldsDefaultWindowAndServerFiltersQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// Independent audit fixture reaches the event reader without relying on a new writer.
	var objectID string
	if err := f.owner.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&objectID); err != nil {
		t.Fatal(err)
	}
	_, err := f.owner.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary) VALUES('personnel_changed','success',$1,'DEPARTMENT_CREATED','activity-test','department',$2,'{"before":null,"after":{"name":"安全活动对象"}}')`, f.actor.UserID, objectID)
	if err != nil {
		t.Fatal(err)
	}
	events, err := f.app.Events(ctx, f.actor, EventQuery{PageQuery: PageQuery{Search: "安全活动对象"}, Action: "DEPARTMENT_CREATED"})
	if err != nil || events.Total != 1 || len(events.Items) != 1 {
		t.Fatalf("controlled safe activity filter: %v", err)
	}
	b, _ := json.Marshal(events.Items[0])
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	for _, secret := range []string{"sessionRef", "session_ref", "clientIp", "userAgent", "password", "invitationCode"} {
		if raw[secret] != nil {
			t.Fatalf("activity may not expose %s", secret)
		}
	}
	if _, err := f.owner.Exec(ctx, "UPDATE auth.authentication_events SET occurred_at=now()-interval '8 days' WHERE object_id=$1", objectID); err != nil {
		t.Fatal(err)
	}
	recent, err := f.app.Events(ctx, f.actor, EventQuery{PageQuery: PageQuery{Search: "安全活动对象"}})
	if err != nil || recent.Total != 0 {
		t.Fatal("default window is last seven days hot storage")
	}
	history, err := f.app.Events(ctx, f.actor, EventQuery{PageQuery: PageQuery{Search: "安全活动对象"}, From: time.Now().AddDate(0, 0, -10), To: time.Now()})
	if err != nil || history.Total != 1 {
		t.Fatal("explicit valid hot time window")
	}
}
func TestPersonnelReadAndWriteDenyAfterRevokeQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, err := f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1 AND permission_code='personnel.manage'", f.template)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.Departments(ctx, f.actor); !errors.Is(err, ErrDenied) {
		t.Fatalf("department read qualification: %v", err)
	}
	if _, err := f.app.Catalog(ctx, f.actor); !errors.Is(err, ErrDenied) {
		t.Fatal("permission directory must require manage")
	}
	if _, err := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "denied", ParentID: rootDepartment(t, f)}, RequestMetadata{RequestID: "denied-test"}); !errors.Is(err, ErrDenied) {
		t.Fatal("write must recheck current qualification")
	}
}
