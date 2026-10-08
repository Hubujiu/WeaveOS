package apprecords

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

type concurrentGate struct{ schema string }

func (g concurrentGate) LockTable(ctx context.Context, tx pgx.Tx, id string, expected int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, id); err != nil {
		return err
	}
	var version int64
	var ready bool
	q := `SELECT schema_version,ready FROM ` + pgx.Identifier{g.schema, "v015_schema"}.Sanitize() + ` WHERE table_id=$1 FOR SHARE`
	if err := tx.QueryRow(ctx, q, id).Scan(&version, &ready); err != nil {
		return err
	}
	if !ready {
		return ErrNotReady
	}
	if version != expected {
		return ErrConflict
	}
	return nil
}

type concurrentFence struct{ schema string }

func (f concurrentFence) Check(ctx context.Context, tx pgx.Tx, table, id string, v int64) error {
	q := `SELECT EXISTS(SELECT 1 FROM ` + pgx.Identifier{f.schema, "v015_fence"}.Sanitize() + ` WHERE record_id=$1 AND pending)`
	var pending bool
	if err := tx.QueryRow(ctx, q, id).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrUnavailable
	}
	return nil
}

type concurrentAudit struct{ schema string }

func (a concurrentAudit) Append(ctx context.Context, tx pgx.Tx, r MutationResult, kind string, ids []string) error {
	q := `INSERT INTO ` + pgx.Identifier{a.schema, "v015_audit"}.Sanitize() + `(record_id,record_version,operation_id) VALUES($1,$2,$3)`
	_, err := tx.Exec(ctx, q, r.ID, r.RecordVersion, r.OperationID)
	return err
}

func TestTwoRealPGConnectionsCompeteOnRecordVersion(t *testing.T) {
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL 18 database required")
	}
	ctx := context.Background()
	owner, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(context.Background()); err != nil {
			t.Errorf("close owned fixture connection: %v", err)
		}
	})
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	schema := "v015c_" + hex.EncodeToString(raw)
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := owner.Exec(ctx, `CREATE SCHEMA `+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := owner.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`); err != nil {
			t.Errorf("drop owned fixture schema %s: %v", schema, err)
			return
		}
		var absent bool
		if err := owner.QueryRow(context.Background(), "SELECT NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname=$1)", schema).Scan(&absent); err != nil || !absent {
			t.Errorf("owned schema cleanup not verified: absent=%v err=%v", absent, err)
		}
	})
	tableName := pgx.Identifier{schema, "t_bbbbbbbbbbbb4bbb8bbbbbbbbbbbbbbb"}.Sanitize()
	commands := []string{
		`CREATE TABLE ` + tableName + `(id uuid PRIMARY KEY,created_by uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),record_version bigint NOT NULL DEFAULT 1,f_11111111111141118111111111111111 text)`,
		`CREATE TABLE ` + pgx.Identifier{schema, "v015_schema"}.Sanitize() + `(table_id uuid PRIMARY KEY,schema_version bigint NOT NULL,ready boolean NOT NULL)`,
		`CREATE TABLE ` + pgx.Identifier{schema, "v015_fence"}.Sanitize() + `(record_id uuid PRIMARY KEY,pending boolean NOT NULL)`,
		`CREATE TABLE ` + pgx.Identifier{schema, "v015_audit"}.Sanitize() + `(record_id uuid NOT NULL,record_version bigint NOT NULL,operation_id uuid NOT NULL)`,
	}
	for _, q := range commands {
		if _, err := owner.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := owner.Exec(ctx, `INSERT INTO `+pgx.Identifier{schema, "v015_schema"}.Sanitize()+` VALUES($1,1,true)`, tableID); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO `+tableName+`(id,created_by,f_11111111111141118111111111111111) VALUES($1,$2,'before')`, recordID, actorID); err != nil {
		t.Fatal(err)
	}
	w := Writer{Gate: concurrentGate{schema}, Authorization: allowField{}, Fence: concurrentFence{schema}, Audit: concurrentAudit{schema}, DML: sqlFixtureTypedDML{}}
	meta := Table{AppID: appID, TableID: tableID, ViewID: viewID, Namespace: schema, SchemaVersion: 1, Ready: true, ActiveFieldIDs: []string{fieldID}}
	start := make(chan struct{})
	out := make(chan error, 2)
	var wg sync.WaitGroup
	for i, op := range []string{"44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555"} {
		wg.Add(1)
		go func(i int, op string) {
			defer wg.Done()
			db, e := pgx.Connect(ctx, dsn)
			if e != nil {
				out <- e
				return
			}
			defer db.Close(ctx)
			tx, e := db.Begin(ctx)
			if e != nil {
				out <- e
				return
			}
			<-start
			_, e = w.EditInTx(ctx, tx, meta, Edit{OperationID: op, ID: recordID, ActorID: actorID, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{fieldID: string(rune('A' + i))}})
			if e != nil {
				_ = tx.Rollback(ctx)
				out <- e
				return
			}
			out <- tx.Commit(ctx)
		}(i, op)
	}
	close(start)
	wg.Wait()
	close(out)
	success, conflicts := 0, 0
	for e := range out {
		if e == nil {
			success++
		} else if errors.Is(e, ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected writer error %v", e)
		}
	}
	var version int64
	var audits int
	if err := owner.QueryRow(ctx, `SELECT record_version FROM `+tableName+` WHERE id=$1`, recordID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{schema, "v015_audit"}.Sanitize()).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if success != 1 || conflicts != 1 || version != 2 || audits != 1 {
		t.Fatalf("success=%d conflicts=%d version=%d audit=%d", success, conflicts, version, audits)
	}
}
