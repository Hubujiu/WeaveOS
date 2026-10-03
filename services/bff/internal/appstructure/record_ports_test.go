package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"
)

func TestRealRecordGateFenceAuditAndMinimumOperationLifecycle(t *testing.T) {
	f := setup(t)
	view, table := newForm(t, f)
	data(t, f.call(t, "PUT", "/forms/"+view+"/definition", input(t, f, 0, 0)), 200)
	ctx := context.Background()
	op := uuid(t, f.owner)
	options := applications.RecordWriteOptions{OperationID: op, Kind: "record.create", LockTimeout: time.Second, StatementTimeout: 5 * time.Second, Authorize: func(_ context.Context, _ pgx.Tx, facts applications.RecordContext) error {
		if facts.App.OwnerUserID != facts.Actor.ID {
			return applications.ErrDenied
		}
		return nil
	}}
	p := session.Principal{UserID: f.actor, Record: session.Record{AuthVersion: "1"}}
	w, e := (&applications.Application{Pool: f.runtime}).BeginRecordWrite(ctx, p, f.app, view, options)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Rollback(ctx)
	if e = w.Claim(ctx, op, options.Kind, options.Fingerprint); e != nil {
		t.Fatal(e)
	}
	if e := (RecordGate{AppID: f.app, TableID: table, ViewID: view}).LockTable(ctx, w.Tx(), table, 1); e != nil {
		t.Fatal("same real table gate must be callable", e)
	}
	id := uuid(t, f.owner)
	fence := RecordFence{AppID: f.app, TableID: table}
	if e = fence.Check(ctx, w.Tx(), table, id, 1); e != nil {
		t.Fatal("available authoritative empty fence must permit", e)
	}
	if e = fence.Check(ctx, nil, table, id, 1); !errors.Is(e, session.ErrUnavailable) {
		t.Fatal("missing fence transaction must fail closed", e)
	}
	header, e := (RecordDML{}).Insert(ctx, w.Tx(), RecordTable{AppID: f.app, TableID: table, ViewID: view, Namespace: "pg_catalog", SchemaVersion: 1, Ready: true, ActiveFieldIDs: []string{}}, RecordCreate{OperationID: op, ID: id, ActorID: f.actor, ExpectedSchemaVersion: 1, Values: map[string]any{}}, []string{})
	if e != nil {
		t.Fatal(e)
	}
	result := RecordMutationResult{OperationID: op, ID: id, RecordVersion: header.RecordVersion, SchemaVersion: 1, CreatedAt: header.CreatedAt, UpdatedAt: header.UpdatedAt}
	audit := RecordAudit{Context: w.Context(), OperationID: op, Metadata: applications.Metadata{RequestID: "actual-record-txn-correlation"}}
	if e = audit.Append(ctx, w.Tx(), result, "create", []string{}); e != nil {
		t.Fatal("minimum audit must share caller tx", e)
	}
	raw, _ := json.Marshal(result)
	if e = w.Complete(ctx, op, applications.Result{Status: 201, Location: "/api/v1/applications/" + f.app + "/forms/" + view + "/records/" + id, Data: raw}); e != nil {
		t.Fatal(e)
	}
	if e = w.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	own, e := (&applications.Application{Pool: f.runtime}).Operation(ctx, p, op)
	if e != nil || own.HTTPStatus != 201 || json.Unmarshal(own.Result, &map[string]any{}) != nil {
		t.Fatal("current same actor minimum operation unavailable", own, e)
	}
	var count int
	if e = f.owner.QueryRow(ctx, "SELECT count(*) FROM applications.record_write_audit WHERE actor_user_id=$1 AND operation_id=$2 AND request_id='actual-record-txn-correlation'", f.actor, op).Scan(&count); e != nil || count != 1 {
		t.Fatal("minimum audit not durable", count, e)
	}
	if _, e = f.owner.Exec(ctx, "INSERT INTO applications.record_command_fences(app_id,table_id,record_id,command_id,expected_record_version,state) VALUES($1,$2,$3,$4,1,'pending')", f.app, table, id, uuid(t, f.owner)); e != nil {
		t.Fatal(e)
	}
	tx, e := f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	e = fence.Check(ctx, tx, table, id, 1)
	var blocked *Error
	if !errors.As(e, &blocked) || blocked.Code != "APPLICATION_RECORD_FENCED" {
		t.Fatal("real pending fence must block", e)
	}
	options.SourceGuard = func(context.Context, pgx.Tx) error { return applications.ErrResourceInvalid }
	options.Authorize = func(context.Context, pgx.Tx, applications.RecordContext) error { return applications.ErrDenied }
	replay, e := (&applications.Application{Pool: f.runtime}).BeginRecordWrite(ctx, p, f.app, view, options)
	if e != nil {
		t.Fatal("confirmed replay must precede new-source/action policy", e)
	}
	defer replay.Rollback(ctx)
	recovered, e := replay.Replay(ctx, op, options.Kind, options.Fingerprint)
	if e != nil || recovered == nil || string(recovered.Data) != string(own.Result) {
		t.Fatal("confirmed minimum replay altered", recovered, e)
	}
}
