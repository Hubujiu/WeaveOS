package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestDefinitionCreateUpdateVersionAndAuditQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	meta := RequestMetadata{RequestID: "definition-create-test"}
	created, err := f.app.SaveDefinition(ctx, f.actor, Identity, "", DefinitionInput{Name: "普通", Description: "说明", TemplateIDs: []string{f.template}, PermissionCodes: []string{"app.test.A"}}, meta)
	if err != nil {
		t.Fatalf("identity creation unavailable: %v", err)
	}
	cleanupDefinition(t, f, Identity, created.ID)
	if created.ID == "" || created.Version != 1 || created.Name != "普通" || len(created.TemplateIDs) != 1 {
		t.Fatal("created definition must match explicit configuration")
	}
	updated, err := f.app.SaveDefinition(ctx, f.actor, Identity, created.ID, DefinitionInput{Name: "普通2", Description: "新说明", Version: 1, TemplateIDs: []string{}, PermissionCodes: []string{}}, RequestMetadata{RequestID: "definition-update-test"})
	if err != nil || updated.Version != 2 || updated.Name != "普通2" || len(updated.PermissionCodes) != 0 || len(updated.TemplateIDs) != 0 {
		t.Fatalf("explicit replacement and version required: %v", err)
	}
	_, err = f.app.SaveDefinition(ctx, f.actor, Identity, created.ID, DefinitionInput{Name: "stale", Version: 1, TemplateIDs: []string{}, PermissionCodes: []string{}}, meta)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale version must conflict, got %v", err)
	}
	current, err := f.app.GetDefinition(ctx, f.actor, Identity, created.ID)
	if err != nil || current.Name != "普通2" {
		t.Fatal("stale update must not overwrite current config")
	}
	var count int
	var raw []byte
	if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM auth.authentication_events WHERE object_id=$1", created.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("only two actual mutations must produce audit")
	}
	if err := f.owner.QueryRow(ctx, "SELECT change_summary FROM auth.authentication_events WHERE object_id=$1 AND reason_code='IDENTITY_UPDATED'", created.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var summary map[string]json.RawMessage
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["before"] == nil || summary["after"] == nil {
		t.Fatal("safe summary must retain explicit before/after config")
	}
	if err := f.app.DeleteDefinition(ctx, f.actor, Identity, created.ID, 2, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.GetDefinition(ctx, f.actor, Identity, created.ID); !errors.Is(err, ErrMissing) {
		t.Fatal("deleted object must be missing")
	}
}

func TestDefinitionValidationAndReferenceProtectionQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	meta := RequestMetadata{RequestID: "definition-invalid-test"}
	for _, in := range []DefinitionInput{{Name: "   "}, {Name: "unknown", PermissionCodes: []string{"documents.delete"}}, {Name: "missing template", TemplateIDs: []string{"00000000-0000-4000-8000-000000000099"}}} {
		if _, err := f.app.SaveDefinition(ctx, f.actor, Identity, "", in, meta); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid config must fail as 400: %v", err)
		}
	}
	if err := f.app.DeleteDefinition(ctx, f.actor, Identity, f.i1, 1, meta); !errors.Is(err, ErrConflict) {
		t.Fatalf("member reference protects identity: %v", err)
	}
	if err := f.app.DeleteDefinition(ctx, f.actor, Template, f.template, 1, meta); !errors.Is(err, ErrConflict) {
		t.Fatalf("identity reference protects template: %v", err)
	}
}

func TestDefinitionAuditFailureRollsBackQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, err := f.app.SaveDefinition(ctx, f.actor, Identity, "", DefinitionInput{Name: "must-rollback", PermissionCodes: []string{}, TemplateIDs: []string{}}, RequestMetadata{})
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "23514" || pgerr.ConstraintName != "ck_authentication_events_request_id" {
		t.Fatalf("expected the audit request-id CHECK failure, got %v", err)
	}
	var count int
	if err := f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.identities WHERE name='must-rollback'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("audit failure must roll back object and relations")
	}
}

func TestDefinitionPaginationAndServerSearchQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	meta := RequestMetadata{RequestID: "definition-page-test"}
	for i := 0; i < 3; i++ {
		created, err := f.app.SaveDefinition(ctx, f.actor, Template, "", DefinitionInput{Name: "分页目标", PermissionCodes: []string{}}, meta)
		if err != nil {
			t.Fatalf("create duplicate display names: %v", err)
		}
		cleanupDefinition(t, f, Template, created.ID)
	}
	page, err := f.app.ListDefinitions(ctx, f.actor, Template, PageQuery{Page: 2, PageSize: 2, Search: "分页目标"})
	if err != nil || page.Total != 3 || len(page.Items) != 1 || page.Page != 2 || page.PageSize != 2 {
		t.Fatalf("filtered total + stable second page required: %v", err)
	}
	if _, err := f.app.ListDefinitions(ctx, f.actor, Template, PageQuery{Page: 1, PageSize: 101}); !errors.Is(err, ErrInvalid) {
		t.Fatal("page size above 100 must reject")
	}
}

func cleanupDefinition(t *testing.T, f *fixture, kind DefinitionKind, id string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		if kind == Identity {
			_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", id)
			_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.identity_templates WHERE identity_id=$1", id)
			_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.identities WHERE id=$1", id)
		} else {
			_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.template_permissions WHERE template_id=$1", id)
			_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.permission_templates WHERE id=$1", id)
		}
	})
}
