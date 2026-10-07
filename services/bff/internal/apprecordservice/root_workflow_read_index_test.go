package apprecordservice

import (
	"context"
	"github.com/jackc/pgx/v5"
	"os"
	"strings"
	"testing"
)

func TestRootWorkflowReadScopedOrderIndexIsReady(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	c, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close(ctx)
	var definition string
	var valid, ready bool
	e = c.QueryRow(ctx, `SELECT pg_get_indexdef(i.indexrelid),i.indisvalid,i.indisready FROM pg_index i JOIN pg_class ix ON ix.oid=i.indexrelid JOIN pg_class rel ON rel.oid=i.indrelid JOIN pg_namespace n ON n.oid=rel.relnamespace WHERE n.nspname='applications' AND rel.relname='workflow_instances' AND ix.relname='ix_workflow_instances_record_order'`).Scan(&definition, &valid, &ready)
	if e != nil {
		t.Fatalf("record-scoped stable-order lookup requires its deployed index: %v", e)
	}
	if !valid || !ready || !strings.Contains(definition, "USING btree (app_id, table_id, record_id, created_at DESC, id DESC)") || strings.Contains(definition, " WHERE ") {
		t.Fatalf("wrong or partial scoped index: ready=%t valid=%t %s", ready, valid, definition)
	}
}
