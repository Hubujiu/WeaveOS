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
	"sync"
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

func TestRootStructureCopyExecuteFourKindsEmptyAndIndependent(t *testing.T) {
	for _, kind := range []string{"application", "directory", "table", "form"} {
		t.Run(kind, func(t *testing.T) {
			f := rootHTTPTaskSetup(t)
			s := apptemplates.Service{Pool: f.runtime}
			p := rootTemplatePrincipal(f)
			p.SessionRef = ""
			directory := f.id(t)
			if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.directories(id,app_id,name,position) VALUES($1,$2,'Source directory',0)", directory, f.app); e != nil {
				t.Fatal(e)
			}
			if _, e := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET directory_id=$1 WHERE id=$2", directory, f.table); e != nil {
				t.Fatal(e)
			}
			if _, e := f.owner.Exec(f.ctx, "UPDATE applications.form_views SET directory_id=$1 WHERE id=$2", directory, f.view); e != nil {
				t.Fatal(e)
			}
			in := rootCopyPlanInput(f)
			in.Source.Kind = kind
			in.Target.Name = "Independent copy"
			switch kind {
			case "application":
				in.Source.ID = f.app
				in.Target.AppID = nil
				rootTemplateGrantCreate(t, f, f.actor)
			case "directory":
				in.Source.ID = directory
			case "table":
				in.Source.ID = f.table
			}
			plan, e := s.PlanCopy(f.ctx, p, in)
			if e != nil {
				t.Fatal(e)
			}
			req := rootCopyPreflightInput(plan, in)
			op := f.id(t)
			before := rootTemplateReadCounts(t, f)
			result, e := s.ExecuteCopy(f.ctx, p, op, req, applications.Metadata{RequestID: "copy-write"})
			if e != nil {
				t.Fatal("atomic copy unavailable", e)
			}
			var receipt struct {
				OperationID, AppID, ResourceKind, ID, Status string
				StructureVersion, PolicyRevision             int64
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(result.Data, &receipt) != nil || json.Unmarshal(result.Data, &fields) != nil || len(fields) != 7 || result.Status != 201 || receipt.OperationID != op || receipt.ResourceKind != kind || receipt.Status != "copied" || receipt.ID == in.Source.ID {
				t.Fatal("wrong minimum receipt", string(result.Data))
			}
			expectedPolicy, expectedVersion := plan.TargetPolicyRevision, plan.TargetStructureVersion+1
			if kind == "application" {
				expectedPolicy = 1
				if receipt.AppID == f.app || receipt.ID != receipt.AppID {
					t.Fatal("app identity reused")
				}
			} else if receipt.AppID != f.app {
				t.Fatal("partial copy changed target")
			}
			if receipt.PolicyRevision != expectedPolicy || receipt.StructureVersion != expectedVersion {
				t.Fatal("wrong target versions", receipt)
			}
			var table, view string
			switch kind {
			case "application":
				e = f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.logical_tables WHERE app_id=$1", receipt.AppID).Scan(&table)
			case "directory":
				e = f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.logical_tables WHERE directory_id=$1", receipt.ID).Scan(&table)
			case "table":
				table = receipt.ID
			case "form":
				e = f.owner.QueryRow(f.ctx, "SELECT table_id::text FROM applications.form_views WHERE id=$1", receipt.ID).Scan(&table)
			}
			if e != nil || table == f.table {
				t.Fatal("new table absent or reused", e)
			}
			if e = f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.form_views WHERE table_id=$1", table).Scan(&view); e != nil || view == f.view {
				t.Fatal("new view absent", e)
			}
			var n int
			if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM "+pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(table, "-", "")}.Sanitize()).Scan(&n); e != nil || n != 0 {
				t.Fatal("copied business rows", n, e)
			}
			var state string
			var current, candidate int
			if e = f.owner.QueryRow(f.ctx, "SELECT state,current_version,candidate_version FROM applications.workflow_definitions WHERE view_id=$1", view).Scan(&state, &current, &candidate); e != nil || state != "disabled" || current != 0 || candidate != 1 {
				t.Fatal("flow copied active", e)
			}
			after := rootTemplateReadCounts(t, f)
			if after[1] != before[1]+1 || after[2] != before[2]+1 || after[4] != before[4]+1 {
				t.Fatal("wrong atomic operation/audit/table counts", before, after)
			}
			if _, e = f.owner.Exec(f.ctx, "UPDATE personnel.permission_catalog SET enabled=false WHERE app_id=$1", f.app); e != nil {
				t.Fatal(e)
			}
			replay, e := s.ExecuteCopy(f.ctx, p, op, req, applications.Metadata{})
			if e != nil || !rootTemplateJSONEqual(result.Data, replay.Data) || result.Location != replay.Location {
				t.Fatal("replay after revocation failed", e)
			}
			stored, e := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, p, op)
			if e != nil || !rootTemplateJSONEqual(stored.Result, result.Data) {
				t.Fatal("minimum recovery unavailable", e)
			}
			req.Target.Name = "Different body"
			if _, e = s.ExecuteCopy(f.ctx, p, op, req, applications.Metadata{}); !errors.Is(e, applications.ErrOperationConflict) {
				t.Fatal("same key different body accepted", e)
			}
		})
	}
}

func TestRootStructureCopyCrossAppIndependentGroupAndDirectory(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	s := apptemplates.Service{Pool: f.runtime}
	p := rootTemplatePrincipal(f)
	p.SessionRef = ""
	target, dir, group, existing := f.id(t), f.id(t), f.id(t), f.id(t)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO applications.apps(id,name,owner_user_id,structure_version) VALUES($1,'Target',$2,4)", []any{target, f.actor}},
		{"SELECT applications.register_catalog_entry($1)", []any{target}},
		{"INSERT INTO applications.directories(id,app_id,name,position) VALUES($1,$2,'Destination',0)", []any{dir, target}},
		{"INSERT INTO applications.permission_groups(id,app_id,name,enabled) VALUES($1,$2,'Existing unchanged',false),($3,$4,'Source role',false)", []any{existing, target, group, f.app}},
		{"INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", []any{f.app, group, f.actor}},
		{"INSERT INTO applications.grants(id,app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,$3,'form',$4,'data.read','own')", []any{f.id(t), f.app, group, f.view}},
	} {
		if _, e := f.owner.Exec(f.ctx, q.sql, q.args...); e != nil {
			t.Fatal(e)
		}
	}
	in := rootCopyPlanInput(f)
	in.Target.AppID = &target
	in.Target.DirectoryID = &dir
	plan, e := s.PlanCopy(f.ctx, p, in)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan.Manifest.PermissionGroups) != 1 {
		t.Fatal("source grant absent")
	}
	req := rootCopyPreflightInput(plan, in)
	for i := range req.Bindings {
		req.Bindings[i].TargetID = f.other
	}
	result, e := s.ExecuteCopy(f.ctx, p, f.id(t), req, applications.Metadata{RequestID: "copy-acceptance"})
	if e != nil {
		t.Fatal(e)
	}
	var receipt struct {
		ID                               string
		PolicyRevision, StructureVersion int64
	}
	if json.Unmarshal(result.Data, &receipt) != nil || receipt.PolicyRevision != 2 || receipt.StructureVersion != 5 {
		t.Fatal(string(result.Data))
	}
	var targetDir, tableDir, assignee, member, newGroup string
	var enabled bool
	if e = f.owner.QueryRow(f.ctx, "SELECT v.directory_id::text,t.directory_id::text FROM applications.form_views v JOIN applications.logical_tables t ON t.id=v.table_id WHERE v.id=$1", receipt.ID).Scan(&targetDir, &tableDir); e != nil || targetDir != dir || tableDir != dir {
		t.Fatal("root placement wrong", e)
	}
	if e = f.owner.QueryRow(f.ctx, "SELECT id::text,enabled FROM applications.permission_groups WHERE app_id=$1 AND id<>$2", target, existing).Scan(&newGroup, &enabled); e != nil || newGroup == group || enabled {
		t.Fatal("independent group lost", e)
	}
	if e = f.owner.QueryRow(f.ctx, "SELECT user_id::text FROM applications.group_members WHERE group_id=$1", newGroup).Scan(&member); e != nil || member != f.other {
		t.Fatal("member mapping lost", e)
	}
	if e = f.owner.QueryRow(f.ctx, `SELECT graph_json->'Nodes'->1->'Approval'->'AssigneeIDs'->>0 FROM applications.workflow_versions WHERE flow_id IN (SELECT id FROM applications.workflow_definitions WHERE view_id=$1)`, receipt.ID).Scan(&assignee); e != nil || assignee != f.other {
		t.Fatal("approver mapping lost", e)
	}
	var existingName string
	if e = f.owner.QueryRow(f.ctx, "SELECT name FROM applications.permission_groups WHERE id=$1", existing).Scan(&existingName); e != nil || existingName != "Existing unchanged" {
		t.Fatal("existing group modified", e)
	}
	var sourcePolicy int
	if e = f.owner.QueryRow(f.ctx, "SELECT policy_revision FROM applications.apps WHERE id=$1", f.app).Scan(&sourcePolicy); e != nil || sourcePolicy != 1 {
		t.Fatal("source policy mutated", e)
	}
}
func TestRootStructureCopyFaultRollsBackEverything(t *testing.T) {
	for _, phase := range []string{"form", "workflow", "audit", "receipt"} {
		t.Run(phase, func(t *testing.T) {
			f := rootHTTPTaskSetup(t)
			s := apptemplates.Service{Pool: f.runtime}
			p := rootTemplatePrincipal(f)
			p.SessionRef = ""
			in := rootCopyPlanInput(f)
			plan, e := s.PlanCopy(f.ctx, p, in)
			if e != nil {
				t.Fatal(e)
			}
			req := rootCopyPreflightInput(plan, in)
			op := f.id(t)
			name := "copy_fault_" + strings.ReplaceAll(op, "-", "")
			fn := pgx.Identifier{"public", name}.Sanitize()
			trigger := pgx.Identifier{name}.Sanitize()
			table, event, condition := "applications.form_views", "INSERT", "NEW.app_id='"+f.app+"'::uuid"
			switch phase {
			case "workflow":
				table = "applications.workflow_versions"
			case "audit":
				table = "auth.authentication_events"
				condition = "NEW.change_summary->>'operationId'='" + op + "'"
			case "receipt":
				table = "applications.operations"
				event = "UPDATE"
				condition = "NEW.operation_id='" + op + "'::uuid"
			}
			if _, e = f.owner.Exec(f.ctx, "CREATE FUNCTION "+fn+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF "+condition+" THEN RAISE EXCEPTION 'isolated copy failure';END IF;RETURN NEW;END $$;CREATE TRIGGER "+trigger+" BEFORE "+event+" ON "+table+" FOR EACH ROW EXECUTE FUNCTION "+fn+"()"); e != nil {
				t.Fatal(e)
			}
			defer func() {
				if _, e := f.owner.Exec(f.ctx, "DROP TRIGGER "+trigger+" ON "+table+";DROP FUNCTION "+fn+"()"); e != nil {
					t.Error(e)
				}
			}()
			before := rootTemplateReadCounts(t, f)
			_, e = s.ExecuteCopy(f.ctx, p, op, req, applications.Metadata{RequestID: "copy-acceptance"})
			var fault *pgconn.PgError
			if !errors.As(e, &fault) || fault.Message != "isolated copy failure" {
				t.Fatal("did not reach injected phase", e)
			}
			if after := rootTemplateReadCounts(t, f); !reflect.DeepEqual(before, after) {
				t.Fatal("partial durable state", before, after)
			}
			current, e := s.PlanCopy(f.ctx, p, in)
			if e != nil || current.SourceFingerprint != plan.SourceFingerprint || current.TargetStructureVersion != plan.TargetStructureVersion || current.TargetPolicyRevision != plan.TargetPolicyRevision {
				t.Fatal("rollback changed source/versions", e)
			}
		})
	}
}

func TestRootStructureCopyConcurrentSameKeyAndTargetCAS(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		t.Run(fmt.Sprint(sameKey), func(t *testing.T) {
			f := rootHTTPTaskSetup(t)
			s := apptemplates.Service{Pool: f.runtime}
			p := rootTemplatePrincipal(f)
			p.SessionRef = ""
			in := rootCopyPlanInput(f)
			plan, e := s.PlanCopy(f.ctx, p, in)
			if e != nil {
				t.Fatal(e)
			}
			req := rootCopyPreflightInput(plan, in)
			ops := [2]string{f.id(t), f.id(t)}
			if sameKey {
				ops[1] = ops[0]
			}
			start := make(chan struct{})
			var wg sync.WaitGroup
			var results [2]applications.Result
			var errs [2]error
			before := rootTemplateReadCounts(t, f)
			for i := range 2 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					results[i], errs[i] = s.ExecuteCopy(f.ctx, p, ops[i], req, applications.Metadata{RequestID: "copy-concurrent"})
				}(i)
			}
			close(start)
			wg.Wait()
			success := 0
			for _, e := range errs {
				if e == nil {
					success++
				} else {
					var conflict *appstructure.Error
					if !errors.As(e, &conflict) || conflict.Code != "APPLICATION_STRUCTURE_CONFLICT" {
						t.Fatal("unexpected concurrent error", e)
					}
				}
			}
			if sameKey {
				if success != 2 || !rootTemplateJSONEqual(results[0].Data, results[1].Data) {
					t.Fatal("same key did not converge", errs)
				}
			} else if success != 1 {
				t.Fatal("two writes consumed same target CAS", success)
			}
			after := rootTemplateReadCounts(t, f)
			if after[1] != before[1]+1 || after[2] != before[2]+1 || after[4] != before[4]+1 {
				t.Fatal("duplicate or orphaned durable copies", before, after)
			}
		})
	}
}
