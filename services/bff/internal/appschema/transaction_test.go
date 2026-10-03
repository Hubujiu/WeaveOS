package appschema

import (
	"context"
	"testing"
)

func TestApplyInCallerTransactionCanRollbackDDLAndMetadata(t *testing.T) {
	f := newFixture(t, nil)
	tx, e := f.pool.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	r, e := f.executor.ApplyInTx(context.Background(), tx, f.request([]Field{{ID: fieldID1, Name: "x", Type: Text}}))
	if e != nil || r.Revision != "1" {
		t.Fatalf("caller tx apply missing %+v %v", r, e)
	}
	if e = tx.Rollback(context.Background()); e != nil {
		t.Fatal(e)
	}
	var present bool
	if e = f.pool.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", f.namespace+"."+tableName).Scan(&present); e != nil || present {
		t.Fatal("rollback must remove real DDL", present, e)
	}
}
