package apprecordservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rootCaptureSetup(t *testing.T) rootEvidenceStoreFixture {
	t.Helper()
	f := rootEvidenceStoreSetup(t)
	definitions := make([]appfields.Field, len(f.fields))
	for i, field := range f.fields {
		definitions[i] = field.Definition
	}
	rootCaptureDefinitions(t, f, definitions)
	return f
}
func rootCaptureDefinitions(t *testing.T, f rootEvidenceStoreFixture, definitions []appfields.Field) {
	t.Helper()
	for _, field := range definitions {
		raw, e := json.Marshal(field)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.owner.Exec(f.ctx, "UPDATE applications.fields SET definition=$4 WHERE app_id=$1 AND table_id=$2 AND id=$3", f.app, f.table, field.ID, raw); e != nil {
			t.Fatal(e)
		}
	}
	raw, e := json.Marshal(definitions)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=$3 WHERE app_id=$1 AND id=$2", f.app, f.table, raw); e != nil {
		t.Fatal(e)
	}
}
func rootCaptureRead(t *testing.T, f rootEvidenceStoreFixture, principal session.Principal, id string) ev.Bundle {
	t.Helper()
	tx, facts, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	b, e := captureWorkflowEvidence(f.ctx, tx, facts, id)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	return b
}
func rootCapturedFields(t *testing.T, b ev.Bundle) map[string]ev.Field {
	t.Helper()
	out := map[string]ev.Field{}
	for _, blob := range b.Fields {
		f, e := ev.DecodeField(blob.Body)
		if e != nil {
			t.Fatal(e)
		}
		out[blob.FieldID] = f
	}
	return out
}
func rootCaptureColumn(id string) string {
	return pgx.Identifier{"f_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
}
func rootCaptureRelation(f rootEvidenceStoreFixture) string {
	return pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
}
func rootCaptureAddField(t *testing.T, f rootEvidenceStoreFixture, field appfields.Field, physical map[string]any) {
	t.Helper()
	raw, e := json.Marshal(field)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "INSERT INTO applications.fields(id,app_id,table_id,definition) VALUES($1,$2,$3,$4)", field.ID, f.app, f.table, raw); e != nil {
		t.Fatal(e)
	}
	physical["ID"], physical["Required"] = field.ID, false
	p, e := json.Marshal(physical)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "SELECT applications.apply_schema_change($1,$2,$3,'add_column',NULL,$4)", f.other, f.app, f.table, p); e != nil {
		t.Fatal(e)
	}
}
func TestRootEvidenceCaptureUsesRealValuesAndOriginalVisibleMask(t *testing.T) {
	f := rootCaptureSetup(t)
	b := rootCaptureRead(t, f, f.principal, f.otherRecord)
	m, e := ev.DecodeManifest(b.Manifest)
	if e != nil {
		t.Fatal(e)
	}
	if m.Header.AppID != f.app || m.Header.TableID != f.table || m.Header.ViewID != f.view || m.Header.RecordID != f.otherRecord || m.Header.CreatedBy != f.other || m.Header.RecordVersion != 1 || m.Header.SchemaVersion != 1 {
		t.Fatal("scope/version does not describe actual row")
	}
	want := []string{f.public, f.reference}
	sort.Strings(want)
	got := m.VisibleFieldIDs
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("row-specific original mask=%v want%v", got, want)
	}
	fields := rootCapturedFields(t, b)
	if len(fields) != 3 || string(fields[f.public].Value) != `"beta"` || string(fields[f.secret].Value) != `"private beta"` {
		t.Fatal("internal complete evidence did not capture actual other row")
	}
	// This internal bundle must never be serialized as an authorized HTTP record.
	if fields[f.reference].Reference == nil || fields[f.reference].Reference.ID != f.other {
		t.Fatal("reference display missing")
	}
	rootEvidenceStorePut(t, f, b)
	_, history := rootEvidenceStoreRead(t, f, b.Hash, []string{f.public})
	if len(history) != 1 || string(history[0].Value) != `"beta"` {
		t.Fatal("captured bundle did not survive immutable store")
	}
}
func TestRootEvidenceCaptureRetainsRRSourceSnapshotThenSeesRefresh(t *testing.T) {
	f := rootCaptureSetup(t)
	tx, facts, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, f.principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	var old string
	if e = tx.QueryRow(f.ctx, "SELECT label FROM applications.member_sources WHERE id=$1", f.other).Scan(&old); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", f.other, "renamed-"+f.other); e != nil {
		t.Fatal(e)
	}
	before, e := captureWorkflowEvidence(f.ctx, tx, facts, f.ownRecord)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	after := rootCaptureRead(t, f, f.principal, f.ownRecord)
	a, z := rootCapturedFields(t, before), rootCapturedFields(t, after)
	if a[f.reference].Reference.Label != old || z[f.reference].Reference.Label != "renamed-"+f.other || before.Hash == after.Hash {
		t.Fatal("source display was stale or mixed across RR boundaries")
	}
	ma, _ := ev.DecodeManifest(before.Manifest)
	mz, _ := ev.DecodeManifest(after.Manifest)
	if ma.Header.RecordVersion != mz.Header.RecordVersion {
		t.Fatal("reference rename unexpectedly changed record version")
	}
}
func TestRootEvidenceCaptureRejectsDeniedAndMissingRows(t *testing.T) {
	f := rootCaptureSetup(t)
	tx, facts, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, f.principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	missing := recordOperationID(t, f.recordFixture)
	if b, e := captureWorkflowEvidence(f.ctx, tx, facts, missing); !errors.Is(e, applications.ErrMissing) || len(b.Manifest) != 0 {
		t.Fatalf("missing row leaked a bundle: %v", e)
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app); e != nil {
		t.Fatal(e)
	}
	fresh, denied, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, f.principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Rollback(context.Background())
	if b, e := captureWorkflowEvidence(f.ctx, fresh, denied, f.ownRecord); !errors.Is(e, applications.ErrDenied) || len(b.Fields) != 0 {
		t.Fatalf("revoked group allowed capture: %v", e)
	}
}
func TestRootEvidenceCaptureDeletedMemberUsesRetainedRegistry(t *testing.T) {
	f := rootCaptureSetup(t)
	id := recordOperationID(t, f.recordFixture)
	label := "former-member-" + id
	if _, e := f.owner.Exec(f.ctx, "INSERT INTO auth.users(id,account) VALUES($1,$2)", id, label); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE "+rootCaptureRelation(f)+" SET "+rootCaptureColumn(f.reference)+"=$2 WHERE id=$1", f.ownRecord, id); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(f.ctx, "DELETE FROM auth.users WHERE id=$1", id); e != nil {
		t.Fatal(e)
	}
	b := rootCaptureRead(t, f, f.principal, f.ownRecord)
	field := rootCapturedFields(t, b)[f.reference]
	if field.Reference == nil || field.Reference.ID != id || field.Reference.Label != label || !field.Reference.Deleted {
		t.Fatal("deleted member label was lost")
	}
	// Simulate registry corruption only in this synthetic scope: fail rather than invent a label.
	if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.member_sources WHERE id=$1", id); e != nil {
		t.Fatal(e)
	}
	tx, facts, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, f.principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if b, e := captureWorkflowEvidence(f.ctx, tx, facts, f.ownRecord); !errors.Is(e, ErrUnavailable) || len(b.Manifest) != 0 {
		t.Fatalf("missing registry silently guessed display: %v", e)
	}
}
func TestRootEvidenceCaptureTypedValuesAndRemovedOptionLabels(t *testing.T) {
	f := rootCaptureSetup(t)
	defs := make([]appfields.Field, len(f.fields))
	for i, v := range f.fields {
		defs[i] = v.Definition
	}
	a, b := recordOperationID(t, f.recordFixture), recordOperationID(t, f.recordFixture)
	mk := func(kind, config string) appfields.Field {
		return appfields.Field{ID: recordOperationID(t, f.recordFixture), Name: kind, Kind: kind, Default: json.RawMessage("null"), Config: json.RawMessage(config)}
	}
	money := mk("money", `{"precision":38,"scale":18,"roundingPlaces":0,"roundingMode":"TOWARD_ZERO"}`)
	number := mk("number", `{"precision":38,"scale":18,"roundingPlaces":0,"roundingMode":"HALF_UP"}`)
	datetime := mk("datetime", `{"precision":"minute"}`)
	zone := "Asia/Shanghai"
	datetime.Presentation.DisplayTimeZone = &zone
	date := mk("date", `{}`)
	boolean := mk("boolean", `{}`)
	multi := mk("multi_select", fmt.Sprintf(`{"options":[{"id":%q,"label":"First"},{"id":%q,"label":"Removed second"}]}`, a, b))
	for _, item := range []struct {
		f        appfields.Field
		physical map[string]any
	}{{money, map[string]any{"Type": "numeric", "Precision": 38, "Scale": 18}}, {number, map[string]any{"Type": "numeric", "Precision": 38, "Scale": 18}}, {datetime, map[string]any{"Type": "timestamptz"}}, {date, map[string]any{"Type": "date"}}, {boolean, map[string]any{"Type": "boolean"}}, {multi, map[string]any{"Type": "uuid[]"}}} {
		rootCaptureAddField(t, f, item.f, item.physical)
		defs = append(defs, item.f)
	}
	exact := "12345678901234567890.123456789012345678"
	query := "UPDATE " + rootCaptureRelation(f) + " SET " + rootCaptureColumn(money.ID) + "=$2::numeric," + rootCaptureColumn(number.ID) + "=$2::numeric," + rootCaptureColumn(datetime.ID) + "=$3::timestamptz," + rootCaptureColumn(date.ID) + "='2026-10-05'," + rootCaptureColumn(boolean.ID) + "=false," + rootCaptureColumn(multi.ID) + "=$4::uuid[] WHERE id=$1"
	if _, e := f.owner.Exec(f.ctx, query, f.ownRecord, exact, "2026-10-05T18:02:03.123456+08:00", []string{b, a}); e != nil {
		t.Fatal(e)
	}
	// Removing an option updates the real tombstone trigger, not the stored value.
	defs[len(defs)-1].Config = json.RawMessage(fmt.Sprintf(`{"options":[{"id":%q,"label":"First"}]}`, a))
	// Current validation changes do not authorize re-normalizing existing data.
	defs[0].Config = json.RawMessage(`{"maxLength":1}`)
	rootCaptureDefinitions(t, f, defs)
	principal := f.principal
	principal.UserID = f.other
	tx, facts, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(f.ctx, "SET LOCAL DateStyle='SQL, DMY'"); e != nil {
		t.Fatal(e)
	}
	captured, e := captureWorkflowEvidence(f.ctx, tx, facts, f.ownRecord)
	if e != nil {
		t.Fatal(e)
	}
	fields := rootCapturedFields(t, captured)
	for _, id := range []string{money.ID, number.ID} {
		if string(fields[id].Value) != fmt.Sprintf(`%q`, exact) {
			t.Fatal("decimal was rounded or converted through float")
		}
	}
	if string(fields[datetime.ID].Value) != `"2026-10-05T10:02:03.123456Z"` || string(fields[date.ID].Value) != `"2026-10-05"` {
		t.Fatal("stored date/time precision or formatting changed")
	}
	if string(fields[boolean.ID].Value) != "false" || string(fields[f.public].Value) != `"alpha"` {
		t.Fatal("false value or old long text lost")
	}
	if !bytes.Equal(fields[multi.ID].Value, json.RawMessage(fmt.Sprintf(`[%q,%q]`, b, a))) {
		t.Fatal("stored selection order changed")
	}
	displays := fields[multi.ID].OptionDisplays
	if len(displays) != 2 {
		t.Fatal("selected labels missing")
	}
	seen := map[string]ev.Reference{}
	for _, v := range displays {
		seen[v.ID] = v
	}
	if seen[a].Label != "First" || seen[a].Deleted || seen[b].Label != "Removed second" || !seen[b].Deleted {
		t.Fatal("active and removed option displays disagree with registry")
	}
	if bytes.Contains(fields[multi.ID].Definition.Config, []byte("Removed second")) {
		t.Fatal("deleted option became active config")
	}
}
func TestRootEvidenceCaptureWriteLifecycleSharesOuterRollback(t *testing.T) {
	f := rootCaptureSetup(t)
	op := recordOperationID(t, f.recordFixture)
	write, e := (&applications.Application{Pool: f.runtime}).BeginRecordWrite(f.ctx, f.principal, f.app, f.view, applications.RecordWriteOptions{OperationID: op, Kind: "record.edit", LockTimeout: time.Second, StatementTimeout: 5 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	defer write.Rollback(context.Background())
	b, e := captureWorkflowEvidence(f.ctx, write.Tx(), write.Context(), f.ownRecord)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = (ev.Store{}).PutInTx(f.ctx, write.Tx(), b); e != nil {
		t.Fatal(e)
	}
	rootEvidenceStoreCounts(t, f, 0, 0, 0)
	if e = write.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	rootEvidenceStoreCounts(t, f, 0, 0, 0)
}
func TestRootEvidenceCaptureRejectsUnboundScopeAndInvalidPorts(t *testing.T) {
	f := rootCaptureSetup(t)
	other := rootCaptureSetup(t)
	tx, facts, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, f.principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	for _, bad := range []string{"", "not-an-id"} {
		if b, e := captureWorkflowEvidence(f.ctx, tx, facts, bad); !errors.Is(e, ev.ErrInvalid) || len(b.Manifest) != 0 {
			t.Fatalf("invalid ID: %v", e)
		}
	}
	if _, e = captureWorkflowEvidence(nil, tx, facts, f.ownRecord); !errors.Is(e, ev.ErrInvalid) {
		t.Fatalf("nil context: %v", e)
	}
	if _, e = captureWorkflowEvidence(f.ctx, nil, facts, f.ownRecord); !errors.Is(e, ev.ErrInvalid) {
		t.Fatalf("nil transaction: %v", e)
	}
	forged := facts
	forged.TableID = other.table
	if b, e := captureWorkflowEvidence(f.ctx, tx, forged, f.ownRecord); !errors.Is(e, applications.ErrMissing) || len(b.Fields) != 0 {
		t.Fatalf("unbound app/view/table: %v", e)
	}
}

func TestRootEvidenceCaptureReferenceQueriesAreBatched(t *testing.T) {
	f := rootCaptureSetup(t)
	defs := make([]appfields.Field, len(f.fields))
	for i, v := range f.fields {
		defs[i] = v.Definition
	}
	for i := 0; i < 20; i++ {
		field := appfields.Field{ID: recordOperationID(t, f.recordFixture), Name: fmt.Sprintf("Member %02d", i), Kind: "member", Default: json.RawMessage("null"), Config: json.RawMessage(`{}`)}
		rootCaptureAddField(t, f, field, map[string]any{"Type": "uuid"})
		defs = append(defs, field)
		if _, e := f.owner.Exec(f.ctx, "UPDATE "+rootCaptureRelation(f)+" SET "+rootCaptureColumn(field.ID)+"=$2 WHERE id=$1", f.ownRecord, f.other); e != nil {
			t.Fatal(e)
		}
	}
	rootCaptureDefinitions(t, f, defs)
	trace := new(rootEvidenceQueryTrace)
	cfg := f.runtime.Config()
	cfg.ConnConfig.Tracer = trace
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	principal := f.principal
	principal.UserID = f.other
	tx, facts, e := (&applications.Application{Pool: pool}).BeginRecordRead(f.ctx, principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	trace.count.Store(0)
	b, e := captureWorkflowEvidence(f.ctx, tx, facts, f.ownRecord)
	queries := trace.count.Load()
	if e != nil || len(b.Fields) != 23 {
		t.Fatalf("wide capture: %v", e)
	}
	if queries > 6 {
		t.Fatalf("capture issued %d statements for 21 reference fields", queries)
	}
	for _, field := range rootCapturedFields(t, b) {
		if field.Definition.Kind == "member" && (field.Reference == nil || field.Reference.ID != f.other) {
			t.Fatal("batched source display missing")
		}
	}
	t.Logf("23 fields including 21 member references: %d capture SQL statements", queries)
}
