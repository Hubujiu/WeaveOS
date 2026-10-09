package persistence_test

import (
	"context"
	"testing"
)

func TestWorkflowTriggerConfigurationColumnContract(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	var count int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='applications' AND table_name='workflow_versions' AND column_name='triggers_json' AND data_type='jsonb' AND is_nullable='NO' AND column_default='''[]''::jsonb'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("versioned trigger configuration column/default/required contract is missing")
	}
	var canRead, canInsert, canUpdate bool
	if err := conn.QueryRow(ctx, `SELECT has_column_privilege('auth_app','applications.workflow_versions','triggers_json','SELECT'),has_column_privilege('auth_app','applications.workflow_versions','triggers_json','INSERT'),has_column_privilege('auth_app','applications.workflow_versions','triggers_json','UPDATE')`).Scan(&canRead, &canInsert, &canUpdate); err != nil {
		t.Fatal(err)
	}
	if !canRead || !canInsert || canUpdate {
		t.Fatalf("immutable version permissions violated: read=%v insert=%v update=%v", canRead, canInsert, canUpdate)
	}
}
