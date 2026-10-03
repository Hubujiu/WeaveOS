package applications

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestB5OperationsRejectNullConfirmedStatus(t *testing.T) {
	f := fixture(t, true)
	app, _ := f.create(t)
	ctx := context.Background()
	op := f.operation(t)
	_, err := f.runtime.Exec(ctx, "INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES($1,$2,$3,'group.create',decode(repeat('00',32),'hex'),'{}',NULL,'')", f.actor, op, app)
	t.Cleanup(func() {
		_, _ = f.owner.Exec(ctx, "DELETE FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2", f.actor, op)
	})
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23514" {
		t.Fatalf("confirmed operation requires non-null HTTP status; want CHECK 23514 got %v", err)
	}
}
