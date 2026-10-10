package main

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apptemplates"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
)

func rootTemplateGrantCreate(t *testing.T, f *rootTaskHTTPFixture, user string) string {
	t.Helper()
	id := f.id(t)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO personnel.identities(id,name) VALUES($1,'template creator')", id); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'applications.create')", id); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", user, id); e != nil {
		t.Fatal(e)
	}
	return id
}
func rootTemplatePreflightSource(t *testing.T) (*rootTaskHTTPFixture, apptemplates.Manifest, []apptemplates.Binding, string) {
	t.Helper()
	f := rootHTTPTaskSetup(t)
	m, e := (&apptemplates.Service{Pool: f.runtime}).ExportManifest(f.ctx, rootTemplatePrincipal(f), f.app)
	if e != nil {
		t.Fatal(e)
	}
	source, target := f.id(t), f.id(t)
	if _, e = f.owner.Exec(f.ctx, "INSERT INTO personnel.departments(id,parent_id,name) SELECT $1,id,'Target department' FROM personnel.departments WHERE is_root", target); e != nil {
		t.Fatal(e)
	}
	m.Tables[0].Fields = append(m.Tables[0].Fields, appfields.Field{ID: f.id(t), Name: "Department", Kind: "department", Default: json.RawMessage(`"` + source + `"`), Config: json.RawMessage(`{}`)})
	return f, m, []apptemplates.Binding{{Kind: "user", SourceID: f.actor, TargetID: f.actor}, {Kind: "department", SourceID: source, TargetID: target}}, target
}
func rootTemplateReadCounts(t *testing.T, f *rootTaskHTTPFixture) []int64 {
	t.Helper()
	out := []int64{}
	for _, q := range []string{"SELECT count(*) FROM applications.apps", "SELECT count(*) FROM applications.operations", "SELECT count(*) FROM auth.authentication_events", "SELECT count(*) FROM personnel.member_configuration", "SELECT count(*) FROM pg_tables WHERE schemaname='appdata'"} {
		var n int64
		if e := f.owner.QueryRow(f.ctx, q).Scan(&n); e != nil {
			t.Fatal(e)
		}
		out = append(out, n)
	}
	return out
}
func TestRootTemplatePreflightCurrentCreateGrantAndNoWrites(t *testing.T) {
	f, m, bindings, _ := rootTemplatePreflightSource(t)
	rootTemplateGrantCreate(t, f, f.other)
	p := session.Principal{UserID: f.other, Record: session.Record{UserID: f.other, AuthVersion: "1"}}
	cfg := f.runtime.Config()
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	before := rootTemplateReadCounts(t, f)
	got, e := (&apptemplates.Service{Pool: pool}).Preflight(f.ctx, p, m, bindings)
	if e != nil || !got.Valid || got.Counts.Tables != 1 || got.Counts.Fields != 2 || got.Counts.Forms != 1 || got.Counts.Workflows != 1 {
		t.Fatal("valid creator preflight unavailable", got, e)
	}
	after := rootTemplateReadCounts(t, f)
	for i, n := range before {
		if after[i] != n {
			t.Fatal("preflight wrote application, audit, operation, personnel config, or physical table", before, after)
		}
	}
}
func TestRootTemplatePreflightRejectsCurrentRevocationsAndInvalidTargets(t *testing.T) {
	for _, mode := range []string{"owner-only", "forged-root", "revoked-create", "stale-account", "inactive-target", "missing-department"} {
		t.Run(mode, func(t *testing.T) {
			f, m, b, target := rootTemplatePreflightSource(t)
			p := rootTemplatePrincipal(f)
			identity := rootTemplateGrantCreate(t, f, f.actor)
			want := applications.ErrDenied
			switch mode {
			case "owner-only", "forged-root", "revoked-create":
				if mode == "forged-root" {
					p.BootstrapAdmin = true
				}
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity); e != nil {
					t.Fatal(e)
				}
			case "stale-account":
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor); e != nil {
					t.Fatal(e)
				}
				want = session.ErrUnauthorized
			case "inactive-target":
				b[0].TargetID = f.other
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.other); e != nil {
					t.Fatal(e)
				}
				want = applications.ErrResourceInvalid
			case "missing-department":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM personnel.departments WHERE id=$1", target); e != nil {
					t.Fatal(e)
				}
				want = applications.ErrResourceInvalid
			}
			if _, e := (&apptemplates.Service{Pool: f.runtime}).Preflight(f.ctx, p, m, b); !errors.Is(e, want) {
				t.Fatal("wrong live denial", mode, e)
			}
		})
	}
}
func TestRootTemplatePreflightActualBootstrapAndMappingFailure(t *testing.T) {
	f, m, b, _ := rootTemplatePreflightSource(t)
	p := rootTemplateCurrentBootstrap(t, f, f.actor)
	s := &apptemplates.Service{Pool: f.runtime}
	if got, e := s.Preflight(f.ctx, p, m, b); e != nil || !got.Valid {
		t.Fatal("actual Bootstrap rejected", e)
	}
	before := rootTemplateReadCounts(t, f)
	if _, e := s.Preflight(f.ctx, p, m, b[:1]); !errors.Is(e, apptemplates.ErrInvalid) {
		t.Fatal("incomplete mapping accepted", e)
	}
	after := rootTemplateReadCounts(t, f)
	for i, n := range before {
		if after[i] != n {
			t.Fatal("failed preflight wrote state")
		}
	}
}
