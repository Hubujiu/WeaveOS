package applications

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appmeta"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxRevision = int64(9007199254740991)

func grantResourceIDs(grants []Grant) []string {
	out := []string{}
	for _, g := range grants {
		out = append(out, g.ResourceID)
	}
	return out
}

func canonicalID(id string) (string, bool) {
	var u pgtype.UUID
	if u.Scan(id) != nil || !u.Valid || u.String() != strings.ToLower(id) {
		return "", false
	}
	return u.String(), true
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}
func normalized(kind string, in Input) (Input, error) {
	var ok bool
	in.OperationID, ok = canonicalID(in.OperationID)
	if !ok {
		return in, ErrInvalid
	}
	if kind != "application.create" && (in.ExpectedPolicyRevision < 1 || in.ExpectedPolicyRevision > maxRevision) {
		return in, ErrInvalid
	}
	if kind == "application.create" || kind == "group.create" || kind == "group.update" {
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" || strings.ContainsRune(in.Name, 0) || !utf8.ValidString(in.Name) || utf8.RuneCountInString(in.Name) > 100 {
			return in, ErrInvalid
		}
	}
	if kind == "members.replace" {
		if in.MemberIDs == nil {
			return in, ErrInvalid
		}
		seen := map[string]bool{}
		ids := make([]string, 0, len(in.MemberIDs))
		for _, id := range in.MemberIDs {
			v, ok := canonicalID(id)
			if !ok {
				return in, ErrInvalid
			}
			if !seen[v] {
				ids = append(ids, v)
				seen[v] = true
			}
		}
		sort.Strings(ids)
		in.MemberIDs = ids
	}
	if kind == "grants.replace" {
		if in.Grants == nil {
			return in, ErrInvalid
		}
		seen := map[string]bool{}
		grants := make([]Grant, 0, len(in.Grants))
		for _, g := range in.Grants {
			v, ok := canonicalID(g.ResourceID)
			menu := g.Action == "menu.enter" && (g.ResourceKind == "application" || g.ResourceKind == "directory" || g.ResourceKind == "form") && g.RowScope == "all"
			data := g.ResourceKind == "form" && ((g.Action == "data.create" && g.RowScope == "all") || ((g.Action == "data.read" || g.Action == "data.edit" || g.Action == "data.history") && (g.RowScope == "all" || g.RowScope == "own")))
			if !ok || !menu && !data || g.Fields == nil || menu && len(g.Fields) != 0 {
				return in, ErrResourceInvalid
			}
			g.ResourceID = v
			fields := map[string]bool{}
			for _, id := range g.Fields {
				canonical, ok := canonicalID(id)
				if !ok {
					return in, ErrResourceInvalid
				}
				fields[canonical] = true
			}
			g.Fields = []string{}
			for id := range fields {
				g.Fields = append(g.Fields, id)
			}
			sort.Strings(g.Fields)
			key := g.ResourceKind + ":" + v + ":" + g.Action + ":" + g.RowScope
			if !seen[key] {
				grants = append(grants, g)
				seen[key] = true
			} else {
				for i := range grants {
					old := &grants[i]
					if old.ResourceKind == g.ResourceKind && old.ResourceID == g.ResourceID && old.Action == g.Action && old.RowScope == g.RowScope {
						for _, id := range old.Fields {
							fields[id] = true
						}
						old.Fields = []string{}
						for id := range fields {
							old.Fields = append(old.Fields, id)
						}
						sort.Strings(old.Fields)
						break
					}
				}
			}
		}
		sort.Slice(grants, func(i, j int) bool {
			a, b := grants[i], grants[j]
			return a.ResourceKind+":"+a.ResourceID+":"+a.Action+":"+a.RowScope < b.ResourceKind+":"+b.ResourceID+":"+b.Action+":"+b.RowScope
		})
		in.Grants = grants
	}
	return in, nil
}
func classify(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "40001", "40P01", "23503", "23505", "23514":
			return ErrPolicyConflict
		}
	}
	return err
}

// A SQLSTATE describes a server error, not proof that COMMIT rolled back.
// Preserve its cause internally while the HTTP boundary exposes only the
// unconfirmed operation contract. Never retry an ambiguous commit here.
func commitWrite(ctx context.Context, tx pgx.Tx) error {
	err := tx.Commit(ctx)
	if err == nil || errors.Is(err, pgx.ErrTxCommitRollback) {
		return err
	}
	return errors.Join(ErrUnconfirmed, err)
}

// Every read stays in one short RR snapshot; none of the metadata or policy
// reads escape to the pool. Central grants are not a second app-entry gate.
func (a *Application) read(ctx context.Context, p session.Principal) (pgx.Tx, apppolicy.TrustedActor, error) {
	if a == nil || a.Pool == nil {
		return nil, apppolicy.TrustedActor{}, session.ErrUnavailable
	}
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, apppolicy.TrustedActor{}, err
	}
	var root bool
	err = tx.QueryRow(ctx, "SELECT is_bootstrap_admin FROM auth.users WHERE id=$1 AND status='active' AND auth_version::text=$2", p.UserID, p.Record.AuthVersion).Scan(&root)
	if err != nil {
		_ = tx.Rollback(context.Background())
		if errors.Is(err, pgx.ErrNoRows) {
			err = session.ErrUnauthorized
		}
		return nil, apppolicy.TrustedActor{}, err
	}
	return tx, apppolicy.TrustedActor{ID: p.UserID, BootstrapAdmin: root}, nil
}
func loadApp(ctx context.Context, tx pgx.Tx, id string, lock bool) (App, error) {
	q := "SELECT id::text,name,owner_user_id::text,policy_revision FROM applications.apps WHERE id=$1 AND deleted_at IS NULL"
	if lock {
		q += " FOR UPDATE"
	}
	var v App
	err := tx.QueryRow(ctx, q, id).Scan(&v.ID, &v.Name, &v.OwnerUserID, &v.PolicyRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrMissing
	}
	return v, err
}
func registered(ctx context.Context, tx pgx.Tx, app App) error {
	var yes bool
	err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='app.'||$1::text||'.access' AND app_id=$1::text AND category='application' AND enabled)", app.ID).Scan(&yes)
	if err != nil {
		return err
	}
	if !yes {
		return ErrDenied
	}
	return nil
}
func manager(actor apppolicy.TrustedActor, app App) bool {
	return actor.BootstrapAdmin || actor.ID == app.OwnerUserID
}
func structure(ctx context.Context, tx pgx.Tx, app App) (appmeta.Structure, []Menu, error) {
	s := appmeta.Structure{Application: appmeta.Application{ID: appmeta.ID(app.ID), Name: app.Name}}
	if err := appmeta.ValidateStructure(s); err != nil {
		return s, nil, err
	}
	rows, err := tx.Query(ctx, "SELECT resource_kind,resource_id::text FROM applications.menu_resources WHERE app_id=$1 AND resource_kind='application' ORDER BY resource_kind,resource_id", app.ID)
	if err != nil {
		return s, nil, err
	}
	defer rows.Close()
	menus := []Menu{}
	for rows.Next() {
		var m Menu
		if err := rows.Scan(&m.ResourceKind, &m.ResourceID); err != nil {
			return s, nil, err
		}
		if m.ResourceKind != "application" || m.ResourceID != app.ID {
			return s, nil, ErrResourceInvalid
		}
		menus = append(menus, m)
	}
	return s, menus, rows.Err()
}
func snapshot(ctx context.Context, tx pgx.Tx, actor apppolicy.TrustedActor, app App) (apppolicy.TrustedContext, []Menu, error) {
	policy := apppolicy.TrustedContext{Actor: actor, Application: apppolicy.Application{ID: app.ID, OwnerID: app.OwnerUserID, Exists: true}, Groups: []apppolicy.PermissionGroup{}}
	if err := registered(ctx, tx, app); err != nil {
		return policy, nil, err
	}
	_, menus, err := structure(ctx, tx, app)
	if err != nil {
		return policy, nil, err
	}
	rows, err := tx.Query(ctx, `SELECT g.id::text,g.enabled,r.resource_kind,r.resource_id::text,r.action,r.row_scope
 FROM applications.permission_groups g JOIN applications.group_members m ON m.app_id=g.app_id AND m.group_id=g.id AND m.user_id=$2
 LEFT JOIN applications.grants r ON r.app_id=g.app_id AND r.group_id=g.id
 WHERE g.app_id=$1 ORDER BY g.id,r.id`, app.ID, actor.ID)
	if err != nil {
		return policy, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var enabled bool
		var kind, resource, action, scope *string
		if err := rows.Scan(&id, &enabled, &kind, &resource, &action, &scope); err != nil {
			return policy, nil, err
		}
		g := apppolicy.PermissionGroup{ID: id, ApplicationID: app.ID, Enabled: enabled, MemberIDs: []string{actor.ID}, Grants: []apppolicy.Grant{}}
		if kind != nil && resource != nil && action != nil && scope != nil {
			g.Grants = append(g.Grants, apppolicy.Grant{Resource: apppolicy.ResourceRef{ApplicationID: app.ID, Kind: *kind, ID: *resource}, Action: apppolicy.Action(*action), Rows: apppolicy.RowScope(*scope), Fields: []string{}})
		}
		policy.Groups = append(policy.Groups, g)
	}
	return policy, menus, rows.Err()
}
func entry(ctx context.Context, tx pgx.Tx, actor apppolicy.TrustedActor, app App) (Access, error) {
	policy, menus, err := snapshot(ctx, tx, actor, app)
	v := Access{AppID: app.ID, PolicyRevision: app.PolicyRevision, Menus: []Menu{}}
	if err != nil {
		return v, err
	}
	for _, m := range menus {
		if apppolicy.Allows(policy, apppolicy.Resource{Ref: apppolicy.ResourceRef{ApplicationID: app.ID, Kind: m.ResourceKind, ID: m.ResourceID}, Exists: true}, apppolicy.MenuEnter, apppolicy.RowFact{}, "") {
			v.Menus = append(v.Menus, m)
		}
	}
	v.CanEnter = len(v.Menus) > 0
	return v, nil
}
func (a *Application) List(ctx context.Context, p session.Principal) ([]App, error) {
	tx, actor, err := a.read(ctx, p)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	rows, err := tx.Query(ctx, `SELECT a.id::text,a.name,a.owner_user_id::text,a.policy_revision
 FROM applications.apps a
 JOIN applications.menu_resources mr ON mr.app_id=a.id AND mr.resource_kind='application' AND mr.resource_id=a.id
 JOIN personnel.permission_catalog c ON c.code='app.'||a.id::text||'.access' AND c.app_id=a.id::text AND c.category='application' AND c.enabled
 WHERE $2::boolean OR a.owner_user_id=$1::uuid OR EXISTS (
   SELECT 1 FROM applications.group_members m
   JOIN applications.permission_groups g ON g.app_id=m.app_id AND g.id=m.group_id AND g.enabled
   JOIN applications.grants r ON r.app_id=m.app_id AND r.group_id=m.group_id
     AND r.resource_kind=mr.resource_kind AND r.resource_id=mr.resource_id
     AND r.action='menu.enter' AND r.row_scope='all'
   WHERE m.app_id=a.id AND m.user_id=$1::uuid
     AND NOT EXISTS (SELECT 1 FROM applications.grant_fields f WHERE f.app_id=r.app_id AND f.grant_id=r.id)
 ) ORDER BY a.created_at,a.id`, actor.ID, actor.BootstrapAdmin)
	if err != nil {
		return nil, err
	}
	apps := []App{}
	for rows.Next() {
		var v App
		if err := rows.Scan(&v.ID, &v.Name, &v.OwnerUserID, &v.PolicyRevision); err != nil {
			rows.Close()
			return nil, err
		}
		if err := appmeta.ValidateStructure(appmeta.Structure{Application: appmeta.Application{ID: appmeta.ID(v.ID), Name: v.Name}}); err != nil {
			rows.Close()
			return nil, err
		}
		apps = append(apps, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return apps, tx.Commit(ctx)
}
func (a *Application) Get(ctx context.Context, p session.Principal, id string) (App, Access, error) {
	tx, actor, err := a.read(ctx, p)
	if err != nil {
		return App{}, Access{}, err
	}
	defer tx.Rollback(context.Background())
	app, err := loadApp(ctx, tx, id, false)
	if err != nil {
		return app, Access{}, err
	}
	access, err := entry(ctx, tx, actor, app)
	if err == nil && !access.CanEnter {
		err = ErrDenied
	}
	if err != nil {
		return app, access, err
	}
	return app, access, tx.Commit(ctx)
}
func (a *Application) Configuration(ctx context.Context, p session.Principal, appID, groupID, part string) (any, error) {
	tx, actor, err := a.read(ctx, p)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	app, err := loadApp(ctx, tx, appID, false)
	if err != nil {
		return nil, err
	}
	if err := registered(ctx, tx, app); err != nil {
		return nil, err
	}
	if !manager(actor, app) {
		return nil, ErrDenied
	}
	if groupID == "" {
		rows, err := tx.Query(ctx, "SELECT id::text,name,enabled FROM applications.permission_groups WHERE app_id=$1 ORDER BY created_at,id", appID)
		if err != nil {
			return nil, err
		}
		items := []Group{}
		for rows.Next() {
			g := Group{PolicyRevision: app.PolicyRevision}
			if err := rows.Scan(&g.ID, &g.Name, &g.Enabled); err != nil {
				rows.Close()
				return nil, err
			}
			items = append(items, g)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items, "policyRevision": app.PolicyRevision}, tx.Commit(ctx)
	}
	if _, err := loadGroup(ctx, tx, appID, groupID); err != nil {
		return nil, err
	}
	if part == "members" {
		rows, err := tx.Query(ctx, "SELECT u.id::text,u.account,u.status FROM applications.group_members m JOIN auth.users u ON u.id=m.user_id WHERE m.app_id=$1 AND m.group_id=$2 ORDER BY u.id", appID, groupID)
		if err != nil {
			return nil, err
		}
		ids := []string{}
		members := []MemberDisplay{}
		for rows.Next() {
			var member MemberDisplay
			if err := rows.Scan(&member.ID, &member.Label, &member.Status); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, member.ID)
			member.Selectable = member.Status == "active"
			members = append(members, member)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		return map[string]any{"memberIds": ids, "members": members, "policyRevision": app.PolicyRevision}, tx.Commit(ctx)
	}
	rows, err := tx.Query(ctx, `SELECT resource_kind,resource_id::text,action,row_scope,
 ARRAY(SELECT field_id::text FROM applications.grant_fields f WHERE f.app_id=g.app_id AND f.grant_id=g.id ORDER BY field_id)
 FROM applications.grants g WHERE app_id=$1 AND group_id=$2 ORDER BY resource_kind,resource_id,action,row_scope`, appID, groupID)
	if err != nil {
		return nil, err
	}
	grants := []Grant{}
	for rows.Next() {
		g := Grant{Fields: []string{}}
		if err := rows.Scan(&g.ResourceKind, &g.ResourceID, &g.Action, &g.RowScope, &g.Fields); err != nil {
			rows.Close()
			return nil, err
		}
		grants = append(grants, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return map[string]any{"grants": grants, "policyRevision": app.PolicyRevision}, tx.Commit(ctx)
}
func loadGroup(ctx context.Context, tx pgx.Tx, appID, id string) (Group, error) {
	var g Group
	err := tx.QueryRow(ctx, "SELECT id::text,name,enabled FROM applications.permission_groups WHERE app_id=$1 AND id=$2", appID, id).Scan(&g.ID, &g.Name, &g.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrMissing
	}
	return g, err
}

func readableOperation(ctx context.Context, tx pgx.Tx, actor apppolicy.TrustedActor, appID, kind string) error {
	// operation() already constrains the lookup to this live actor. New record/
	// draft kinds store only minimum results, independently of current data grants.
	if recordOperation(kind) || structureDeletion(kind) || kind == TemplateImportKind {
		return nil
	}
	app, err := loadApp(ctx, tx, appID, false)
	if err != nil {
		return err
	}
	if err := registered(ctx, tx, app); err != nil {
		return err
	}
	if kind != "application.create" {
		if !manager(actor, app) {
			return ErrDenied
		}
		return nil
	}
	access, err := entry(ctx, tx, actor, app)
	if err != nil {
		return err
	}
	if !access.CanEnter {
		return ErrDenied
	}
	return nil
}
func operation(ctx context.Context, tx pgx.Tx, actor apppolicy.TrustedActor, id string) (Operation, string, string, []byte, error) {
	var op Operation
	var appID, kind string
	var fingerprint []byte
	op.OperationID = id
	op.Status = "confirmed"
	err := tx.QueryRow(ctx, "SELECT app_id::text,operation_kind,fingerprint,result_json,http_status,location FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", actor.ID, id).Scan(&appID, &kind, &fingerprint, &op.Result, &op.HTTPStatus, &op.Location)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrMissing
	}
	return op, appID, kind, fingerprint, err
}
func (a *Application) Operation(ctx context.Context, p session.Principal, id string) (Operation, error) {
	tx, actor, err := a.read(ctx, p)
	if err != nil {
		return Operation{}, err
	}
	defer tx.Rollback(context.Background())
	op, appID, kind, _, err := operation(ctx, tx, actor, id)
	if err != nil {
		return op, err
	}
	if err := readableOperation(ctx, tx, actor, appID, kind); err != nil {
		return op, err
	}
	return op, tx.Commit(ctx)
}

// Write takes the existing revision locks before any account/dependency lock.
// It executes business changes once; no ambiguous COMMIT is retried here.
func (a *Application) Write(ctx context.Context, p session.Principal, kind, appID, groupID string, in Input, meta Metadata) (Result, error) {
	in, err := normalized(kind, in)
	if err != nil {
		return Result{}, err
	}
	if a == nil || a.Pool == nil {
		return Result{}, session.ErrUnavailable
	}
	canonical, _ := json.Marshal(struct {
		Kind, AppID, GroupID string
		Input                Input
	}{kind, appID, groupID, in})
	fingerprint := sha256.Sum256(canonical)
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err != nil {
		return Result{}, classify(err)
	}
	access, err := (&personnel.Application{}).AccessForWrite(ctx, tx, p)
	if err != nil {
		return Result{}, err
	}
	actor := apppolicy.TrustedActor{ID: access.User.ID, BootstrapAdmin: access.BootstrapAdmin}
	old, oldApp, oldKind, oldHash, err := operation(ctx, tx, actor, in.OperationID)
	if err == nil {
		if !bytes.Equal(oldHash, fingerprint[:]) {
			return Result{}, ErrOperationConflict
		}
		if err := readableOperation(ctx, tx, actor, oldApp, oldKind); err != nil {
			return Result{}, err
		}
		if err := commitWrite(ctx, tx); err != nil {
			return Result{}, err
		}
		return Result{Status: old.HTTPStatus, Location: old.Location, Data: old.Result}, nil
	}
	if !errors.Is(err, ErrMissing) {
		return Result{}, err
	}
	// Source rows precede the application/policy gate. Defer source validation
	// until after the real app/group permission checks to preserve error order.
	memberStatuses := map[string]string{}
	if kind == "members.replace" {
		rows, e := tx.Query(ctx, "SELECT id::text,status FROM auth.users WHERE id=ANY($1::uuid[]) ORDER BY id FOR SHARE", in.MemberIDs)
		if e != nil {
			return Result{}, classify(e)
		}
		for rows.Next() {
			var id, status string
			if e = rows.Scan(&id, &status); e != nil {
				rows.Close()
				return Result{}, e
			}
			memberStatuses[id] = status
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return Result{}, e
		}
	}
	var app App
	if kind == "application.create" {
		for _, permission := range access.Permissions {
			if permission.Code == "applications.create" {
				actor.CreateApp = true
			}
		}
		if !apppolicy.CanCreate(actor) {
			return Result{}, ErrDenied
		}
		appID, err = newID()
		if err != nil {
			return Result{}, err
		}
		app = App{ID: appID, Name: in.Name, OwnerUserID: actor.ID, PolicyRevision: 1}
	} else {
		app, err = loadApp(ctx, tx, appID, true)
		if err != nil {
			return Result{}, err
		}
		if err := registered(ctx, tx, app); err != nil {
			return Result{}, err
		}
		if !manager(actor, app) {
			return Result{}, ErrDenied
		}
		if app.PolicyRevision != in.ExpectedPolicyRevision {
			return Result{}, ErrPolicyConflict
		}
		if kind != "group.create" {
			if _, err := loadGroup(ctx, tx, appID, groupID); err != nil {
				return Result{}, err
			}
		}
	}
	// A unique actor/key claim is the durable concurrent operation arbiter.
	var claimed string
	err = tx.QueryRow(ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint) VALUES($1,$2,$3,$4,$5) ON CONFLICT(actor_user_id,operation_id) DO NOTHING RETURNING operation_id::text`, actor.ID, in.OperationID, appID, kind, fingerprint[:]).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrOperationConflict
	}
	if err != nil {
		return Result{}, classify(err)
	}
	before := app.PolicyRevision
	status := 200
	location := ""
	reason := ""
	objectType := "permission_group"
	objectID := groupID
	counts := map[string]int64{}
	var result any
	switch kind {
	case "application.create":
		before = 0
		status = 201
		location = "/api/v1/applications/" + appID
		reason = "APPLICATION_CREATED"
		objectType = "application"
		objectID = appID
		counts["applications"] = 1
		if _, err = tx.Exec(ctx, "INSERT INTO applications.apps(id,name,owner_user_id) VALUES($1,$2,$3)", appID, in.Name, actor.ID); err != nil {
			break
		}
		if _, err = tx.Exec(ctx, "INSERT INTO applications.menu_resources(app_id,resource_kind,resource_id) VALUES($1,'application',$1)", appID); err != nil {
			break
		}
		_, err = tx.Exec(ctx, "SELECT applications.register_catalog_entry($1)", appID)
		result = app
	case "group.create":
		var g Group
		g.Name = in.Name
		g.Enabled = true
		err = tx.QueryRow(ctx, "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,$2) RETURNING id::text", appID, in.Name).Scan(&g.ID)
		groupID = g.ID
		objectID = g.ID
		reason = "GROUP_CREATED"
		counts["groups"] = 1
		status = 201
		location = "/api/v1/applications/" + appID + "/permission-groups/" + g.ID
		g.PolicyRevision = before + 1
		result = g
	case "group.update":
		_, err = tx.Exec(ctx, "UPDATE applications.permission_groups SET name=$3,enabled=$4,updated_at=now() WHERE app_id=$1 AND id=$2", appID, groupID, in.Name, in.Enabled)
		reason = "GROUP_UPDATED"
		counts["groups"] = 1
		result = Group{ID: groupID, Name: in.Name, Enabled: in.Enabled, PolicyRevision: before + 1}
	case "members.replace":
		if len(memberStatuses) != len(in.MemberIDs) {
			err = ErrResourceInvalid
		}
		inactive := []string{}
		for _, id := range in.MemberIDs {
			if memberStatuses[id] != "active" {
				inactive = append(inactive, id)
			}
		}
		if err == nil && len(inactive) > 0 {
			var existing int
			err = tx.QueryRow(ctx, "SELECT count(*) FROM applications.group_members WHERE app_id=$1 AND group_id=$2 AND user_id=ANY($3::uuid[])", appID, groupID, inactive).Scan(&existing)
			if err == nil && existing != len(inactive) {
				err = ErrResourceInvalid
			}
		}
		if err != nil {
			break
		}
		if _, err = tx.Exec(ctx, "DELETE FROM applications.group_members WHERE app_id=$1 AND group_id=$2", appID, groupID); err != nil {
			break
		}
		_, err = tx.Exec(ctx, "INSERT INTO applications.group_members(app_id,group_id,user_id) SELECT $1,$2,user_id FROM unnest($3::uuid[]) AS members(user_id)", appID, groupID, in.MemberIDs)
		reason = "GROUP_MEMBERS_REPLACED"
		counts["members"] = int64(len(in.MemberIDs))
		result = map[string]any{"id": groupID, "policyRevision": before + 1}
	case "grants.replace":
		// Parallel arrays come from each normalized complete tuple. UNNEST zips
		// those tuples; independent ANY predicates could combine privileges.
		kinds := make([]string, 0, len(in.Grants))
		ids := make([]string, 0, len(in.Grants))
		actions := make([]string, 0, len(in.Grants))
		scopes := make([]string, 0, len(in.Grants))
		for _, g := range in.Grants {
			kinds = append(kinds, g.ResourceKind)
			ids = append(ids, g.ResourceID)
			actions = append(actions, g.Action)
			scopes = append(scopes, g.RowScope)
		}
		grantJSON, _ := json.Marshal(in.Grants)
		var found int
		err = tx.QueryRow(ctx, `WITH locked_tables AS MATERIALIZED (
 SELECT id FROM applications.logical_tables WHERE app_id=$1 AND id IN (
 SELECT v.table_id FROM applications.form_views v WHERE v.app_id=$1 AND v.id IN (SELECT resource_id FROM applications.grants WHERE app_id=$1 AND group_id=$7)
 UNION SELECT v.table_id FROM applications.form_views v WHERE v.app_id=$1 AND v.id=ANY($3::uuid[])) ORDER BY id FOR UPDATE
 ) SELECT count(*)
 FROM unnest($2::text[],$3::uuid[],$4::text[],$5::text[]) WITH ORDINALITY AS submitted(resource_kind,resource_id,action,row_scope,position)
 JOIN applications.menu_resources mr ON mr.app_id=$1 AND mr.resource_kind=submitted.resource_kind AND mr.resource_id=submitted.resource_id
 WHERE (SELECT count(*) FROM locked_tables)>=0 AND ((submitted.action='menu.enter' AND submitted.row_scope='all' AND jsonb_array_length(($6::jsonb->(position::integer-1))->'fields')=0)
 OR (submitted.resource_kind='form' AND ((submitted.action='data.create' AND submitted.row_scope='all') OR (submitted.action IN('data.read','data.edit','data.history') AND submitted.row_scope IN('all','own')))
 AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text(($6::jsonb->(position::integer-1))->'fields') field_id
 LEFT JOIN applications.form_views v ON v.app_id=$1 AND v.id=submitted.resource_id
 LEFT JOIN applications.fields f ON f.app_id=$1 AND f.table_id=v.table_id AND f.id=field_id::uuid AND NOT f.removed
 WHERE f.id IS NULL)))`, appID, kinds, ids, actions, scopes, grantJSON, groupID).Scan(&found)
		if err == nil && found != len(in.Grants) {
			err = ErrResourceInvalid
		}
		if err != nil {
			break
		}
		if _, err = tx.Exec(ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND group_id=$2)", appID, groupID); err != nil {
			break
		}
		if _, err = tx.Exec(ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND group_id=$2", appID, groupID); err != nil {
			break
		}
		_, err = tx.Exec(ctx, `WITH new_grants AS (
 INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope)
 SELECT $1,$2,resource_kind,resource_id,action,row_scope
 FROM unnest($3::text[],$4::uuid[],$5::text[],$6::text[]) AS submitted(resource_kind,resource_id,action,row_scope) RETURNING *
 ) INSERT INTO applications.grant_fields(app_id,grant_id,table_id,field_id)
 SELECT $1,g.id,v.table_id,fid::uuid FROM jsonb_to_recordset($7::jsonb) AS submitted("resourceKind" text,"resourceId" uuid,action text,"rowScope" text,fields jsonb)
 JOIN new_grants g ON g.resource_kind=submitted."resourceKind" AND g.resource_id=submitted."resourceId" AND g.action=submitted.action AND g.row_scope=submitted."rowScope"
 JOIN applications.form_views v ON v.app_id=g.app_id AND v.id=g.resource_id AND g.resource_kind='form'
 CROSS JOIN LATERAL jsonb_array_elements_text(submitted.fields) fid`, appID, groupID, kinds, ids, actions, scopes, grantJSON)
		reason = "GROUP_GRANTS_REPLACED"
		counts["grants"] = int64(len(in.Grants))
		result = map[string]any{"id": groupID, "policyRevision": before + 1}
	default:
		return Result{}, ErrInvalid
	}
	if err != nil {
		return Result{}, classify(err)
	}
	if kind != "application.create" {
		if _, err := tx.Exec(ctx, "UPDATE applications.apps SET policy_revision=policy_revision+1,updated_at=now() WHERE id=$1", appID); err != nil {
			return Result{}, classify(err)
		}
	}
	after := before + 1
	summary, _ := json.Marshal(map[string]any{"appId": appID, "operationId": in.OperationID, "beforePolicyRevision": before, "afterPolicyRevision": after, "changeCounts": counts})
	if _, err := tx.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,session_ref,reason_code,request_id,client_ip,user_agent,object_type,object_id,change_summary) VALUES('application_changed','success',$1,NULLIF($2,'')::uuid,$3,$4,NULLIF($5,'')::inet,NULLIF($6,''),$7,$8,$9::jsonb)`, actor.ID, p.SessionRef, reason, meta.RequestID, meta.ClientIP, meta.UserAgent, objectType, objectID, summary); err != nil {
		return Result{}, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return Result{}, err
	}
	if _, err := tx.Exec(ctx, "UPDATE applications.operations SET result_json=$3::jsonb,http_status=$4,location=$5 WHERE actor_user_id=$1 AND operation_id=$2", actor.ID, in.OperationID, raw, status, location); err != nil {
		return Result{}, err
	}
	if err := commitWrite(ctx, tx); err != nil {
		return Result{}, err
	}
	return Result{Status: status, Location: location, Data: raw}, nil
}
