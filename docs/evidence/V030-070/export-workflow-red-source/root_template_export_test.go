package main

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"testing"
)

func rootTemplatePrincipal(f *rootTaskHTTPFixture) session.Principal {
	return session.Principal{UserID: f.actor, SessionRef: "export-test", Record: session.Record{UserID: f.actor, AuthVersion: "1"}}
}
func TestRootTemplateExportCompleteConfigurationNotBusinessRows(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	s := apptemplates.Service{Pool: f.runtime}
	p := rootTemplatePrincipal(f)
	// This fixture has a real typed row plus a configured flow and runnable task.
	group := f.id(t)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.permission_groups(id,app_id,name,enabled) VALUES($1,$2,'Export role',false)", group, f.app); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", f.app, group, f.other); e != nil {
		t.Fatal(e)
	}
	var before int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1", f.app).Scan(&before); e != nil {
		t.Fatal(e)
	}
	got, e := s.ExportManifest(f.ctx, p, f.app)
	if e != nil {
		t.Fatal("owner export unavailable", e)
	}
	if got.Application.ID != f.app || len(got.Tables) != 1 || len(got.Forms) != 1 || len(got.Workflows) != 1 || len(got.PermissionGroups) != 1 || got.PermissionGroups[0].Enabled {
		t.Fatal("source configuration missing or changed")
	}
	raw, e := json.Marshal(got)
	if e != nil {
		t.Fatal(e)
	}
	for _, forbidden := range []string{f.record, f.instance, f.task, `"records"`, `"ownerUserId"`, `"bpmn_xml"`, `"original"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("template leaked runtime data %s", forbidden)
		}
	}
	if got.Tables[0].Fields[0].ID != f.field || got.Workflows[0].Graph.Nodes[1].Approval.AssigneeIDs[0] != f.actor || got.PermissionGroups[0].MemberIDs[0] != f.other {
		t.Fatal("configuration references dropped")
	}
	if _, e = apptemplates.DecodeManifest(raw); e != nil {
		t.Fatal("export is not importable manifest", e)
	}
	var after int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1", f.app).Scan(&after); e != nil || after != before {
		t.Fatal("read export wrote operation", after, e)
	}
}
func TestRootTemplateExportCurrentManagementIdentity(t *testing.T) {
	for _, mode := range []string{"member", "forged-root", "actual-root", "stale-account"} {
		t.Run(mode, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			s := apptemplates.Service{Pool: f.runtime}
			p := rootTemplatePrincipal(f)
			want := applications.ErrDenied
			switch mode {
			case "member":
				p.UserID = f.other
			case "forged-root":
				p.UserID = f.other
				p.BootstrapAdmin = true
			case "actual-root":
				p.UserID = f.other
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET is_bootstrap_admin=true WHERE id=$1", f.other); e != nil {
					t.Fatal(e)
				}
				want = nil
			case "stale-account":
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor); e != nil {
					t.Fatal(e)
				}
				want = session.ErrUnauthorized
			}
			got, e := s.ExportManifest(f.ctx, p, f.app)
			if want == nil {
				if e != nil || got.Application.ID != f.app {
					t.Fatal("actual Bootstrap denied", e)
				}
			} else if !errors.Is(e, want) {
				t.Fatalf("%s wrong denial %v", mode, e)
			}
		})
	}
}
func TestRootTemplateExportSingleConnectionAndDeterminism(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	cfg := f.runtime.Config()
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	s := apptemplates.Service{Pool: pool}
	p := rootTemplatePrincipal(f)
	first, e := s.ExportManifest(f.ctx, p, f.app)
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.ExportManifest(f.ctx, p, f.app)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("stable structure export is nondeterministic")
	}
	var n int
	if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_instances WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 0 {
		t.Fatal("export started workflow", e)
	}
}

func TestRootTemplateExportRejectsUnsupportedPersistedConfiguration(t *testing.T) {
	for _, which := range []string{"unknown-field-member", "unknown-layout-member", "unknown-field-presentation"} {
		t.Run(which, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			var query string
			switch which {
			case "unknown-field-member":
				query = `UPDATE applications.logical_tables SET fields_json=jsonb_set(fields_json,'{0,futureBehavior}','true') WHERE app_id=$1`
			case "unknown-field-presentation":
				query = `UPDATE applications.logical_tables SET fields_json=jsonb_set(fields_json,'{0,presentation,futureBehavior}','true') WHERE app_id=$1`
			case "unknown-layout-member":
				raw := `[{"id":"` + f.id(t) + `","kind":"divider","futureBehavior":true}]`
				if _, e := f.owner.Exec(f.ctx, `UPDATE applications.form_views SET layout=$2::jsonb WHERE app_id=$1`, f.app, raw); e != nil {
					t.Fatal(e)
				}
			}
			if query != "" {
				if _, e := f.owner.Exec(f.ctx, query, f.app); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := (&apptemplates.Service{Pool: f.runtime}).ExportManifest(f.ctx, rootTemplatePrincipal(f), f.app); !errors.Is(e, apptemplates.ErrNotExportable) {
				t.Fatal("silently discarded unsupported persisted configuration", e)
			}
		})
	}
}

func TestRootTemplateExportManagerSnapshotIsReadOnlyRepeatableRead(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	tx, app, e := (&applications.Application{Pool: f.runtime}).BeginManagerRead(f.ctx, rootTemplatePrincipal(f), f.app)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	var isolation, readonly string
	if e = tx.QueryRow(f.ctx, "SHOW transaction_isolation").Scan(&isolation); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(f.ctx, "SHOW transaction_read_only").Scan(&readonly); e != nil {
		t.Fatal(e)
	}
	if isolation != "repeatable read" || readonly != "on" {
		t.Fatal("unsafe read transaction", isolation, readonly)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.apps SET name='New concurrent name' WHERE id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	var got string
	if e = tx.QueryRow(f.ctx, "SELECT name FROM applications.apps WHERE id=$1", f.app).Scan(&got); e != nil || got != app.Name {
		t.Fatal("torn read snapshot", got, e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	next, e := (&apptemplates.Service{Pool: f.runtime}).ExportManifest(f.ctx, rootTemplatePrincipal(f), f.app)
	if e != nil || next.Application.Name != "New concurrent name" {
		t.Fatal("new request reused stale snapshot", e)
	}
}

func TestRootTemplateExportRejectsLostWorkflowConfiguration(t *testing.T) {
	for _, which := range []string{"missing-candidate", "unknown-graph", "unknown-approval", "unknown-trigger"} {
		t.Run(which, func(t *testing.T) {
			f := rootHTTPTaskSetup(t)
			query := ""
			switch which {
			case "missing-candidate":
				query = `UPDATE applications.workflow_definitions SET candidate_version=2 WHERE app_id=$1`
			case "unknown-graph":
				query = `UPDATE applications.workflow_versions SET graph_json=graph_json||'{"FutureBehavior":true}'::jsonb WHERE app_id=$1`
			case "unknown-approval":
				query = `UPDATE applications.workflow_versions SET graph_json=jsonb_set(graph_json,'{Nodes,1,Approval,FutureBehavior}','true') WHERE app_id=$1`
			case "unknown-trigger":
				query = `UPDATE applications.workflow_versions SET triggers_json='[{"event":"manual","condition":null,"futureBehavior":true}]'::jsonb WHERE app_id=$1`
			}
			if _, e := f.owner.Exec(f.ctx, query, f.app); e != nil {
				t.Fatal("corrupt source fixture", e)
			}
			if _, e := (&apptemplates.Service{Pool: f.runtime}).ExportManifest(f.ctx, rootTemplatePrincipal(f), f.app); !errors.Is(e, apptemplates.ErrNotExportable) {
				t.Fatal("incomplete workflow silently exported", e)
			}
		})
	}
}
