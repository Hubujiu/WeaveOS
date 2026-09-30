package persistence_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

// Oracle: approved Q25 physical dictionary, not the migration under test.
func TestPersonnelSchemaMatchesQ25(t *testing.T) {
	c := testConn(t)
	ctx := context.Background()
	for _, table := range []string{"departments", "department_members", "identities", "permission_templates", "permission_catalog", "member_configuration", "member_identities", "identity_templates", "identity_permissions", "template_permissions"} {
		var present bool
		if err := c.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "personnel."+table).Scan(&present); err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Errorf("Q25 requires personnel.%s, currently absent", table)
		}
	}
	var count int
	if err := c.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='auth' AND table_name='authentication_events'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 15 {
		t.Errorf("Q25 audit must preserve 12 original columns plus 3 compatible summary columns; got %d", count)
	}
}
func TestPersonnelConstraintsQ25(t *testing.T) {
	c := testConn(t)
	ctx := context.Background()
	var present bool
	_ = c.QueryRow(ctx, "SELECT to_regclass('personnel.departments') IS NOT NULL").Scan(&present)
	if !present {
		t.Fatal("Q25 root/group/identity invariants cannot be satisfied: personnel tables are absent")
	}
	var root string
	if err := c.QueryRow(ctx, "SELECT id FROM personnel.departments WHERE is_root").Scan(&root); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql, code string }{
		{"one protected root", "INSERT INTO personnel.departments(name,is_root) VALUES('second',true)", "23505"},
		{"nonroot needs parent", "INSERT INTO personnel.departments(name) VALUES('orphan')", "23514"},
		{"name required", "INSERT INTO personnel.identities(name) VALUES('   ')", "23514"},
		{"positive object version", "INSERT INTO personnel.identities(name,version) VALUES('valid',0)", "23514"},
		{"catalog no made-up system grants", "INSERT INTO personnel.permission_catalog(code,name,category) VALUES('documents.delete','invalid','system')", "23514"},
		{"application must exist as stable id", "INSERT INTO personnel.permission_catalog(code,name,category) VALUES('app.A','A','application')", "23514"},
		{"missing user rejected", "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES('00000000-0000-4000-8000-000000000099','00000000-0000-4000-8000-000000000098')", "23503"},
		{"audit cannot omit safe summary", "INSERT INTO auth.authentication_events(event_type,outcome,request_id) VALUES('personnel_changed','success','schema-test')", "23514"},
		{"audit summary must be object", "INSERT INTO auth.authentication_events(event_type,outcome,request_id,object_type,object_id,change_summary) VALUES('personnel_changed','success','schema-test','identity','00000000-0000-4000-8000-000000000099','[]')", "23514"},
		{"old auth events cannot smuggle summaries", "INSERT INTO auth.authentication_events(event_type,outcome,request_id,object_type,object_id,change_summary) VALUES('login','success','schema-test','identity','00000000-0000-4000-8000-000000000099','{}')", "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := c.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			_, err = tx.Exec(ctx, tc.sql)
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != tc.code {
				t.Fatalf("expected constraint %s, got %v", tc.code, err)
			}
		})
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, sql := range []string{
		"INSERT INTO personnel.identities(name) VALUES('same'),('same')",
		"INSERT INTO personnel.permission_templates(name) VALUES('same'),('same')",
		"INSERT INTO personnel.departments(name,parent_id) VALUES('same',$1),('same',$1)",
	} {
		if sql[len(sql)-1] == '1' {
			t.Fatal("invalid fixture")
		}
		var err error
		if sql == "INSERT INTO personnel.departments(name,parent_id) VALUES('same',$1),('same',$1)" {
			_, err = tx.Exec(ctx, sql, root)
		} else {
			_, err = tx.Exec(ctx, sql)
		}
		if err != nil {
			t.Fatalf("Q25 duplicate display names allowed: %v", err)
		}
	}
}
