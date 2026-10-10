package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func rootTemplateImportReceipt(t *testing.T, r applications.Result, op string) string {
	t.Helper()
	var data map[string]any
	if r.Status != 201 || json.Unmarshal(r.Data, &data) != nil || len(data) != 5 || data["operationId"] != op || data["status"] != "imported" || data["policyRevision"] != float64(1) || data["structureVersion"] != float64(1) {
		t.Fatal("invalid import receipt", r)
	}
	app, ok := data["appId"].(string)
	if !ok || app == "" || r.Location != "/api/v1/applications/"+app {
		t.Fatal("invalid import location", r)
	}
	return app
}
func rootTemplateImportSource(t *testing.T) (*rootTaskHTTPFixture, apptemplates.Manifest, []apptemplates.Binding, session.Principal, string) {
	t.Helper()
	f, m, b, _ := rootTemplatePreflightSource(t)
	identity := rootTemplateGrantCreate(t, f, f.other)
	parent, child := f.id(t), f.id(t)
	m.Directories = []apptemplates.Directory{{ID: child, Name: "Child", ParentID: &parent, Position: 1}, {ID: parent, Name: "Root", Position: 0}}
	m.Tables[0].DirectoryID = &child
	m.Forms[0].DirectoryID = &child
	second := m.Forms[0]
	second.ID = f.id(t)
	second.Name = "Second view"
	m.Forms = append(m.Forms, second)
	m.PermissionGroups = []apptemplates.PermissionGroup{{ID: f.id(t), Name: "Retained role", Enabled: false, MemberIDs: []string{f.actor}, Grants: []apptemplates.Grant{{ResourceKind: "form", ResourceID: m.Forms[0].ID, Action: "data.read", RowScope: "own", Fields: []string{f.field}}}}}
	p := session.Principal{UserID: f.other, Record: session.Record{UserID: f.other, AuthVersion: "1"}}
	if _, e := apptemplates.NormalizeManifest(m); e != nil {
		t.Fatal("independent full import fixture", e)
	}
	return f, m, b, p, identity
}
func TestRootTemplateImportCompleteEmptyStructureCurrentOwnerAndDisabledFlow(t *testing.T) {
	f, m, b, p, _ := rootTemplateImportSource(t)
	op := f.id(t)
	original, _ := json.Marshal(m)
	cfg := f.runtime.Config()
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	s := apptemplates.Service{Pool: pool}
	r, e := s.Import(f.ctx, p, op, m, b, applications.Metadata{RequestID: "template-import-trace"})
	if e != nil {
		t.Fatal("complete atomic import unavailable", e)
	}
	app := rootTemplateImportReceipt(t, r, op)
	if app == f.app {
		t.Fatal("import reused source app")
	}
	after, _ := json.Marshal(m)
	if !bytes.Equal(original, after) {
		t.Fatal("import mutated source")
	}
	var owner string
	var policy, structure int
	if e = f.owner.QueryRow(f.ctx, "SELECT owner_user_id::text,policy_revision,structure_version FROM applications.apps WHERE id=$1", app).Scan(&owner, &policy, &structure); e != nil || owner != p.UserID || policy != 1 || structure != 1 {
		t.Fatal("invalid new owner/version", owner, policy, structure, e)
	}
	out, e := s.ExportManifest(f.ctx, p, app)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Directories) != 2 || len(out.Tables) != 1 || len(out.Forms) != 2 || len(out.Workflows) != 1 || len(out.PermissionGroups) != 1 {
		t.Fatal("incomplete import", out)
	}
	if out.Tables[0].ID == f.table || out.Tables[0].Fields[0].ID == f.field || out.Workflows[0].ID == f.flow {
		t.Fatal("reused source identity")
	}
	for _, v := range out.Forms {
		if v.TableID != out.Tables[0].ID {
			t.Fatal("shared views split table")
		}
	}
	g := out.PermissionGroups[0]
	if g.Enabled || !reflect.DeepEqual(g.MemberIDs, []string{f.actor}) || len(g.Grants) != 1 || g.Grants[0].Fields[0] != out.Tables[0].Fields[0].ID {
		t.Fatal("permission configuration changed", g)
	}
	var state string
	var current, candidate, instances, publications, commands int
	e = f.owner.QueryRow(f.ctx, `SELECT state,current_version,candidate_version,(SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1),(SELECT count(*) FROM applications.workflow_publications WHERE app_id=$1),(SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1::text) FROM applications.workflow_definitions WHERE app_id=$1`, app).Scan(&state, &current, &candidate, &instances, &publications, &commands)
	if e != nil || state != "disabled" || current != 0 || candidate != 1 || instances != 0 || publications != 0 || commands != 0 {
		t.Fatal("import ran or published workflow", state, current, candidate, instances, publications, commands, e)
	}
	var rows int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM "+pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(out.Tables[0].ID, "-", "")}.Sanitize()).Scan(&rows); e != nil || rows != 0 {
		t.Fatal("import included business records", rows, e)
	}
	var request string
	var count int
	if e = f.owner.QueryRow(f.ctx, `SELECT count(*),min(request_id) FROM auth.authentication_events WHERE reason_code='APPLICATION_CREATED' AND change_summary->>'operationId'=$1`, op).Scan(&count, &request); e != nil || count != 1 || request != "template-import-trace" {
		t.Fatal("missing correlated minimum audit", count, request, e)
	}
}
func TestRootTemplateImportEmptyApplicationAndStableReplayAfterRevocation(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	identity := rootTemplateGrantCreate(t, f, f.actor)
	p := rootTemplatePrincipal(f)
	p.SessionRef = ""
	m := apptemplates.Manifest{Format: "weaveos.structure-template", Version: 1, Application: apptemplates.Application{ID: f.id(t), Name: "Empty import"}, Directories: []apptemplates.Directory{}, Tables: []apptemplates.Table{}, Forms: []apptemplates.Form{}, Workflows: []apptemplates.Workflow{}, PermissionGroups: []apptemplates.PermissionGroup{}}
	s := apptemplates.Service{Pool: f.runtime}
	op := f.id(t)
	r, e := s.Import(f.ctx, p, op, m, []apptemplates.Binding{}, applications.Metadata{RequestID: "empty-import"})
	if e != nil {
		t.Fatal("empty structure import unavailable", e)
	}
	app := rootTemplateImportReceipt(t, r, op)
	if _, e = f.owner.Exec(f.ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity); e != nil {
		t.Fatal(e)
	}
	replay, e := s.Import(f.ctx, p, op, m, []apptemplates.Binding{}, applications.Metadata{RequestID: "new-trace"})
	if e != nil || r.Status != replay.Status || r.Location != replay.Location || !rootTemplateJSONEqual(r.Data, replay.Data) {
		t.Fatal("same-key confirmed recovery changed", replay, e)
	}
	receipt, e := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, p, op)
	if e != nil || !rootTemplateJSONEqual(receipt.Result, r.Data) {
		t.Fatal("receipt read after revoke", receipt, e)
	}
	if _, e = s.Import(f.ctx, p, f.id(t), m, []apptemplates.Binding{}, applications.Metadata{}); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("new import ignored revoked create", e)
	}
	m.Application.Name = "Different request"
	if _, e = s.Import(f.ctx, p, op, m, []apptemplates.Binding{}, applications.Metadata{}); !errors.Is(e, applications.ErrOperationConflict) {
		t.Fatal("same key changed body accepted", e)
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1", app).Scan(&n); e != nil || n != 1 {
		t.Fatal("replay duplicated writes", n, e)
	}
}
func TestRootTemplateImportRejectsCurrentAuthorityAndTargetsWithoutWrites(t *testing.T) {
	for _, mode := range []string{"owner-only", "forged-root", "stale-auth", "inactive-target", "missing-department"} {
		t.Run(mode, func(t *testing.T) {
			f, m, b, p, identity := rootTemplateImportSource(t)
			want := applications.ErrDenied
			switch mode {
			case "owner-only", "forged-root":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity); e != nil {
					t.Fatal(e)
				}
				p.BootstrapAdmin = mode == "forged-root"
			case "stale-auth":
				p.Record.AuthVersion = "0"
				want = session.ErrUnauthorized
			case "inactive-target":
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.actor); e != nil {
					t.Fatal(e)
				}
				want = applications.ErrResourceInvalid
			case "missing-department":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM personnel.departments WHERE id=$1", b[1].TargetID); e != nil {
					t.Fatal(e)
				}
				want = applications.ErrResourceInvalid
			}
			before := rootTemplateReadCounts(t, f)
			_, e := (&apptemplates.Service{Pool: f.runtime}).Import(f.ctx, p, f.id(t), m, b, applications.Metadata{})
			if !errors.Is(e, want) {
				t.Fatal("wrong import refusal", e, want)
			}
			if after := rootTemplateReadCounts(t, f); !reflect.DeepEqual(before, after) {
				t.Fatal("failed import left partial writes", before, after)
			}
		})
	}
}

func rootTemplateJSONEqual(a, b json.RawMessage) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func rootTemplateCanonicalStructure(t *testing.T, m apptemplates.Manifest) apptemplates.Manifest {
	t.Helper()
	sort.Slice(m.Directories, func(i, j int) bool { return m.Directories[i].Name < m.Directories[j].Name })
	sort.Slice(m.Tables, func(i, j int) bool { return m.Tables[i].Name < m.Tables[j].Name })
	sort.Slice(m.Forms, func(i, j int) bool { return m.Forms[i].Name < m.Forms[j].Name })
	sort.Slice(m.Workflows, func(i, j int) bool { return m.Workflows[i].Name < m.Workflows[j].Name })
	sort.Slice(m.PermissionGroups, func(i, j int) bool { return m.PermissionGroups[i].Name < m.PermissionGroups[j].Name })
	next := 0
	m, e := apptemplates.RemapInternalIDs(m, func() (string, error) { next++; return fmt.Sprintf("00000000-0000-4000-8000-%012x", next), nil })
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestRootTemplateImportRoundtripPreservesMappedConfigurationAfterTargetRevocation(t *testing.T) {
	f, m, b, p, identity := rootTemplateImportSource(t)
	s := apptemplates.Service{Pool: f.runtime}
	op := f.id(t)
	r, e := s.Import(f.ctx, p, op, m, b, applications.Metadata{RequestID: "roundtrip"})
	if e != nil {
		t.Fatal(e)
	}
	app := rootTemplateImportReceipt(t, r, op)
	out, e := s.ExportManifest(f.ctx, p, app)
	if e != nil {
		t.Fatal(e)
	}
	expected, e := apptemplates.MapExternalReferences(m, b)
	if e != nil {
		t.Fatal(e)
	}
	want, got := rootTemplateCanonicalStructure(t, expected), rootTemplateCanonicalStructure(t, out)
	a, _ := json.Marshal(want)
	z, _ := json.Marshal(got)
	if !rootTemplateJSONEqual(a, z) {
		t.Fatalf("roundtrip lost supported configuration\nwant %s\ngot %s", a, z)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.actor); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity); e != nil {
		t.Fatal(e)
	}
	reverse := []apptemplates.Binding{b[1], b[0]}
	again, e := s.Import(f.ctx, p, op, m, reverse, applications.Metadata{})
	if e != nil || !rootTemplateJSONEqual(r.Data, again.Data) {
		t.Fatal("confirmed original cannot recover after target or create revocation", e)
	}
}
