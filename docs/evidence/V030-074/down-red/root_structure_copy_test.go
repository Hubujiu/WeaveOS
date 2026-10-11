package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func rootCopyPlanInput(f *rootTaskHTTPFixture) apptemplates.CopyPlanInput {
	return apptemplates.CopyPlanInput{Source: apptemplates.CopySource{AppID: f.app, Kind: "form", ID: f.view}, Target: apptemplates.CopyTarget{AppID: &f.app, Name: "Form copy"}}
}
func rootCopyPreflightInput(plan apptemplates.CopyPlan, in apptemplates.CopyPlanInput) apptemplates.CopyPreflightInput {
	bindings := []apptemplates.Binding{}
	for _, id := range plan.ExternalReferences.UserIDs {
		bindings = append(bindings, apptemplates.Binding{Kind: "user", SourceID: id, TargetID: id})
	}
	for _, id := range plan.ExternalReferences.DepartmentIDs {
		bindings = append(bindings, apptemplates.Binding{Kind: "department", SourceID: id, TargetID: id})
	}
	groups := []apptemplates.CopyGroupMapping{}
	for _, g := range plan.Manifest.PermissionGroups {
		groups = append(groups, apptemplates.CopyGroupMapping{SourceGroupID: g.ID, Name: "Mapped group"})
	}
	return apptemplates.CopyPreflightInput{Source: in.Source, Target: in.Target, ExpectedSourceFingerprint: plan.SourceFingerprint, ExpectedTargetStructureVersion: plan.TargetStructureVersion, ExpectedTargetPolicyRevision: plan.TargetPolicyRevision, Bindings: bindings, GroupMappings: groups}
}
func TestRootStructureCopyPlanSingleSnapshotReadOnly(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	before := rootTemplateReadCounts(t, f)
	cfg := f.runtime.Config()
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	s := apptemplates.Service{Pool: pool}
	in := rootCopyPlanInput(f)
	plan, e := s.PlanCopy(f.ctx, rootTemplatePrincipal(f), in)
	if e != nil {
		t.Fatal("valid plan rejected", e)
	}
	if len(plan.SourceFingerprint) != 64 || len(plan.Manifest.Forms) != 1 || plan.Manifest.Forms[0].ID != f.view || len(plan.Manifest.Tables) != 1 || len(plan.Manifest.Workflows) != 1 || plan.TargetPolicyRevision != 1 {
		t.Fatal("plan lost configuration", plan)
	}
	var version int64
	if e = f.owner.QueryRow(f.ctx, "SELECT structure_version FROM applications.apps WHERE id=$1", f.app).Scan(&version); e != nil || plan.TargetStructureVersion != version {
		t.Fatal("wrong target version", e)
	}
	raw, _ := json.Marshal(plan)
	for _, id := range []string{f.record, f.instance, f.task} {
		if strings.Contains(string(raw), id) {
			t.Fatal("runtime identity leaked")
		}
	}
	pre, e := s.PreflightCopy(f.ctx, rootTemplatePrincipal(f), rootCopyPreflightInput(plan, in))
	if e != nil || !pre.Valid {
		t.Fatal("valid preflight rejected", e)
	}
	if pre.Counts.Tables != 1 || pre.Counts.Forms != 1 || pre.Counts.Workflows != 1 || pre.MappedManifest.Forms[0].ID != f.view {
		t.Fatal("wrong preview counts or allocated final ID")
	}
	if !reflect.DeepEqual(before, rootTemplateReadCounts(t, f)) {
		t.Fatal("read-side persisted state")
	}
	again, e := s.PlanCopy(f.ctx, rootTemplatePrincipal(f), in)
	if e != nil || again.SourceFingerprint != plan.SourceFingerprint {
		t.Fatal("unstable fingerprint", e)
	}
}
func TestRootStructureCopyCurrentAuthorityAndScope(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	s := apptemplates.Service{Pool: f.runtime}
	p := rootTemplatePrincipal(f)
	in := rootCopyPlanInput(f)
	forged := p
	forged.UserID = f.other
	forged.Record.UserID = f.other
	forged.BootstrapAdmin = true
	if _, e := s.PlanCopy(f.ctx, forged, in); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("forged manager accepted", e)
	}
	app := in
	app.Source.Kind = "application"
	app.Source.ID = f.app
	app.Target.AppID = nil
	if _, e := s.PlanCopy(f.ctx, p, app); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("owner without create grant accepted", e)
	}
	rootTemplateGrantCreate(t, f, f.actor)
	plan, e := s.PlanCopy(f.ctx, p, app)
	if e != nil || plan.TargetStructureVersion != 0 || plan.TargetPolicyRevision != 0 {
		t.Fatal("current creation grant ignored", e)
	}
	missing := f.id(t)
	in.Target.DirectoryID = &missing
	if _, e = s.PlanCopy(f.ctx, p, in); !errors.Is(e, applications.ErrResourceInvalid) {
		t.Fatal("missing target directory accepted", e)
	}
	in.Target.DirectoryID = nil
	in.Target.AppID = &missing
	if _, e = s.PlanCopy(f.ctx, p, in); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("missing target accepted", e)
	}
	p.Record.AuthVersion = "2"
	if _, e = s.PlanCopy(f.ctx, p, app); !errors.Is(e, session.ErrUnauthorized) {
		t.Fatal("stale identity accepted", e)
	}
}
func TestRootStructureCopyPreflightSourceAndTargetCAS(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	s := apptemplates.Service{Pool: f.runtime}
	p := rootTemplatePrincipal(f)
	in := rootCopyPlanInput(f)
	plan, e := s.PlanCopy(f.ctx, p, in)
	if e != nil {
		t.Fatal(e)
	}
	req := rootCopyPreflightInput(plan, in)
	req.ExpectedTargetPolicyRevision++
	if _, e = s.PreflightCopy(f.ctx, p, req); !errors.Is(e, applications.ErrPolicyConflict) {
		t.Fatal("target policy CAS ignored", e)
	}
	req = rootCopyPreflightInput(plan, in)
	req.ExpectedSourceFingerprint = strings.Repeat("a", 64)
	if _, e = s.PreflightCopy(f.ctx, p, req); !errors.Is(e, apptemplates.ErrCopySourceChanged) {
		t.Fatal("source CAS ignored", e)
	}
	req = rootCopyPreflightInput(plan, in)
	req.Bindings = []apptemplates.Binding{}
	if _, e = s.PreflightCopy(f.ctx, p, req); !errors.Is(e, apptemplates.ErrInvalid) {
		t.Fatal("implicit personnel mapping accepted", e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.form_views SET name='Changed source' WHERE id=$1", f.view); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PreflightCopy(f.ctx, p, rootCopyPreflightInput(plan, in)); !errors.Is(e, apptemplates.ErrCopySourceChanged) {
		t.Fatal("changed source accepted", e)
	}
}

func TestRootStructureCopyMissingSourceIsNotFound(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	s := apptemplates.Service{Pool: f.runtime}
	in := rootCopyPlanInput(f)
	in.Source.ID = f.id(t)
	if _, e := s.PlanCopy(f.ctx, rootTemplatePrincipal(f), in); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("missing source needs not-found", e)
	}
}
func TestRootStructureCopyTargetCASAndLiveMapping(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	s := apptemplates.Service{Pool: f.runtime}
	in := rootCopyPlanInput(f)
	p := rootTemplatePrincipal(f)
	plan, e := s.PlanCopy(f.ctx, p, in)
	if e != nil {
		t.Fatal(e)
	}
	req := rootCopyPreflightInput(plan, in)
	req.ExpectedTargetStructureVersion++
	var conflict *appstructure.Error
	if _, e = s.PreflightCopy(f.ctx, p, req); !errors.As(e, &conflict) || conflict.Code != "APPLICATION_STRUCTURE_CONFLICT" {
		t.Fatal("target structure CAS ignored", e)
	}
	req = rootCopyPreflightInput(plan, in)
	for i := range req.Bindings {
		req.Bindings[i].TargetID = f.other
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.other); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PreflightCopy(f.ctx, p, req); e == nil {
		t.Fatal("inactive mapped user accepted")
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE personnel.permission_catalog SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PreflightCopy(f.ctx, p, rootCopyPreflightInput(plan, in)); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("revoked source authority accepted", e)
	}
}

func TestRootStructureCopyReceiptSchema(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	for _, kind := range []string{"application", "directory", "table", "form"} {
		t.Run(kind, func(t *testing.T) {
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			op, id := f.id(t), f.id(t)
			location := "/api/v1/applications/" + f.app
			if kind == "application" {
				id = f.app
			} else {
				suffix := map[string]string{"directory": "directories", "table": "tables", "form": "forms"}[kind]
				location += "/" + suffix + "/" + id
			}
			raw, _ := json.Marshal(map[string]any{"operationId": op, "appId": f.app, "resourceKind": kind, "id": id, "structureVersion": 3, "policyRevision": 2, "status": "copied"})
			_, e = tx.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'structure.copy',decode(repeat('12',32),'hex'),$4,201,$5)`, f.actor, op, f.app, raw, location)
			if e != nil {
				t.Fatal("valid closed copy receipt rejected", e)
			}
		})
	}
	for _, mode := range []string{"extra", "missing", "kind", "id", "id-zero", "id-app", "op", "app", "status", "policy-string", "policy-zero", "policy-fraction", "structure-zero", "structure-big", "null", "http", "location"} {
		t.Run(mode, func(t *testing.T) {
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			op := f.id(t)
			status := 201
			location := "/api/v1/applications/" + f.app
			receipt := map[string]any{"operationId": op, "appId": f.app, "resourceKind": "application", "id": f.app, "structureVersion": 1, "policyRevision": 1, "status": "copied"}
			switch mode {
			case "extra":
				receipt["manifest"] = map[string]any{}
			case "missing":
				delete(receipt, "id")
			case "kind":
				receipt["resourceKind"] = "workflow"
			case "id":
				receipt["id"] = "wrong"
			case "id-zero":
				receipt["id"] = "00000000-0000-0000-0000-000000000000"
			case "id-app":
				receipt["id"] = f.id(t)
			case "op":
				receipt["operationId"] = f.id(t)
			case "app":
				receipt["appId"] = f.id(t)
			case "status":
				receipt["status"] = "imported"
			case "policy-string":
				receipt["policyRevision"] = "1"
			case "policy-zero":
				receipt["policyRevision"] = 0
			case "policy-fraction":
				receipt["policyRevision"] = 1.5
			case "structure-zero":
				receipt["structureVersion"] = 0
			case "structure-big":
				receipt["structureVersion"] = 9007199254740992
			case "null":
				receipt["structureVersion"] = nil
			case "http":
				status = 200
			case "location":
				location = "/api/v1/application-operations/" + op
			}
			raw, _ := json.Marshal(receipt)
			_, e = tx.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'structure.copy',decode(repeat('12',32),'hex'),$4,$5,$6)`, f.actor, op, f.app, raw, status, location)
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "23514" || pg.ConstraintName != "ck_structure_copy_result" {
				t.Fatal("receipt constraint missing or wrong failure", e)
			}
		})
	}
}

func rootCopyMigration(t *testing.T) (string, string) {
	t.Helper()
	raw, e := os.ReadFile("../../../../db/migrations/00036_structure_copy.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("missing migration boundary")
	}
	return parts[0], parts[1]
}
func TestRootStructureCopyDownRefusesDurableOrPending(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	_, down := rootCopyMigration(t)
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprint(pending), func(t *testing.T) {
			tx, e := f.owner.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(f.ctx)
			op := f.id(t)
			if pending {
				_, e = tx.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint) VALUES($1,$2,$3,'structure.copy',decode(repeat('12',32),'hex'))`, f.actor, op, f.app)
			} else {
				raw, _ := json.Marshal(map[string]any{"operationId": op, "appId": f.app, "resourceKind": "application", "id": f.app, "structureVersion": 1, "policyRevision": 1, "status": "copied"})
				_, e = tx.Exec(f.ctx, `INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'structure.copy',decode(repeat('12',32),'hex'),$4,201,$5)`, f.actor, op, f.app, raw, "/api/v1/applications/"+f.app)
			}
			if e != nil {
				t.Fatal(e)
			}
			_, e = tx.Exec(f.ctx, down)
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "55000" {
				t.Fatal("Down did not refuse retained copy", e)
			}
		})
	}
}
func TestRootStructureCopyEmptyDownRestoresOldConstraints(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	dbname := "weaveos_v074_empty_" + strings.ReplaceAll(f.id(t), "-", "")
	quoted := pgx.Identifier{dbname}.Sanitize()
	if _, e := f.owner.Exec(f.ctx, "CREATE DATABASE "+quoted); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	u.Path = "/" + dbname
	pool, e := pgxpool.New(f.ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		pool.Close()
		_, e := f.owner.Exec(f.ctx, "DROP DATABASE "+quoted)
		if e != nil {
			t.Error(e)
		}
	}()
	files, e := filepath.Glob("../../../../db/migrations/[0-9]*.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, file := range files {
		if filepath.Base(file) > "00035_structure_deletion.sql" {
			break
		}
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(f.ctx, strings.SplitN(string(raw), "-- +goose Down", 2)[0]); e != nil {
			t.Fatal(file, e)
		}
	}
	q := `SELECT string_agg(conname||':'||pg_get_constraintdef(oid),'|' ORDER BY conname) FROM pg_constraint WHERE conrelid='applications.operations'::regclass`
	var before, after string
	if e = pool.QueryRow(f.ctx, q).Scan(&before); e != nil {
		t.Fatal(e)
	}
	up, down := rootCopyMigration(t)
	if _, e = pool.Exec(f.ctx, up); e != nil {
		t.Fatal(e)
	}
	tx, e := pool.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	if _, e = tx.Exec(f.ctx, down); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(f.ctx, q).Scan(&after); e != nil || before != after {
		t.Fatal("Down did not restore exact hot35 checks", e)
	}
}
