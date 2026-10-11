package main

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
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
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.apps SET owner_user_id=$1 WHERE id=$2", f.other, f.app); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PreflightCopy(f.ctx, p, rootCopyPreflightInput(plan, in)); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("revoked source authority accepted", e)
	}
}
