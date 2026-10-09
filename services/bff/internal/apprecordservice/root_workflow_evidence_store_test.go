package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These are storage contracts over synthetic scopes and typed-record fixtures.
// They do not replace live user authorization or authoritative capture tests.
type rootEvidenceStoreFixture struct {
	recordFixture
	header ev.Header
	fields []ev.Field
	bundle ev.Bundle
}

func rootEvidenceString(value string) json.RawMessage { raw, _ := json.Marshal(value); return raw }
func rootEvidenceStoreSetup(t *testing.T) rootEvidenceStoreFixture {
	t.Helper()
	f := newRecordFixture(t)
	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	t.Cleanup(cancel)
	f.ctx = ctx
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	var created, updated time.Time
	var version int64
	if e := f.owner.QueryRow(ctx, "SELECT created_at,updated_at,record_version FROM "+relation+" WHERE id=$1", f.ownRecord).Scan(&created, &updated, &version); e != nil {
		t.Fatal(e)
	}
	var label string
	if e := f.owner.QueryRow(ctx, "SELECT label FROM applications.member_sources WHERE id=$1", f.other).Scan(&label); e != nil {
		t.Fatal(e)
	}
	text := func(id, name, value string) ev.Field {
		return ev.Field{Definition: appfields.Field{ID: id, Name: name, Kind: "text", Default: json.RawMessage("null"), Config: json.RawMessage(`{"maxLength":null}`)}, Value: rootEvidenceString(value)}
	}
	fields := []ev.Field{text(f.public, "Public", "alpha"), text(f.secret, "Secret", "private alpha"), text(f.reference, "Member", f.other)}
	fields[2].Definition.Kind = "member"
	fields[2].Definition.Config = json.RawMessage(`{}`)
	fields[2].Reference = &ev.Reference{ID: f.other, Label: label}
	h := ev.Header{AppID: f.app, TableID: f.table, ViewID: f.view, RecordID: f.ownRecord, CreatedBy: f.actor, SchemaVersion: 1, RecordVersion: version, CreatedAt: created.UTC().Format(time.RFC3339Nano), UpdatedAt: updated.UTC().Format(time.RFC3339Nano)}
	b, e := ev.Build(h, fields, []string{f.public, f.reference})
	if e != nil {
		t.Fatal(e)
	}
	return rootEvidenceStoreFixture{f, h, fields, b}
}
func rootEvidenceStoreCounts(t *testing.T, f rootEvidenceStoreFixture, blobs, docs, members int) {
	t.Helper()
	for _, v := range []struct {
		name string
		want int
	}{{"workflow_evidence_blobs", blobs}, {"workflow_evidence_documents", docs}, {"workflow_evidence_members", members}} {
		var n int
		if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+v.name+" WHERE app_id=$1", f.app).Scan(&n); e != nil || n != v.want {
			t.Fatalf("%s count%d want%d: %v", v.name, n, v.want, e)
		}
	}
}
func rootEvidenceStorePut(t *testing.T, f rootEvidenceStoreFixture, b ev.Bundle) bool {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	created, e := (ev.Store{}).PutInTx(f.ctx, tx, b)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	return created
}
func rootEvidenceStoreRead(t *testing.T, f rootEvidenceStoreFixture, hash [32]byte, ids []string) (ev.Manifest, []ev.Field) {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	manifest, e := (ev.Store{}).ManifestInTx(f.ctx, tx, f.app, hash)
	if e != nil {
		t.Fatal(e)
	}
	fields, e := (ev.Store{}).FieldsInTx(f.ctx, tx, f.app, hash, ids)
	if e != nil {
		t.Fatal(e)
	}
	return manifest, fields
}
func rootEvidenceBlob(t *testing.T, b ev.Bundle, id string) ev.Blob {
	t.Helper()
	for _, v := range b.Fields {
		if v.FieldID == id {
			return v
		}
	}
	t.Fatal("missing synthetic field blob")
	return ev.Blob{}
}
func rootEvidenceCloneBundle(b ev.Bundle) ev.Bundle {
	out := b
	out.Manifest = bytes.Clone(b.Manifest)
	out.Fields = append([]ev.Blob(nil), b.Fields...)
	for i := range out.Fields {
		out.Fields[i].Body = bytes.Clone(out.Fields[i].Body)
	}
	return out
}

func TestRootEvidenceStoreRoundtripReplayAndSelectedFields(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	if !rootEvidenceStorePut(t, f, f.bundle) || rootEvidenceStorePut(t, f, f.bundle) {
		t.Fatal("first acceptance/replay identity wrong")
	}
	rootEvidenceStoreCounts(t, f, 3, 1, 3)
	m, fields := rootEvidenceStoreRead(t, f, f.bundle.Hash, []string{f.reference, f.public})
	raw, e := ev.EncodeManifest(m)
	if e != nil || !bytes.Equal(raw, f.bundle.Manifest) || len(fields) != 2 {
		t.Fatalf("immutable manifest/subset differs: %v", e)
	}
	ids := []string{f.reference, f.public}
	sort.Strings(ids)
	for i, v := range fields {
		body, e := ev.EncodeField(v)
		if e != nil || v.Definition.ID != ids[i] || !bytes.Equal(body, rootEvidenceBlob(t, f.bundle, ids[i]).Body) {
			t.Fatalf("selected body changed: %v", e)
		}
	}
	_, empty := rootEvidenceStoreRead(t, f, f.bundle.Hash, nil)
	if len(empty) != 0 {
		t.Fatal("empty mask expanded to all fields")
	}
	fields[0].Value[0] ^= 1
	f.bundle.Fields[0].Body[0] ^= 1
	f.bundle.Manifest[0] ^= 1
	m, again := rootEvidenceStoreRead(t, f, f.bundle.Hash, []string{f.reference, f.public})
	if m.Header != f.header || len(again) != 2 {
		t.Fatal("caller-owned bytes changed durable state")
	}
}
func TestRootEvidenceStoreOuterTransactionControlsVisibilityAndRollback(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	created, e := (ev.Store{}).PutInTx(f.ctx, tx, f.bundle)
	if e != nil || !created {
		t.Fatalf("put: %v", e)
	}
	rootEvidenceStoreCounts(t, f, 0, 0, 0)
	if e = tx.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	rootEvidenceStoreCounts(t, f, 0, 0, 0)
	if !rootEvidenceStorePut(t, f, f.bundle) {
		t.Fatal("rolled-back document remained claimed")
	}
	rootEvidenceStoreCounts(t, f, 3, 1, 3)
}
func TestRootEvidenceStoreChangedFieldReusesOtherContent(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	rootEvidenceStorePut(t, f, f.bundle)
	nextFields := append([]ev.Field(nil), f.fields...)
	nextFields[0].Value = rootEvidenceString("changed")
	h := f.header
	h.RecordVersion++
	next, e := ev.Build(h, nextFields, []string{f.public, f.reference})
	if e != nil {
		t.Fatal(e)
	}
	if !rootEvidenceStorePut(t, f, next) {
		t.Fatal("new evidence not stored")
	}
	rootEvidenceStoreCounts(t, f, 4, 2, 6)
	_, old := rootEvidenceStoreRead(t, f, f.bundle.Hash, []string{f.public})
	_, now := rootEvidenceStoreRead(t, f, next.Hash, []string{f.public})
	if string(old[0].Value) != `"alpha"` || string(now[0].Value) != `"changed"` {
		t.Fatal("new value rewrote historical evidence")
	}
	// Same record/schema versions can still differ by a reference display.
	changedRef := nextFields[2]
	r := *changedRef.Reference
	r.Label = "changed reference display"
	changedRef.Reference = &r
	nextFields[2] = changedRef
	refVersion, e := ev.Build(h, nextFields, []string{f.public, f.reference})
	if e != nil {
		t.Fatal(e)
	}
	rootEvidenceStorePut(t, f, refVersion)
	rootEvidenceStoreCounts(t, f, 5, 3, 9)
	// A visibility-only difference needs another manifest, not duplicate blobs.
	visible, e := ev.Build(h, nextFields, []string{f.public})
	if e != nil {
		t.Fatal(e)
	}
	rootEvidenceStorePut(t, f, visible)
	rootEvidenceStoreCounts(t, f, 5, 4, 12)
}
func TestRootEvidenceStoreSavepointRollsBackEveryPartialStage(t *testing.T) {
	for _, table := range []string{"workflow_evidence_blobs", "workflow_evidence_documents", "workflow_evidence_members"} {
		t.Run(table, func(t *testing.T) {
			f := rootEvidenceStoreSetup(t)
			fn := "root_evidence_" + strings.ReplaceAll(f.app, "-", "")
			q := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root evidence storage fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER INSERT ON applications.%s FOR EACH ROW WHEN (NEW.app_id='%s'::uuid) EXECUTE FUNCTION applications.%s()", fn, fn, table, f.app, fn)
			if _, e := f.owner.Exec(f.ctx, q); e != nil {
				t.Fatal(e)
			}
			cleanup := func() {
				_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON applications.%s; DROP FUNCTION IF EXISTS applications.%s()", fn, table, fn))
			}
			t.Cleanup(cleanup)
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			created, e := (ev.Store{}).PutInTx(f.ctx, tx, f.bundle)
			var fault *pgconn.PgError
			if created || !errors.As(e, &fault) || fault.Code != "P0001" || !strings.Contains(fault.Message, "root evidence storage fault") {
				t.Fatalf("actual %s failure not reached: %v", table, e)
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatalf("savepoint did not restore caller transaction: %v", e)
			}
			rootEvidenceStoreCounts(t, f, 0, 0, 0)
			cleanup()
			if !rootEvidenceStorePut(t, f, f.bundle) {
				t.Fatal("failed evidence left an irreversible claim")
			}
		})
	}
}
func TestRootEvidenceStoreInvalidBundlesNeverWriteOrPoisonCaller(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	for _, name := range []string{"manifest-hash", "body-hash", "wrong-field", "missing-blob", "duplicate-blob", "noncanonical-manifest"} {
		t.Run(name, func(t *testing.T) {
			b := rootEvidenceCloneBundle(f.bundle)
			switch name {
			case "manifest-hash":
				b.Hash[0] ^= 1
			case "body-hash":
				b.Fields[0].Hash[0] ^= 1
			case "wrong-field":
				b.Fields[0].FieldID = recordOperationID(t, f.recordFixture)
			case "missing-blob":
				b.Fields = b.Fields[:2]
			case "duplicate-blob":
				b.Fields[2] = b.Fields[0]
			case "noncanonical-manifest":
				b.Manifest = append(b.Manifest, ' ')
				b.Hash = sha256.Sum256(b.Manifest)
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			created, e := (ev.Store{}).PutInTx(f.ctx, tx, b)
			if created || !errors.Is(e, ev.ErrInvalid) {
				t.Fatalf("invalid bundle accepted: %v", e)
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatalf("invalid input poisoned caller: %v", e)
			}
			rootEvidenceStoreCounts(t, f, 0, 0, 0)
		})
	}
}
func TestRootEvidenceStoreScopeAndForeignResourceBinding(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	other := rootEvidenceStoreSetup(t)
	rootEvidenceStorePut(t, f, f.bundle)
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = (ev.Store{}).ManifestInTx(f.ctx, tx, other.app, f.bundle.Hash); !errors.Is(e, ev.ErrMissing) {
		t.Fatalf("cross-app manifest leaked: %v", e)
	}
	if _, e = (ev.Store{}).FieldsInTx(f.ctx, tx, other.app, f.bundle.Hash, nil); !errors.Is(e, ev.ErrMissing) {
		t.Fatalf("empty projection accepted foreign evidence: %v", e)
	}
	h := f.header
	h.AppID = other.app
	forged, e := ev.Build(h, f.fields, []string{f.public})
	if e != nil {
		t.Fatal(e)
	}
	created, e := (ev.Store{}).PutInTx(f.ctx, tx, forged)
	var foreign *pgconn.PgError
	if created || !errors.As(e, &foreign) || foreign.Code != "23503" {
		t.Fatalf("real app/table/view FK was bypassed: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	rootEvidenceStoreCounts(t, other, 0, 0, 0)
	rootEvidenceStoreCounts(t, f, 3, 1, 3)
}
func TestRootEvidenceStoreCorruptionFailsClosedAndIsNotSilentlyRepaired(t *testing.T) {
	for _, kind := range []string{"manifest-bytes", "metadata", "member", "field-bytes"} {
		t.Run(kind, func(t *testing.T) {
			f := rootEvidenceStoreSetup(t)
			rootEvidenceStorePut(t, f, f.bundle)
			switch kind {
			case "manifest-bytes":
				body := bytes.Replace(f.bundle.Manifest, []byte(`"recordVersion":1`), []byte(`"recordVersion":2`), 1)
				if bytes.Equal(body, f.bundle.Manifest) {
					t.Fatal("fixture replacement missed")
				}
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_evidence_documents SET body=$3 WHERE app_id=$1 AND evidence_hash=$2", f.app, f.bundle.Hash[:], body); e != nil {
					t.Fatal(e)
				}
			case "metadata":
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_evidence_documents SET record_version=2 WHERE app_id=$1 AND evidence_hash=$2", f.app, f.bundle.Hash[:]); e != nil {
					t.Fatal(e)
				}
			case "member":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.workflow_evidence_members WHERE app_id=$1 AND evidence_hash=$2 AND field_id=$3", f.app, f.bundle.Hash[:], f.public); e != nil {
					t.Fatal(e)
				}
			case "field-bytes":
				blob := rootEvidenceBlob(t, f.bundle, f.public)
				body := bytes.Replace(blob.Body, []byte(`"alpha"`), []byte(`"other"`), 1)
				if bytes.Equal(body, blob.Body) {
					t.Fatal("fixture replacement missed")
				}
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_evidence_blobs SET body=$4 WHERE app_id=$1 AND field_id=$2 AND content_hash=$3", f.app, f.public, blob.Hash[:], body); e != nil {
					t.Fatal(e)
				}
			}
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			if kind != "field-bytes" {
				if m, e := (ev.Store{}).ManifestInTx(f.ctx, tx, f.app, f.bundle.Hash); !errors.Is(e, ev.ErrConflict) || len(m.Fields) != 0 {
					t.Fatalf("corrupt manifest returned partial evidence: %v", e)
				}
			}
			got, e := (ev.Store{}).FieldsInTx(f.ctx, tx, f.app, f.bundle.Hash, []string{f.public, f.secret})
			if !errors.Is(e, ev.ErrConflict) || len(got) != 0 {
				t.Fatalf("corrupt body returned partial fields: %v", e)
			}
			if created, e := (ev.Store{}).PutInTx(f.ctx, tx, f.bundle); created || !errors.Is(e, ev.ErrConflict) {
				t.Fatalf("existing corrupt document was silently repaired: %v", e)
			}
			if e = tx.Commit(f.ctx); e != nil {
				t.Fatal(e)
			}
			count := 3
			if kind == "member" {
				count = 2
			}
			rootEvidenceStoreCounts(t, f, 3, 1, count)
		})
	}
}
func TestRootEvidenceStoreBlobCollisionCannotOverwriteEarlierBytes(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	blob := rootEvidenceBlob(t, f.bundle, f.public)
	poison := bytes.Replace(blob.Body, []byte(`"alpha"`), []byte(`"other"`), 1)
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.workflow_evidence_blobs(app_id,field_id,content_hash,body) VALUES($1,$2,$3,$4)", f.app, f.public, blob.Hash[:], poison); e != nil {
		t.Fatal(e)
	}
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	created, e := (ev.Store{}).PutInTx(f.ctx, tx, f.bundle)
	if created || !errors.Is(e, ev.ErrConflict) {
		t.Fatalf("same-key different bytes accepted: %v", e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	rootEvidenceStoreCounts(t, f, 1, 0, 0)
	var body []byte
	if e = f.owner.QueryRow(f.ctx, "SELECT body FROM applications.workflow_evidence_blobs WHERE app_id=$1 AND field_id=$2 AND content_hash=$3", f.app, f.public, blob.Hash[:]).Scan(&body); e != nil || !bytes.Equal(body, poison) {
		t.Fatal("earlier bytes overwritten")
	}
}
func TestRootEvidenceStoreConcurrentFirstAcceptanceHasOneDocument(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	const writers = 16
	var wg sync.WaitGroup
	errorsOut := make(chan error, writers)
	createdOut := make(chan bool, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, e := f.runtime.Begin(f.ctx)
			if e != nil {
				errorsOut <- e
				return
			}
			defer tx.Rollback(context.Background())
			created, e := (ev.Store{}).PutInTx(f.ctx, tx, f.bundle)
			if e == nil {
				e = tx.Commit(f.ctx)
			}
			errorsOut <- e
			createdOut <- created
		}()
	}
	wg.Wait()
	close(errorsOut)
	close(createdOut)
	for e := range errorsOut {
		if e != nil {
			t.Fatal(e)
		}
	}
	n := 0
	for created := range createdOut {
		if created {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("first-insert count=%d", n)
	}
	rootEvidenceStoreCounts(t, f, 3, 1, 3)
}
func TestRootEvidenceStoreHistoryDoesNotJoinCurrentValuesOrReferenceLabels(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	rootEvidenceStorePut(t, f, f.bundle)
	oldLabel := f.fields[2].Reference.Label
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.public, "-", "")}.Sanitize()
	if _, e := f.owner.Exec(f.ctx, "UPDATE "+relation+" SET "+column+"='current changed',record_version=2 WHERE id=$1", f.ownRecord); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET account='renamed-'||id::text,status='disabled' WHERE id=$1", f.other); e != nil {
		t.Fatal(e)
	}
	m, fields := rootEvidenceStoreRead(t, f, f.bundle.Hash, []string{f.public, f.reference})
	gotIDs := make([]string, len(fields))
	for i, field := range fields {
		gotIDs[i] = field.Definition.ID
	}
	wantIDs := []string{f.public, f.reference}
	sort.Strings(gotIDs)
	sort.Strings(wantIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("complete historical selection got %v, want %v", gotIDs, wantIDs)
	}
	if m.Header.RecordVersion != 1 {
		t.Fatal("history followed current record version")
	}
	for _, field := range fields {
		switch field.Definition.ID {
		case f.public:
			if string(field.Value) != `"alpha"` {
				t.Fatal("history followed current value")
			}
		case f.reference:
			if field.Reference == nil || field.Reference.Label != oldLabel || field.Reference.Deleted {
				t.Fatal("history rejoined changed current source")
			}
		}
	}
	// Owner-only synthetic schema mutation proves independence, not schema Save authorization.
	if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND table_id=$2 AND field_id=$3", f.app, f.table, f.secret); e != nil {
		t.Fatal(e)
	}
	secret := pgx.Identifier{"f_" + strings.ReplaceAll(f.secret, "-", "")}.Sanitize()
	if _, e := f.owner.Exec(f.ctx, "ALTER TABLE "+relation+" DROP COLUMN "+secret); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.fields SET removed=true WHERE app_id=$1 AND id=$2", f.app, f.secret); e != nil {
		t.Fatal(e)
	}
	_, old := rootEvidenceStoreRead(t, f, f.bundle.Hash, []string{f.secret})
	if len(old) != 1 || string(old[0].Value) != `"private alpha"` || old[0].Definition.Name != "Secret" {
		t.Fatal("removed current column erased historical field")
	}
}
func TestRootEvidenceStoreInvalidPortsMissingAndUnknownSelection(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	var typed *pgxpool.Tx
	for _, tx := range []pgx.Tx{nil, typed} {
		if _, e := (ev.Store{}).PutInTx(f.ctx, tx, f.bundle); !errors.Is(e, ev.ErrInvalid) {
			t.Fatalf("nil tx: %v", e)
		}
	}
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = (ev.Store{}).PutInTx(nil, tx, f.bundle); !errors.Is(e, ev.ErrInvalid) {
		t.Fatalf("nil context: %v", e)
	}
	if _, e = (ev.Store{}).ManifestInTx(f.ctx, tx, f.app, f.bundle.Hash); !errors.Is(e, ev.ErrMissing) {
		t.Fatalf("missing evidence: %v", e)
	}
	if _, e = (ev.Store{}).FieldsInTx(f.ctx, tx, f.app, f.bundle.Hash, nil); !errors.Is(e, ev.ErrMissing) {
		t.Fatalf("missing empty projection: %v", e)
	}
	if _, e = (ev.Store{}).PutInTx(f.ctx, tx, f.bundle); e != nil {
		t.Fatal(e)
	}
	for _, ids := range [][]string{{recordOperationID(t, f.recordFixture)}, {f.public, f.public}, {"bad"}} {
		if _, e = (ev.Store{}).FieldsInTx(f.ctx, tx, f.app, f.bundle.Hash, ids); !errors.Is(e, ev.ErrInvalid) {
			t.Fatalf("unknown/duplicate selection: %v", e)
		}
	}
	if _, e = (ev.Store{}).ManifestInTx(f.ctx, tx, "bad", f.bundle.Hash); !errors.Is(e, ev.ErrInvalid) {
		t.Fatalf("invalid scope: %v", e)
	}
}
func TestRootEvidenceStoreRolesAreAppendOnlyAndBackupReadOnly(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	for _, table := range []string{"workflow_evidence_blobs", "workflow_evidence_documents", "workflow_evidence_members"} {
		for _, role := range []string{"auth_app", "auth_backup", "auth_reader", "auth_maintenance"} {
			for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE"} {
				want := role == "auth_app" && (privilege == "SELECT" || privilege == "INSERT") || role == "auth_backup" && privilege == "SELECT"
				var got bool
				if e := f.owner.QueryRow(f.ctx, "SELECT has_table_privilege($1,$2,$3)", role, "applications."+table, privilege).Scan(&got); e != nil || got != want {
					t.Fatalf("%s %s %s=%v want%v: %v", role, table, privilege, got, want, e)
				}
			}
		}
	}
}
func TestRootEvidenceStoreDownRefusesExistingHistory(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	rootEvidenceStorePut(t, f, f.bundle)
	raw, e := os.ReadFile("../../../../db/migrations/00020_workflow_evidence.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("migration Down section absent")
	}
	_, e = f.owner.Exec(f.ctx, parts[1])
	var refused *pgconn.PgError
	if !errors.As(e, &refused) || refused.Code != "55000" {
		t.Fatalf("nonempty Down was not refused: %v", e)
	}
	rootEvidenceStoreCounts(t, f, 3, 1, 3)
	m, _ := rootEvidenceStoreRead(t, f, f.bundle.Hash, nil)
	want, e := ev.DecodeManifest(f.bundle.Manifest)
	if e != nil || !reflect.DeepEqual(m, want) {
		t.Fatal("failed Down altered immutable evidence")
	}
}

// Count statements, including SendBatch statements, rather than timing noisy CI.
type rootEvidenceQueryTrace struct{ count atomic.Int32 }

func (r *rootEvidenceQueryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	r.count.Add(1)
	return ctx
}
func (*rootEvidenceQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (*rootEvidenceQueryTrace) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return ctx
}
func (r *rootEvidenceQueryTrace) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {
	r.count.Add(1)
}
func (*rootEvidenceQueryTrace) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}
func TestRootEvidenceStoreQueriesAreBoundedRatherThanPerField(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	const n = 64
	fields := make([]ev.Field, n)
	ids := make([]string, n)
	for i := range fields {
		ids[i] = recordOperationID(t, f.recordFixture)
		fields[i] = ev.Field{Definition: appfields.Field{ID: ids[i], Name: fmt.Sprintf("Field %02d", i), Kind: "text", Default: json.RawMessage("null"), Config: json.RawMessage(`{"maxLength":null}`)}, Value: rootEvidenceString(fmt.Sprintf("value-%02d", i))}
	}
	bundle, e := ev.Build(f.header, fields, ids)
	if e != nil {
		t.Fatal(e)
	}
	trace := new(rootEvidenceQueryTrace)
	cfg := f.runtime.Config()
	cfg.ConnConfig.Tracer = trace
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if e = pool.Ping(f.ctx); e != nil {
		t.Fatal(e)
	}
	tx, e := pool.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	trace.count.Store(0)
	created, e := (ev.Store{}).PutInTx(f.ctx, tx, bundle)
	writes := trace.count.Load()
	if e != nil || !created {
		t.Fatalf("64-field put: %v", e)
	}
	if writes > 12 {
		t.Fatalf("put used %d statements for 64 fields; per-field SQL is forbidden", writes)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	tx, e = pool.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	trace.count.Store(0)
	got, e := (ev.Store{}).FieldsInTx(f.ctx, tx, f.app, bundle.Hash, ids)
	reads := trace.count.Load()
	if e != nil || len(got) != n {
		t.Fatalf("64-field reconstruction: %v", e)
	}
	if reads > 4 {
		t.Fatalf("read used %d statements for 64 fields; per-field SQL is forbidden", reads)
	}
	t.Logf("64 fields: put=%d SQL statements; read=%d SQL statements", writes, reads)
}
