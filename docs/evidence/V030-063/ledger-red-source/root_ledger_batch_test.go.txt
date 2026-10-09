package flowcommands

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestRootLedgerBatchExactMixedEntries(t *testing.T) {
	f := rootLedgerFixture(t)
	a := rootV2Command()
	b := a
	b.CommandID = otherID
	f.accept(t, a)
	f.accept(t, b)
	tx := f.begin(t)
	calls := 0
	r := rootReceipt(t, a)
	if _, e := f.ledger.ApplyInTx(f.ctx, tx, a, r, 11, f.apply(a, &calls, false)); e != nil {
		t.Fatal(e)
	}
	if e := tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	tx = f.begin(t)
	got, e := f.ledger.GetManyInTx(f.ctx, tx, []string{b.CommandID, a.CommandID})
	if e != nil || len(got) != 2 || got[a.CommandID].Command != a || got[a.CommandID].Receipt == nil || *got[a.CommandID].Receipt != r || got[b.CommandID].Command != b || got[b.CommandID].State != "pending" || got[b.CommandID].Receipt != nil {
		t.Fatalf("batch did not preserve independently known entries: %+v %v", got, e)
	}
}
func TestRootLedgerBatchMissingAndCorruptFailWholeBatch(t *testing.T) {
	for _, kind := range []string{"missing", "hash", "receipt"} {
		t.Run(kind, func(t *testing.T) {
			f := rootLedgerFixture(t)
			a := rootV2Command()
			b := a
			b.CommandID = otherID
			f.accept(t, a)
			if kind != "missing" {
				f.accept(t, b)
			}
			if kind == "hash" {
				if _, e := f.db.Exec(f.ctx, "UPDATE pg_temp.workflow_commands SET command_hash=decode(repeat('00',32),'hex') WHERE command_id=$1", b.CommandID); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "receipt" {
				if _, e := f.db.Exec(f.ctx, "UPDATE pg_temp.workflow_commands SET state='success',receipt_json='{}'::jsonb WHERE command_id=$1", b.CommandID); e != nil {
					t.Fatal(e)
				}
			}
			got, e := f.ledger.GetManyInTx(f.ctx, f.begin(t), []string{a.CommandID, b.CommandID})
			if e == nil || len(got) != 0 {
				t.Fatalf("partial batch accepted %s: %+v %v", kind, got, e)
			}
			if kind == "missing" && !errors.Is(e, ErrMissing) {
				t.Fatal("wrong missing semantics", e)
			}
		})
	}
}
func TestRootLedgerBatchBoundedInputAndEmpty(t *testing.T) {
	f := rootLedgerFixture(t)
	tx := f.begin(t)
	got, e := f.ledger.GetManyInTx(f.ctx, tx, nil)
	if e != nil || got == nil || len(got) != 0 {
		t.Fatal("empty bounded set must be empty result", e)
	}
	a := rootV2Command()
	ids := []string{}
	for i := 0; i < 102; i++ {
		ids = append(ids, fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1))
	}
	for _, bad := range [][]string{{"wrong"}, {a.CommandID, a.CommandID}, ids} {
		if _, e = f.ledger.GetManyInTx(f.ctx, tx, bad); !errors.Is(e, ErrInvalid) {
			t.Fatal("unbounded/duplicate/invalid set accepted", e)
		}
	}
	if _, e = f.ledger.GetManyInTx(context.Background(), nil, nil); !errors.Is(e, ErrInvalid) {
		t.Fatal("nil port accepted", e)
	}
}
