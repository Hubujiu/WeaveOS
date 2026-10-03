package apprecords

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const appID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const tableID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const viewID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
const actorID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
const recordID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
const fieldID = "11111111-1111-4111-8111-111111111111"

type sqlGate struct{}

func (sqlGate) LockTable(ctx context.Context, tx pgx.Tx, id string, expected int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, id); err != nil {
		return err
	}
	var current int64
	var ready bool
	if err := tx.QueryRow(ctx, `SELECT schema_version,ready FROM pg_temp.v015_schema WHERE table_id=$1 FOR SHARE`, id).Scan(&current, &ready); err != nil {
		return err
	}
	if !ready {
		return ErrNotReady
	}
	if current != expected {
		return ErrConflict
	}
	return nil
}

type allowField struct{}

func (allowField) Check(ctx context.Context, tx pgx.Tx, action, createdBy string, ids []string) error {
	for _, id := range ids {
		if id != fieldID {
			return ErrInvalid
		}
	}
	return nil
}

type sqlFence struct{}

func (sqlFence) Check(ctx context.Context, tx pgx.Tx, table, id string, version int64) error {
	var pending bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_temp.v015_fence WHERE record_id=$1 AND pending)`, id).Scan(&pending)
	if err != nil {
		return err
	}
	if pending {
		return ErrUnavailable
	}
	return nil
}

type sqlAudit struct{}

func (sqlAudit) Append(ctx context.Context, tx pgx.Tx, result MutationResult, kind string, ids []string) error {
	_, err := tx.Exec(ctx, `INSERT INTO pg_temp.v015_audit(record_id,record_version,operation_id,kind) VALUES($1,$2,$3,$4)`, result.ID, result.RecordVersion, result.OperationID, kind)
	return err
}

// Owner-role PostgreSQL fixture only. The runtime V013 adapter must call a
// reviewed controlled typed-DML capability and must not copy this direct SQL.
type sqlFixtureTypedDML struct{}

func fixtureTable(t Table) string {
	return pgx.Identifier{t.Namespace, "t_" + strings.ReplaceAll(t.TableID, "-", "")}.Sanitize()
}
func fixtureColumn(id string) string {
	return pgx.Identifier{"f_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
}
func (sqlFixtureTypedDML) Insert(ctx context.Context, tx pgx.Tx, table Table, in Create, ids []string) (StoredHeader, error) {
	cols := []string{"id", "created_by"}
	args := []any{in.ID, in.ActorID}
	for _, id := range ids {
		cols = append(cols, fixtureColumn(id))
		args = append(args, in.Values[id])
	}
	binds := make([]string, len(args))
	for i := range binds {
		binds[i] = fmt.Sprintf("$%d", i+1)
	}
	q := `INSERT INTO ` + fixtureTable(table) + `(` + strings.Join(cols, ",") + `) VALUES(` + strings.Join(binds, ",") + `) RETURNING id::text,created_by::text,record_version,created_at,updated_at`
	var out StoredHeader
	err := tx.QueryRow(ctx, q, args...).Scan(&out.ID, &out.CreatedBy, &out.RecordVersion, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}
func (sqlFixtureTypedDML) LockHeader(ctx context.Context, tx pgx.Tx, table Table, id string) (StoredHeader, error) {
	var out StoredHeader
	q := `SELECT id::text,created_by::text,record_version,created_at,updated_at FROM ` + fixtureTable(table) + ` WHERE id=$1 FOR UPDATE`
	err := tx.QueryRow(ctx, q, id).Scan(&out.ID, &out.CreatedBy, &out.RecordVersion, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}
func (sqlFixtureTypedDML) UpdateCAS(ctx context.Context, tx pgx.Tx, table Table, in Edit, ids []string) (StoredHeader, error) {
	sets := make([]string, 0, len(ids)+2)
	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		args = append(args, in.Changes[id])
		sets = append(sets, fmt.Sprintf("%s=$%d", fixtureColumn(id), len(args)))
	}
	sets = append(sets, "record_version=record_version+1", "updated_at=now()")
	args = append(args, in.ID, in.ExpectedRecordVersion)
	q := `UPDATE ` + fixtureTable(table) + ` SET ` + strings.Join(sets, ",") + fmt.Sprintf(` WHERE id=$%d AND record_version=$%d RETURNING id::text,created_by::text,record_version,created_at,updated_at`, len(args)-1, len(args))
	var out StoredHeader
	err := tx.QueryRow(ctx, q, args...).Scan(&out.ID, &out.CreatedBy, &out.RecordVersion, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func TestTypedPGCreateEditCASFenceAndRollback(t *testing.T) {
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL 18 database required")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	_, err = db.Exec(ctx, `CREATE TEMP TABLE t_bbbbbbbbbbbb4bbb8bbbbbbbbbbbbbbb(id uuid PRIMARY KEY,created_by uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),record_version bigint NOT NULL DEFAULT 1,f_11111111111141118111111111111111 text)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_fence(record_id uuid PRIMARY KEY,pending boolean NOT NULL);CREATE TEMP TABLE v015_audit(record_id uuid NOT NULL,record_version bigint NOT NULL,operation_id uuid NOT NULL,kind text NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_schema(table_id uuid PRIMARY KEY,schema_version bigint NOT NULL,ready boolean NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO v015_schema VALUES($1,1,true)`, tableID)
	if err != nil {
		t.Fatal(err)
	}
	w := Writer{Gate: sqlGate{}, Authorization: allowField{}, Fence: sqlFence{}, Audit: sqlAudit{}}
	table := Table{AppID: appID, TableID: tableID, ViewID: viewID, Namespace: "pg_temp", SchemaVersion: 1, Ready: true, ActiveFieldIDs: []string{fieldID}}
	op1 := "22222222-2222-4222-8222-222222222222"
	op2 := "33333333-3333-4333-8333-333333333333"
	// The runtime role cannot directly INSERT/UPDATE a typed business table.
	// A missing controlled DML port must fail before touching any row.
	withoutPort, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.CreateInTx(ctx, withoutPort, table, Create{OperationID: op1, ID: recordID, ActorID: actorID, ExpectedSchemaVersion: 1, Values: map[string]any{fieldID: "probe"}})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing controlled typed-DML port executed business write: %v", err)
	}
	_ = withoutPort.Rollback(ctx)
	w.DML = sqlFixtureTypedDML{}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := w.CreateInTx(ctx, tx, table, Create{OperationID: op1, ID: recordID, ActorID: actorID, ExpectedSchemaVersion: 1, Values: map[string]any{fieldID: "one"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.RecordVersion != 1 || created.SchemaVersion != 1 {
		t.Fatalf("create %+v", created)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	edited, err := w.EditInTx(ctx, tx, table, Edit{OperationID: op2, ID: recordID, ActorID: actorID, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{fieldID: "two"}})
	if err != nil {
		t.Fatal(err)
	}
	if edited.RecordVersion != 2 {
		t.Fatalf("edit %+v", edited)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.EditInTx(ctx, tx, table, Edit{OperationID: op2, ID: recordID, ActorID: actorID, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{fieldID: "stale"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale CAS %v", err)
	}
	_ = tx.Rollback(ctx)
	_, err = db.Exec(ctx, `INSERT INTO v015_fence VALUES($1,true)`, recordID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.EditInTx(ctx, tx, table, Edit{OperationID: op2, ID: recordID, ActorID: actorID, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 2, Changes: map[string]any{fieldID: "fenced"}})
	if err == nil {
		t.Fatal("pending approval command passed")
	}
	_ = tx.Rollback(ctx)
	_, err = db.Exec(ctx, `DELETE FROM v015_fence WHERE record_id=$1`, recordID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `UPDATE v015_schema SET schema_version=2 WHERE table_id=$1`, tableID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.EditInTx(ctx, tx, table, Edit{OperationID: op2, ID: recordID, ActorID: actorID, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 2, Changes: map[string]any{fieldID: "stale schema"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("gate missed concurrent schema change: %v", err)
	}
	_ = tx.Rollback(ctx)
	var value string
	var version, n int64
	err = db.QueryRow(ctx, `SELECT f_11111111111141118111111111111111,record_version FROM t_bbbbbbbbbbbb4bbb8bbbbbbbbbbbbbbb WHERE id=$1`, recordID).Scan(&value, &version)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(ctx, `SELECT count(*) FROM v015_audit`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if value != "two" || version != 2 || n != 2 {
		t.Fatalf("partial write value=%s version=%d audit=%d", value, version, n)
	}
}
