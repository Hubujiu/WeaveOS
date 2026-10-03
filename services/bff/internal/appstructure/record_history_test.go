package appstructure

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
	"time"
)

func historyCommand(t *testing.T, f *fixture, view string, kind string) *applications.RecordWrite {
	t.Helper()
	op := uuid(t, f.owner)
	w, e := (&applications.Application{Pool: f.runtime}).BeginRecordWrite(context.Background(), session.Principal{UserID: f.actor, Record: session.Record{AuthVersion: "1"}}, f.app, view, applications.RecordWriteOptions{OperationID: op, Kind: kind, LockTimeout: time.Second, StatementTimeout: 5 * time.Second, Authorize: func(context.Context, pgx.Tx, applications.RecordContext) error { return nil }})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { w.Rollback(context.Background()) })
	if e = w.Claim(context.Background(), op, kind, [32]byte{}); e != nil {
		t.Fatal(e)
	}
	return w
}
func historyComplete(t *testing.T, w *applications.RecordWrite, header StoredRecordHeader, op string, status int) {
	t.Helper()
	result := RecordMutationResult{OperationID: op, ID: header.ID, RecordVersion: header.RecordVersion, SchemaVersion: w.Context().SchemaVersion, CreatedAt: header.CreatedAt, UpdatedAt: header.UpdatedAt}
	raw, _ := json.Marshal(result)
	if e := w.Complete(context.Background(), op, applications.Result{Status: status, Location: "/records/" + header.ID, Data: raw}); e != nil {
		t.Fatal(e)
	}
}
func recordOpID(t *testing.T, w *applications.RecordWrite) string {
	t.Helper()
	var op string
	if e := w.Tx().QueryRow(context.Background(), "SELECT operation_id::text FROM applications.operations WHERE actor_user_id=$1 AND result_json IS NULL", w.Context().Actor.ID).Scan(&op); e != nil {
		t.Fatal(e)
	}
	return op
}

func TestPublishedRecordDMLIncludesRequiredSameTxHistory(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	w := historyCommand(t, f, view, "record.create")
	op := recordOpID(t, w)
	id := uuid(t, f.owner)
	header, e := (RecordDML{}).Insert(context.Background(), w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true}, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{text["id"].(string): "published typed value"}}, []string{text["id"].(string)})
	if e != nil {
		t.Fatal(e)
	}
	historyComplete(t, w, header, op, 201)
	if e = w.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.record_change_events WHERE record_id=$1 AND operation_id=$2", id, op).Scan(&count); e != nil || count != 1 {
		t.Fatal("published DML must include same-Tx history", count, e)
	}
}

func TestRequiredCreateHistoryBeforeIsAbsentNull(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", "required constant", map[string]any{"maxLength": nil})
	text["required"] = true
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	w := historyCommand(t, f, view, "record.create")
	op := recordOpID(t, w)
	id := uuid(t, f.owner)
	header, e := (RecordHistoryDML{History: RecordHistoryStore{}}).Insert(context.Background(), w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true}, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{}}, []string{})
	if e != nil {
		t.Fatal("required create's old absence is legal history null", e)
	}
	historyComplete(t, w, header, op, 201)
	if e = w.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	var before, after json.RawMessage
	if e = f.owner.QueryRow(context.Background(), "SELECT old_value,new_value FROM applications.record_change_values WHERE event_id=(SELECT id FROM applications.record_change_events WHERE record_id=$1) AND field_id=$2", id, text["id"]).Scan(&before, &after); e != nil || string(before) != "null" || string(after) != `"required constant"` {
		t.Fatal("create history canonical absence/value", string(before), string(after), e)
	}
}
func TestHistoryTypedWriteCanonicalRollbackNoopAndScopedPage(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	money := field(t, f, "money", nil, map[string]any{})
	secret := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, money, secret)), 200)
	c := context.Background()
	moneyID, secretID := money["id"].(string), secret["id"].(string)
	tablePort := RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true, ActiveFieldIDs: []string{moneyID, secretID}}
	dml := RecordHistoryDML{History: RecordHistoryStore{}, Origin: "ordinary"}
	id := uuid(t, f.owner)
	w := historyCommand(t, f, view, "record.create")
	op := recordOpID(t, w)
	header, e := dml.Insert(c, w.Tx(), tablePort, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{moneyID: "12345678901234567890.12", secretID: "confidential"}}, []string{moneyID, secretID})
	if e != nil {
		t.Fatal("real typed insert must append canonical history in caller tx", e)
	}
	historyComplete(t, w, header, op, 201)
	w.Rollback(c)
	var count int
	if e = f.owner.QueryRow(c, "SELECT count(*) FROM applications.record_change_events WHERE record_id=$1", id).Scan(&count); e != nil || count != 0 {
		t.Fatal("history escaped rollback", count, e)
	}
	if e = f.owner.QueryRow(c, "SELECT count(*) FROM "+tablePhysical(table)+" WHERE id=$1", id).Scan(&count); e != nil || count != 0 {
		t.Fatal("typed insert escaped rollback", count, e)
	}
	w = historyCommand(t, f, view, "record.create")
	op = recordOpID(t, w)
	header, e = dml.Insert(c, w.Tx(), tablePort, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{moneyID: "12345678901234567890.12", secretID: "confidential"}}, []string{moneyID, secretID})
	if e != nil {
		t.Fatal(e)
	}
	if e = (RecordAudit{Context: w.Context(), OperationID: op, Metadata: applications.Metadata{RequestID: "history-create"}}).Append(c, w.Tx(), RecordMutationResult{OperationID: op, ID: id, RecordVersion: 1, SchemaVersion: 1, CreatedAt: header.CreatedAt, UpdatedAt: header.UpdatedAt}, "create", []string{moneyID, secretID}); e != nil {
		t.Fatal(e)
	}
	historyComplete(t, w, header, op, 201)
	if e = w.Commit(c); e != nil {
		t.Fatal(e)
	}
	for _, change := range []map[string]any{{secretID: "hidden newer change"}, {moneyID: "12345678901234567890.13"}, {moneyID: "12345678901234567890.13"}} {
		w = historyCommand(t, f, view, "record.edit")
		op = recordOpID(t, w)
		selected := []string{}
		for id := range change {
			selected = append(selected, id)
		}
		header, e = dml.UpdateCAS(c, w.Tx(), tablePort, RecordEdit{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, ExpectedRecordVersion: header.RecordVersion, Changes: change}, selected)
		if e != nil {
			t.Fatal(e)
		}
		historyComplete(t, w, header, op, 200)
		if e = w.Commit(c); e != nil {
			t.Fatal(e)
		}
	}
	tx, e := f.runtime.BeginTx(c, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(c)
	page, e := (RecordHistoryStore{}).Page(c, tx, HistoryRead{AppID: f.app, TableID: table, ViewID: view, RecordID: id, FieldIDs: []string{moneyID}, PageSize: 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(page.Items) != 1 || !page.HasMore || len(page.Items[0].Changes) != 1 || string(page.Items[0].Changes[0].Before) != `"12345678901234567890.12"` || string(page.Items[0].Changes[0].After) != `"12345678901234567890.13"` {
		t.Fatalf("canonical scoped delta: %+v", page)
	}
	first := page.Items[0]
	page, e = (RecordHistoryStore{}).Page(c, tx, HistoryRead{AppID: f.app, TableID: table, ViewID: view, RecordID: id, FieldIDs: []string{moneyID}, PageSize: 1, AfterTime: &first.OccurredAt, AfterID: &first.ID})
	if e != nil {
		t.Fatal(e)
	}
	if len(page.Items) != 1 || page.HasMore || page.Items[0].RecordVersionBefore != 0 || page.Items[0].Changes[0].FieldKind != "money" {
		t.Fatal("hidden event must be excluded before LIMIT/cursor", page)
	}
	if e = f.owner.QueryRow(c, "SELECT count(*) FROM applications.record_change_events WHERE record_id=$1", id).Scan(&count); e != nil || count != 3 {
		t.Fatal("same-value mutation fabricated delta", count, e)
	}
	var raw string
	if e = f.owner.QueryRow(c, "SELECT result_json::text FROM applications.operations WHERE operation_id=$1", op).Scan(&raw); e != nil || strings.Contains(raw, "confidential") || strings.Contains(raw, "123456789") {
		t.Fatal("operation leaked business values", raw, e)
	}
}
func TestHistoryMigrationPreservesOptionLabelsAndRestrictsAuditReader(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	a, b := uuid(t, f.owner), uuid(t, f.owner)
	choice := field(t, f, "single_select", nil, map[string]any{"options": []any{map[string]any{"id": a, "label": "last A"}, map[string]any{"id": b, "label": "B"}}})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, choice)), 200)
	choice["config"] = map[string]any{"options": []any{map[string]any{"id": b, "label": "B new"}}}
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1, choice)), 200)
	var label string
	var removed bool
	if e := f.runtime.QueryRow(context.Background(), "SELECT label,removed FROM applications.field_option_tombstones WHERE app_id=$1 AND table_id=$2 AND field_id=$3 AND option_id=$4", f.app, table, choice["id"], a).Scan(&label, &removed); e != nil || label != "last A" || !removed {
		t.Fatal("removed option last label lost", label, removed, e)
	}
	for _, relation := range []string{"record_change_events", "record_change_values"} {
		var reader, backup bool
		if e := f.owner.QueryRow(context.Background(), "SELECT has_table_privilege('auth_reader',$1,'SELECT'),has_table_privilege('auth_backup',$1,'SELECT')", "applications."+relation).Scan(&reader, &backup); e != nil || reader || !backup {
			t.Fatal("history values role boundary", relation, reader, backup, e)
		}
	}
}
func TestDataGrantBlocksFieldRemovalAndChangesDependencyRevision(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	text := field(t, f, "text", nil, map[string]any{"maxLength": nil})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, text)), 200)
	c := context.Background()
	var before, after int64
	if e := f.owner.QueryRow(c, "SELECT dependency_revision FROM applications.logical_tables WHERE id=$1", table).Scan(&before); e != nil {
		t.Fatal(e)
	}
	var group, grant string
	if e := f.owner.QueryRow(c, "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'protected field') RETURNING id::text", f.app).Scan(&group); e != nil {
		t.Fatal(e)
	}
	if e := f.owner.QueryRow(c, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'data.read','all') RETURNING id::text", f.app, group, view).Scan(&grant); e != nil {
		t.Fatal(e)
	}
	if _, e := f.owner.Exec(c, "INSERT INTO applications.grant_fields(app_id,grant_id,table_id,field_id) VALUES($1,$2,$3,$4)", f.app, grant, table, text["id"]); e != nil {
		t.Fatal(e)
	}
	out := data(t, f.call(t, "POST", "/forms/"+view+"/definition/preflight", preflight(input(t, f, 1, 1))), 200)
	if out["saveAllowed"] != false {
		t.Fatal("field deletion did not expose data-grant dependency", out)
	}
	deps := out["dependencies"].([]any)
	if len(deps) != 1 || deps[0].(map[string]any)["kind"] != "data_grant" || deps[0].(map[string]any)["resourceId"] != grant {
		t.Fatal("precise grant dependency absent", deps)
	}
	if e := f.owner.QueryRow(c, "SELECT dependency_revision FROM applications.logical_tables WHERE id=$1", table).Scan(&after); e != nil || after <= before {
		t.Fatal("grant addition not bound to confirmation revision", before, after, e)
	}
	expectError(t, f, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1), 409, "APPLICATION_SCHEMA_DEPENDENCY_BLOCKED")
	if _, e := f.owner.Exec(c, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id=$2", f.app, grant); e != nil {
		t.Fatal(e)
	}
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 1, 1)), 200)
}

func TestHistoryCreateCapturesActualSQLDefaultAndStorageFailureRollsBack(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	money := field(t, f, "money", "9.99", map[string]any{})
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0, money)), 200)
	c := context.Background()
	id := uuid(t, f.owner)
	w := historyCommand(t, f, view, "record.create")
	op := recordOpID(t, w)
	dml := RecordHistoryDML{History: RecordHistoryStore{}, Origin: "ordinary"}
	header, e := dml.Insert(c, w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true}, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{}}, []string{})
	if e != nil {
		t.Fatal(e)
	}
	historyComplete(t, w, header, op, 201)
	if e = w.Commit(c); e != nil {
		t.Fatal(e)
	}
	var actual string
	if e = f.owner.QueryRow(c, "SELECT v.new_value::text FROM applications.record_change_values v JOIN applications.record_change_events e ON e.id=v.event_id WHERE e.record_id=$1 AND v.field_id=$2", id, money["id"]).Scan(&actual); e != nil || actual != `"9.99"` {
		t.Fatal("omitted SQL default absent from save history", actual, e)
	}
	function := "fail_history_" + strings.ReplaceAll(uuid(t, f.owner), "-", "")
	if _, e = f.owner.Exec(c, "CREATE FUNCTION applications."+function+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated history failure' USING ERRCODE='23514'; END $$; CREATE TRIGGER "+function+" BEFORE INSERT ON applications.record_change_values FOR EACH ROW EXECUTE FUNCTION applications."+function+"()"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		f.owner.Exec(c, "DROP TRIGGER "+function+" ON applications.record_change_values; DROP FUNCTION applications."+function+"()")
	})
	w = historyCommand(t, f, view, "record.edit")
	op = recordOpID(t, w)
	_, e = dml.UpdateCAS(c, w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, SchemaVersion: 1, Ready: true}, RecordEdit{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1, Changes: map[string]any{money["id"].(string): "10.00"}}, []string{money["id"].(string)})
	if e == nil {
		t.Fatal("real history failure must fail record mutation")
	}
	w.Rollback(c)
	var version int64
	if e = f.owner.QueryRow(c, "SELECT record_version,"+columnPhysical(money["id"].(string))+"::text FROM "+tablePhysical(table)+" WHERE id=$1", id).Scan(&version, &actual); e != nil || version != 1 || actual != "9.99" {
		t.Fatal("record changed despite history failure", version, actual, e)
	}
	var count int
	if e = f.owner.QueryRow(c, "SELECT count(*) FROM applications.operations WHERE operation_id=$1", op).Scan(&count); e != nil || count != 0 {
		t.Fatal("operation survived aborted history transaction", count, e)
	}
}
