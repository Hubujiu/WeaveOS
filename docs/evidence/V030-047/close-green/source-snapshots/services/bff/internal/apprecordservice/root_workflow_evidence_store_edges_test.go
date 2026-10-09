package apprecordservice

import (
	"context"
	"errors"
	"testing"

	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
)

func TestRootEvidenceStoreEmptyFormHasImmutableDirectory(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	b, e := ev.Build(f.header, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if !rootEvidenceStorePut(t, f, b) || rootEvidenceStorePut(t, f, b) {
		t.Fatal("empty directory replay identity changed")
	}
	rootEvidenceStoreCounts(t, f, 0, 1, 0)
	m, fields := rootEvidenceStoreRead(t, f, b.Hash, nil)
	if m.Header != f.header || len(m.Fields) != 0 || len(fields) != 0 {
		t.Fatal("empty form did not reconstruct")
	}
}
func TestRootEvidenceStoreResourceBudgetRejectsBeforeSQL(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	for _, kind := range []string{"manifest", "field-count", "field-body", "aggregate"} {
		t.Run(kind, func(t *testing.T) {
			b := rootEvidenceCloneBundle(f.bundle)
			switch kind {
			case "manifest":
				b.Manifest = make([]byte, ev.MaxManifestBytes+1)
			case "field-count":
				b.Fields = make([]ev.Blob, ev.MaxFields+1)
			case "field-body":
				b.Fields[0].Body = make([]byte, ev.MaxFieldBytes+1)
			case "aggregate":
				// Shared caller backing bytes avoid manufacturing a 68 MiB fixture;
				// the encoding budget still counts every declared body's contribution.
				body := make([]byte, ev.MaxFieldBytes)
				b.Fields = make([]ev.Blob, ev.MaxBundleBytes/ev.MaxFieldBytes+1)
				for i := range b.Fields {
					b.Fields[i] = ev.Blob{Body: body}
				}
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			created, e := (ev.Store{}).PutInTx(f.ctx, tx, b)
			if created || !errors.Is(e, ev.ErrTooLarge) {
				t.Fatalf("%s budget error=%v", kind, e)
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatalf("budget check poisoned caller: %v", e)
			}
			rootEvidenceStoreCounts(t, f, 0, 0, 0)
		})
	}
}
