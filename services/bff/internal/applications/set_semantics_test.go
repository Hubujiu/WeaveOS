package applications

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// These fixtures describe frozen B5a authorization facts independently of the
// set SQL. Groups with membership and grants separated must never fuse.
func seedPolicyGroup(t *testing.T, f *webFixture, app, member string, enabled, grant bool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := f.owner.QueryRow(ctx, "INSERT INTO applications.permission_groups(app_id,name,enabled) VALUES($1,'set-policy',$2) RETURNING id::text", app, enabled).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if member != "" {
		if _, err := f.owner.Exec(ctx, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", app, id, member); err != nil {
			t.Fatal(err)
		}
	}
	if grant {
		if _, err := f.owner.Exec(ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'application',$1,'menu.enter','all')", app, id); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func containsApp(items []App, id string) bool {
	return slices.ContainsFunc(items, func(a App) bool { return a.ID == id })
}

func TestB5SetListUsesCompleteCurrentAuthorizationFacts(t *testing.T) {
	owner := fixture(t, false)
	member := fixture(t, false)
	root := fixture(t, true)
	ctx := context.Background()
	ids := seedOwnedApps(t, owner, 10)
	seedPolicyGroup(t, owner, ids[0], member.actor, true, true)
	seedPolicyGroup(t, owner, ids[1], member.actor, true, false)
	seedPolicyGroup(t, owner, ids[2], "", true, true)
	seedPolicyGroup(t, owner, ids[3], member.actor, false, true)
	seedPolicyGroup(t, owner, ids[4], member.actor, true, false)
	seedPolicyGroup(t, owner, ids[4], "", true, true)
	seedPolicyGroup(t, owner, ids[5], member.actor, true, true)
	if _, err := owner.owner.Exec(ctx, "UPDATE personnel.permission_catalog SET enabled=false WHERE app_id=$1", ids[5]); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.owner.Exec(ctx, "DELETE FROM applications.menu_resources WHERE app_id=$1", ids[6]); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.owner.Exec(ctx, "DELETE FROM personnel.permission_catalog WHERE app_id=$1", ids[7]); err != nil {
		t.Fatal(err)
	}
	seedPolicyGroup(t, owner, ids[8], member.actor, true, true)
	seedPolicyGroup(t, owner, ids[8], member.actor, true, true)
	// A central identity permission by itself is not application membership.
	identity := createPermission(t, member, false)
	if _, err := owner.owner.Exec(ctx, "INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'app.'||$2::text||'.access')", identity, ids[9]); err != nil {
		t.Fatal(err)
	}
	p := principal(t, member)
	p.BootstrapAdmin = true // Only the database may establish Bootstrap status.
	items, err := member.service.Application.List(ctx, p)
	if err != nil || len(items) != 2 || items[0].ID != ids[0] || items[1].ID != ids[8] {
		t.Fatalf("only complete enabled same-group grants may list, once each: %v %v", items, err)
	}
	for _, f := range []*webFixture{owner, root} {
		items, err := f.service.Application.List(ctx, principal(t, f))
		if err != nil {
			t.Fatal(err)
		}
		for i, id := range ids {
			want := i != 5 && i != 6 && i != 7
			if containsApp(items, id) != want {
				t.Fatalf("owner/Bootstrap still requires actual menu and enabled catalogue: case %d", i)
			}
		}
	}
	if _, err := owner.owner.Exec(ctx, "DELETE FROM applications.group_members WHERE app_id=ANY($1::uuid[]) AND user_id=$2", []string{ids[0], ids[8]}, member.actor); err != nil {
		t.Fatal(err)
	}
	items, err = member.service.Application.List(ctx, p)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatal("next request must see revocation and return an empty array", err)
	}
	if _, err := owner.owner.Exec(ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", member.actor); err != nil {
		t.Fatal(err)
	}
	if _, err := member.service.Application.List(ctx, p); !errors.Is(err, session.ErrUnauthorized) {
		t.Fatal("stale actor version must fail before listing", err)
	}
}

func TestB5SetListRetainsOneRRRevisionAndPolicySnapshot(t *testing.T) {
	owner := fixture(t, false)
	member := fixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id := seedOwnedApps(t, owner, 1)[0]
	gid := seedPolicyGroup(t, owner, id, member.actor, true, true)
	// The actor SELECT has already established the RR snapshot when this
	// second data query pauses. A concurrent committed revoke must not tear it.
	g := &queryGate{prefix: "SELECT a.id::text", entered: make(chan struct{}), release: make(chan struct{})}
	a := &Application{Pool: tracedPool(t, g)}
	p := principal(t, member)
	type outcome struct {
		items []App
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		items, err := a.List(ctx, p)
		done <- outcome{items, err}
	}()
	waitGate(t, g)
	defer func() {
		select {
		case <-g.release:
		default:
			close(g.release)
		}
	}()
	tx, err := owner.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1 AND id=$2", id, gid); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "UPDATE applications.apps SET policy_revision=policy_revision+1 WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	close(g.release)
	select {
	case got := <-done:
		if got.err != nil || len(got.items) != 1 || got.items[0].ID != id || got.items[0].PolicyRevision != 1 {
			t.Fatalf("in-flight RR must keep authorized app and revision together: %v %v", got.items, got.err)
		}
	case <-ctx.Done():
		t.Fatal("RR query did not resume")
	}
	items, err := a.List(ctx, p)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatal("next RR sees disabled group without relogin", err)
	}
}

func TestB5SetMemberReplacementRejectsMissingUserAtomically(t *testing.T) {
	f := fixture(t, false)
	ctx := context.Background()
	id := seedOwnedApps(t, f, 1)[0]
	gid := seedPolicyGroup(t, f, id, f.actor, true, false)
	ids := seedMembers(t, f, 1000)
	ids = append(ids, "ffffffff-ffff-ffff-ffff-ffffffffffff")
	op := f.operation(t)
	_, err := f.service.Application.Write(ctx, principal(t, f), "members.replace", id, gid, Input{OperationID: op, ExpectedPolicyRevision: 1, MemberIDs: ids}, Metadata{RequestID: "invalid-member-set"})
	if !errors.Is(err, ErrResourceInvalid) {
		t.Fatal("one missing user must reject whole batch", err)
	}
	var rev, members, operations, audits int
	err = f.owner.QueryRow(ctx, `SELECT policy_revision,
 (SELECT count(*) FROM applications.group_members WHERE app_id=$1 AND group_id=$2 AND user_id=$3),
 (SELECT count(*) FROM applications.operations WHERE actor_user_id=$3 AND operation_id=$4),
 (SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$4::text)
 FROM applications.apps WHERE id=$1`, id, gid, f.actor, op).Scan(&rev, &members, &operations, &audits)
	if err != nil || rev != 1 || members != 1 || operations != 0 || audits != 0 {
		t.Fatal("missing-user validation must preserve old members/revision and roll back operation/audit", err)
	}
}

func TestB5GrantSetCostNormalizationAndAtomicReplacement(t *testing.T) {
	f := fixture(t, false)
	ctx := context.Background()
	id := seedOwnedApps(t, f, 1)[0]
	gid := seedPolicyGroup(t, f, id, "", true, false)
	a, c := countedApplication(t, f)
	p := principal(t, f)
	rev := int64(1)
	grant := Grant{ResourceKind: "application", ResourceID: id, Action: "menu.enter", RowScope: "all", Fields: []string{}}
	// B5a has one actual workspace resource per app: 1000 submitted copies
	// must normalize to one grant, not invent a larger resource registry.
	for _, n := range []int{1000, 1, 0} {
		grants := make([]Grant, n)
		for i := range grants {
			grants[i] = grant
		}
		c.reset()
		start := time.Now()
		_, err := a.Write(ctx, p, "grants.replace", id, gid, Input{OperationID: f.operation(t), ExpectedPolicyRevision: rev, Grants: grants}, Metadata{RequestID: "grant-set-cost"})
		elapsed := float64(time.Since(start).Microseconds()) / 1000
		c.active.Store(false)
		logCost(t, "grants.replace.submitted", n, []int64{c.queries.Load()}, []float64{elapsed})
		if err != nil || c.queries.Load() != 20 || c.menuValidation.Load() != 1 {
			t.Fatal("normalized full grant replacement must use 20 statements and one tuple validation", err)
		}
		rev++
		var stored int
		if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.grants WHERE app_id=$1 AND group_id=$2", id, gid).Scan(&stored); err != nil || (n == 0 && stored != 0) || (n > 0 && stored != 1) {
			t.Fatal("duplicate grants normalize to one; empty replacement clears", err)
		}
		if n == 1000 {
			op := f.operation(t)
			invalid := []Grant{grant, {ResourceKind: "application", ResourceID: "ffffffff-ffff-ffff-ffff-ffffffffffff", Action: "menu.enter", RowScope: "all", Fields: []string{}}}
			_, err := a.Write(ctx, p, "grants.replace", id, gid, Input{OperationID: op, ExpectedPolicyRevision: rev, Grants: invalid}, Metadata{RequestID: "invalid-grant-set"})
			if !errors.Is(err, ErrResourceInvalid) {
				t.Fatal("unknown tuple must reject", err)
			}
			var policyRevision int64
			var grants, operations, audits int
			err = f.owner.QueryRow(ctx, `SELECT policy_revision,
 (SELECT count(*) FROM applications.grants WHERE app_id=$1 AND group_id=$2),
 (SELECT count(*) FROM applications.operations WHERE actor_user_id=$3 AND operation_id=$4),
 (SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$4::text)
 FROM applications.apps WHERE id=$1`, id, gid, f.actor, op).Scan(&policyRevision, &grants, &operations, &audits)
			if err != nil || policyRevision != rev || grants != 1 || operations != 0 || audits != 0 {
				t.Fatal("invalid set must preserve existing grant and roll back revision/operation/audit", err)
			}
		}
	}
}
