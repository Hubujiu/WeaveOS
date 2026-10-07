package appquery

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPGFullProjectionChangesOnlyWhenActualAuthorizedRowsChange(t *testing.T) {
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
	_, err = db.Exec(ctx, `CREATE TEMP TABLE v015_projection(id integer PRIMARY KEY, created_by text NOT NULL, visible text, hidden text)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO v015_projection VALUES(1,'u','one','s1'),(2,'u','two','s2'),(3,'other','three','s3')`)
	if err != nil {
		t.Fatal(err)
	}
	read := func() Projection {
		rows, err := db.Query(ctx, `SELECT jsonb_build_array(id,visible) FROM v015_projection WHERE created_by='u' ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		p, err := FingerprintRows(ctx, rows)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := read()
	if a.Total != 2 || a.Fingerprint == "" {
		t.Fatalf("initial projection %+v", a)
	}
	_, err = db.Exec(ctx, `UPDATE v015_projection SET hidden='changed' WHERE id=2`)
	if err != nil {
		t.Fatal(err)
	}
	if b := read(); b != a {
		t.Fatalf("hidden value changed authorized projection: %+v %+v", a, b)
	}
	_, err = db.Exec(ctx, `UPDATE v015_projection SET visible='changed' WHERE id=2`)
	if err != nil {
		t.Fatal(err)
	}
	beforeUnrelated := read()
	if beforeUnrelated.Total != a.Total || beforeUnrelated.Fingerprint == a.Fingerprint {
		t.Fatalf("off-page visible change missed: %+v %+v", a, beforeUnrelated)
	}
	_, err = db.Exec(ctx, `UPDATE v015_projection SET visible='new' WHERE id=3`)
	if err != nil {
		t.Fatal(err)
	}
	if afterUnrelated := read(); afterUnrelated != beforeUnrelated {
		t.Fatalf("other owner's edit changed authorized projection: before=%+v after=%+v", beforeUnrelated, afterUnrelated)
	}
}
