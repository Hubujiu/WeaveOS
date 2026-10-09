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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func eventSQL(t *testing.T, f rootTaskFixture, q string, args ...any) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, q, args...); e != nil {
		t.Fatal(e)
	}
}
func eventGrant(t *testing.T, f rootTaskFixture, scope string, ids ...string) {
	t.Helper()
	var group, grant string
	if e := f.owner.QueryRow(f.ctx, "SELECT group_id::text FROM applications.group_members WHERE app_id=$1 AND user_id=$2 LIMIT 1", f.app, f.actor).Scan(&group); e != nil {
		t.Fatal(e)
	}
	if e := f.owner.QueryRow(f.ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'data.history',$4) RETURNING id::text", f.app, group, f.view, scope).Scan(&grant); e != nil {
		t.Fatal(e)
	}
	for _, id := range ids {
		eventSQL(t, f, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, id, f.table)
	}
}
func eventConfirmed(t *testing.T, f rootTaskFixture, action string) WorkflowEventRequest {
	t.Helper()
	op := rootAction(t, f, rootActionRequest(t, f, action))
	c, p := rootActionRead(t, f, op)
	state := "completed"
	if action == "reject" {
		state = "rejected"
	}
	r, b := f.receipt(t, c, state, "")
	f.apply(t, c, p, r, b, true)
	return WorkflowEventRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, EventID: c.CommandID}
}
func eventRead(t *testing.T, f rootTaskFixture, p session.Principal, q WorkflowEventRequest) WorkflowEventResult {
	t.Helper()
	got, e := f.service.ReadWorkflowEvent(f.ctx, p, q)
	if e != nil {
		t.Fatal(e)
	}
	return got
}
func eventFields(t *testing.T, got WorkflowEventResult) map[string]WorkflowHistoricalField {
	t.Helper()
	out := map[string]WorkflowHistoricalField{}
	ids := []string{}
	for _, v := range got.Basis.Fields {
		if _, ok := out[v.FieldID]; ok {
			t.Fatal("duplicate historical field")
		}
		out[v.FieldID] = v
		ids = append(ids, v.FieldID)
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("unstable historical field order")
	}
	return out
}
func TestRootWorkflowEventExactHistoricalBasis(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "all", f.public, f.secret, f.reference)
	q := eventConfirmed(t, f, "reject")
	var oldLabel string
	if e := f.owner.QueryRow(f.ctx, "SELECT label FROM applications.member_sources WHERE id=$1", f.other).Scan(&oldLabel); e != nil {
		t.Fatal(e)
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	eventSQL(t, f, "UPDATE "+relation+" SET "+rootCaptureColumn(f.public)+"='new value',record_version=2 WHERE id=$1", f.ownRecord)
	eventSQL(t, f, "UPDATE applications.member_sources SET label='Renamed now' WHERE id=$1", f.other)
	got := eventRead(t, f, f.principal, q)
	if got.Event.ID != q.EventID || got.Event.InstanceID != f.instance.ID || got.Event.FlowID != f.head.FlowID || got.Event.ActorID != f.actor || got.Event.Action != "reject" || got.Event.Outcome != "success" || got.Event.Sequence != 2 || got.Event.SchemaVersion != 1 || got.Event.RecordVersion != 1 || got.Event.NodeID == nil || *got.Event.NodeID != f.first || got.Event.TargetNodeID != nil {
		t.Fatalf("incorrect confirmed event: %+v", got.Event)
	}
	if _, e := time.Parse(time.RFC3339Nano, got.Event.OccurredAt); e != nil || !strings.HasSuffix(got.Event.OccurredAt, "Z") {
		t.Fatal("invalid event time")
	}
	fields := eventFields(t, got)
	if got.Basis.Status != "available" || len(fields) != 2 || string(fields[f.public].Value) != `"beta"` || fields[f.public].FieldName != "Public" || fields[f.public].FieldKind != "text" || fields[f.reference].ValueLabels[f.other].Label != oldLabel {
		t.Fatalf("historical basis replaced with current values: %+v", got.Basis)
	}
	raw, _ := json.Marshal(got)
	if bytes.Contains(raw, []byte(f.secret)) || bytes.Contains(raw, []byte("private beta")) || bytes.Contains(raw, []byte("engine")) || bytes.Contains(raw, []byte("evidenceHash")) {
		t.Fatal("hidden evidence leaked")
	}
	rootActionCount(t, f, 0, 1, 1)
}
func TestRootWorkflowEventCurrentAuthorityAndOriginalMask(t *testing.T) {
	f := rootTaskSetup(t, true)
	q := eventConfirmed(t, f, "agree")
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("history-free actor read event", e)
	}
	eventGrant(t, f, "all", f.public, f.reference)
	got := eventRead(t, f, f.principal, q)
	if len(got.Basis.Fields) != 2 {
		t.Fatal("current projection incomplete")
	}
	owner := f.principal
	owner.UserID = f.other
	owner.SessionRef = f.other
	got = eventRead(t, f, owner, q)
	if len(got.Basis.Fields) != 2 {
		t.Fatal("owner expanded original approver mask")
	}
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.history')", f.app, f.public)
	got = eventRead(t, f, f.principal, q)
	if len(got.Basis.Fields) != 1 || got.Basis.Fields[0].FieldID != f.reference {
		t.Fatal("revoked history field survived")
	}
	eventSQL(t, f, "DELETE FROM applications.grants WHERE app_id=$1 AND action='menu.enter'", f.app)
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("menu revoked actor read event", e)
	}
}
func TestRootWorkflowEventLegacyAbsenceNotReconstructed(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	var id string
	if e := f.owner.QueryRow(f.ctx, "SELECT command_id::text FROM applications.workflow_execution_events WHERE app_id=$1 AND action='start'", f.app).Scan(&id); e != nil {
		t.Fatal(e)
	}
	got := eventRead(t, f, f.principal, WorkflowEventRequest{f.app, f.view, f.ownRecord, id})
	if got.Basis.Status != "unavailable" || got.Basis.Fields == nil || len(got.Basis.Fields) != 0 || got.Event.Action != "start" || got.Event.NodeID != nil {
		t.Fatal("legacy evidence invented", got)
	}
	rootTaskNoEvidenceWrites(t, f)
}
func TestRootWorkflowEventMissingResourceAndCorruptEvidence(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	q := eventConfirmed(t, f, "agree")
	wrong := q
	wrong.RecordID = f.otherRecord
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, wrong); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("cross-record event exposed", e)
	}
	wrong = q
	wrong.EventID = recordOperationID(t, f.recordFixture)
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, wrong); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("missing event exposed", e)
	}
	eventSQL(t, f, "UPDATE applications.workflow_evidence_documents SET body=set_byte(body,8,0) WHERE app_id=$1", f.app)
	got, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q)
	if !errors.Is(e, ErrUnavailable) || len(got.Basis.Fields) != 0 || got.Event.ID != "" {
		t.Fatal("corrupt evidence returned partial success", e)
	}
}

func TestRootWorkflowEventPendingAndNoEffectRemainDistinct(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	op := rootAction(t, f, rootActionRequest(t, f, "agree"))
	c, p := rootActionRead(t, f, op)
	q := WorkflowEventRequest{f.app, f.view, f.ownRecord, c.CommandID}
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("pending command presented as confirmed event", e)
	}
	r, b := f.receipt(t, c, "unchanged", "task_inactive")
	f.apply(t, c, p, r, b, true)
	got := eventRead(t, f, f.principal, q)
	if got.Event.Action != "agree" || got.Event.Outcome != "no_effect" || got.Event.Sequence != 1 || got.Basis.Status != "available" {
		t.Fatal("execution failure disguised as business outcome", got.Event)
	}
	var state string
	if e := f.owner.QueryRow(f.ctx, "SELECT state FROM applications.workflow_instances WHERE id=$1", f.instance.ID).Scan(&state); e != nil || state != "active" {
		t.Fatal("read changed original active instance", e)
	}
}

func TestRootWorkflowEventHistoryOwnAndAuthRevocation(t *testing.T) {
	f := rootTaskSetup(t, true)
	eventGrant(t, f, "own", f.public)
	q := eventConfirmed(t, f, "agree")
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("borrowed all-read for own-history", e)
	}
	eventSQL(t, f, "UPDATE applications.grants SET row_scope='all' WHERE app_id=$1 AND action='data.history'", f.app)
	if len(eventRead(t, f, f.principal, q).Basis.Fields) != 1 {
		t.Fatal("restored exact history field missing")
	}
	p := f.principal
	p.Record.AuthVersion = "999"
	if _, e := f.service.ReadWorkflowEvent(f.ctx, p, q); !errors.Is(e, session.ErrUnauthorized) {
		t.Fatal("revoked live auth accepted", e)
	}
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app)
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("no read mask exposed history", e)
	}
}

func TestRootWorkflowEventAlternateAuthorizedFormAndCrossApp(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	q := eventConfirmed(t, f, "agree")
	view := recordOperationID(t, f.recordFixture)
	eventSQL(t, f, "INSERT INTO applications.form_views(id,app_id,table_id,name,position,view_version) VALUES($1,$2,$3,'history view',1,1)", view, f.app, f.table)
	eventSQL(t, f, "INSERT INTO applications.menu_resources VALUES($1,'form',$2)", f.app, view)
	var group string
	if e := f.owner.QueryRow(f.ctx, "SELECT group_id::text FROM applications.group_members WHERE app_id=$1 AND user_id=$2 LIMIT 1", f.app, f.actor).Scan(&group); e != nil {
		t.Fatal(e)
	}
	for _, action := range []string{"menu.enter", "data.read", "data.history"} {
		var grant string
		if e := f.owner.QueryRow(f.ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,$4,'all') RETURNING id::text", f.app, group, view, action).Scan(&grant); e != nil {
			t.Fatal(e)
		}
		if action != "menu.enter" {
			eventSQL(t, f, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, f.public, f.table)
		}
	}
	q.ViewID = view
	got := eventRead(t, f, f.principal, q)
	if len(got.Basis.Fields) != 1 || string(got.Basis.Fields[0].Value) != `"alpha"` {
		t.Fatal("authorized alternate view rejected or expanded")
	}
	other := rootTaskSetup(t, false)
	eventGrant(t, other, "all", other.public)
	wrong := WorkflowEventRequest{other.app, other.view, other.ownRecord, q.EventID}
	if _, e := other.service.ReadWorkflowEvent(other.ctx, other.principal, wrong); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("cross-app event exposed", e)
	}
}

func TestRootWorkflowEventOwnerRetainsRemovedFieldsButNotMissingForm(t *testing.T) {
	f := rootTaskSetup(t, false)
	q := eventConfirmed(t, f, "agree")
	// Isolate the already-authorized removed-field read model. Schema-save
	// dependency checks have their own real tests; this is not a Save shortcut.
	eventSQL(t, f, "DELETE FROM applications.grant_fields WHERE app_id=$1", f.app)
	eventSQL(t, f, "UPDATE applications.fields SET removed=true WHERE app_id=$1", f.app)
	eventSQL(t, f, "UPDATE applications.logical_tables SET fields_json='[]',schema_version=2 WHERE app_id=$1", f.app)
	owner := f.principal
	owner.UserID = f.other
	owner.SessionRef = f.other
	got := eventRead(t, f, owner, q)
	if len(got.Basis.Fields) != 3 || string(eventFields(t, got)[f.secret].Value) != `"private alpha"` {
		t.Fatal("owner lost retained originally visible field history")
	}
	if _, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("revoked ordinary actor saw removed fields", e)
	}
	q.ViewID = recordOperationID(t, f.recordFixture)
	if _, e := f.service.ReadWorkflowEvent(f.ctx, owner, q); !errors.Is(e, applications.ErrMissing) {
		t.Fatal("owner bypassed actual form existence", e)
	}
}

func TestRootWorkflowEventLoadsOnlyCurrentlyAuthorizedBlobs(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	q := eventConfirmed(t, f, "agree")
	eventSQL(t, f, "UPDATE applications.workflow_evidence_blobs SET body=set_byte(body,8,0) WHERE app_id=$1 AND field_id=$2", f.app, f.secret)
	got := eventRead(t, f, f.principal, q)
	if len(got.Basis.Fields) != 1 || got.Basis.Fields[0].FieldID != f.public {
		t.Fatal("unreadable blob affected allowed projection")
	}
	eventSQL(t, f, "UPDATE applications.workflow_evidence_blobs SET body=set_byte(body,8,0) WHERE app_id=$1 AND field_id=$2", f.app, f.public)
	got, e := f.service.ReadWorkflowEvent(f.ctx, f.principal, q)
	if !errors.Is(e, ErrUnavailable) || got.Event.ID != "" {
		t.Fatal("authorized corrupt blob did not fail closed", e)
	}
}

func TestRootWorkflowEventExactNumericArrayAndHistoricalOptionLabels(t *testing.T) {
	f := rootTaskSetup(t, false)
	base := rootEvidenceStoreFixture{recordFixture: f.recordFixture}
	var defs []appfields.Field
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT fields_json FROM applications.logical_tables WHERE id=$1", f.table).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(raw, &defs); e != nil {
		t.Fatal(e)
	}
	a, b := recordOperationID(t, f.recordFixture), recordOperationID(t, f.recordFixture)
	money := appfields.Field{ID: recordOperationID(t, f.recordFixture), Name: "Original money", Kind: "money", Default: json.RawMessage("null"), Config: json.RawMessage(`{"precision":38,"scale":18,"roundingPlaces":0,"roundingMode":"TOWARD_ZERO"}`)}
	multi := appfields.Field{ID: recordOperationID(t, f.recordFixture), Name: "Original selections", Kind: "multi_select", Default: json.RawMessage("null"), Config: json.RawMessage(fmt.Sprintf(`{"options":[{"id":%q,"label":"First"},{"id":%q,"label":"Second"}]}`, a, b))}
	rootCaptureAddField(t, base, money, map[string]any{"Type": "numeric", "Precision": 38, "Scale": 18})
	rootCaptureAddField(t, base, multi, map[string]any{"Type": "uuid[]"})
	defs = append(defs, money, multi)
	rootCaptureDefinitions(t, base, defs)
	var grant string
	if e := f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all' LIMIT 1", f.app).Scan(&grant); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{money.ID, multi.ID} {
		eventSQL(t, f, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, id, f.table)
	}
	eventGrant(t, f, "all", money.ID, multi.ID)
	exact := "12345678901234567890.123456789012345678"
	eventSQL(t, f, "UPDATE "+rootCaptureRelation(base)+" SET "+rootCaptureColumn(money.ID)+"=$2::numeric,"+rootCaptureColumn(multi.ID)+"=$3::uuid[] WHERE id=$1", f.ownRecord, exact, []string{b, a})
	q := eventConfirmed(t, f, "agree")
	defs[len(defs)-1].Name = "Renamed selections"
	defs[len(defs)-1].Config = json.RawMessage(fmt.Sprintf(`{"options":[{"id":%q,"label":"Now first"}]}`, a))
	rootCaptureDefinitions(t, base, defs)
	eventSQL(t, f, "UPDATE "+rootCaptureRelation(base)+" SET "+rootCaptureColumn(money.ID)+"=0,"+rootCaptureColumn(multi.ID)+"=$2::uuid[],record_version=2 WHERE id=$1", f.ownRecord, []string{a})
	got := eventRead(t, f, f.principal, q)
	fields := eventFields(t, got)
	wantArray, _ := json.Marshal([]string{b, a})
	if len(fields) != 2 || string(fields[money.ID].Value) != fmt.Sprintf("%q", exact) || string(fields[multi.ID].Value) != string(wantArray) || fields[multi.ID].FieldName != "Original selections" || fields[multi.ID].ValueLabels[b].Label != "Second" || fields[multi.ID].ValueLabels[b].Deleted || fields[multi.ID].ValueLabels[a].Label != "First" {
		t.Fatal("original precision/order/labels were rewritten", fields)
	}
}

func TestRootWorkflowEventSystemStartDoesNotInventApproverRead(t *testing.T) {
	f := rootCaptureSetup(t).recordFixture
	instance, record := rootStartIntent(t, f, f.other)
	if ok, e := f.service.acceptWorkflowStart(f.ctx, instance); e != nil || !ok {
		t.Fatal("actual durable intent not accepted", e)
	}
	c, p := rootStartCommand(t, f, instance)
	projection := rootProjection{recordFixture: f}
	// A start that reaches an active approval uses the exact fixed node/roster.
	var node string
	for id := range p.Start.Approvers {
		node = id
	}
	task := projection.task(t, node, f.other, 1)
	r, b := projection.receipt(t, c, "active", "", task)
	projection.apply(t, c, p, r, b, true)
	owner := f.principal
	owner.UserID = f.other
	owner.SessionRef = f.other
	got, e := f.service.ReadWorkflowEvent(f.ctx, owner, WorkflowEventRequest{f.app, f.view, record, c.CommandID})
	if e != nil || got.Basis.Status != "available" || got.Basis.Fields == nil || len(got.Basis.Fields) != 0 || got.Event.Action != "start" || got.Event.NodeID != nil {
		t.Fatal("system capture presented as human-readable approval basis", got, e)
	}
}

func TestRootWorkflowEventBatchedHistoricalReadCost(t *testing.T) {
	f := rootTaskSetup(t, false)
	base := rootEvidenceStoreFixture{recordFixture: f.recordFixture}
	var defs []appfields.Field
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT fields_json FROM applications.logical_tables WHERE id=$1", f.table).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(raw, &defs); e != nil {
		t.Fatal(e)
	}
	var readGrant string
	if e := f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all' LIMIT 1", f.app).Scan(&readGrant); e != nil {
		t.Fatal(e)
	}
	ids := []string{f.public, f.secret, f.reference}
	for i := 0; i < 32; i++ {
		field := appfields.Field{ID: recordOperationID(t, f.recordFixture), Name: fmt.Sprintf("Historical %02d", i), Kind: "text", Default: json.RawMessage("null"), Config: json.RawMessage(`{"maxLength":null}`)}
		rootCaptureAddField(t, base, field, map[string]any{"Type": "text"})
		defs = append(defs, field)
		ids = append(ids, field.ID)
		eventSQL(t, f, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, readGrant, field.ID, f.table)
		eventSQL(t, f, "UPDATE "+rootCaptureRelation(base)+" SET "+rootCaptureColumn(field.ID)+"=$2 WHERE id=$1", f.ownRecord, fmt.Sprintf("value-%02d", i))
	}
	rootCaptureDefinitions(t, base, defs)
	eventGrant(t, f, "all", ids...)
	q := eventConfirmed(t, f, "agree")
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
	service := *f.service
	service.Pool = pool
	var counts []int32
	var times []float64
	for i := 0; i < 3; i++ {
		trace.count.Store(0)
		start := time.Now()
		got, e := service.ReadWorkflowEvent(f.ctx, f.principal, q)
		ms := float64(time.Since(start).Microseconds()) / 1000
		count := trace.count.Load()
		if e != nil || len(got.Basis.Fields) != 35 {
			t.Fatal("wide authorized basis missing", e)
		}
		if count > 24 {
			t.Fatalf("per-field historical SQL: fields35 statements%d", count)
		}
		fields := eventFields(t, got)
		for j, id := range ids[3:] {
			if string(fields[id].Value) != fmt.Sprintf("%q", fmt.Sprintf("value-%02d", j)) {
				t.Fatal("wide basis identity/value mismatch")
			}
		}
		counts = append(counts, count)
		times = append(times, ms)
	}
	t.Logf("V062_COST fields=35 real_readonly_service_statements_including_tx=%v elapsed_ms=%v; isolated warm PG, not production throughput", counts, times)
}

type eventSnapshotTrace struct {
	sql     string
	changed bool
	change  func() error
	err     error
}

func (s *eventSnapshotTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	s.sql = d.SQL
	return ctx
}
func (s *eventSnapshotTrace) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if !s.changed && strings.Contains(s.sql, "FROM applications.workflow_execution_events e JOIN") {
		s.changed = true
		s.err = s.change()
	}
}
func TestRootWorkflowEventOneSnapshotAndNextRequestRevocation(t *testing.T) {
	f := rootTaskSetup(t, false)
	eventGrant(t, f, "all", f.public)
	q := eventConfirmed(t, f, "agree")
	trace := &eventSnapshotTrace{change: func() error {
		_, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.history')", f.app)
		if e != nil {
			return e
		}
		_, e = f.owner.Exec(f.ctx, "UPDATE applications.workflow_evidence_blobs SET body=set_byte(body,8,0) WHERE app_id=$1 AND field_id=$2", f.app, f.public)
		return e
	}}
	cfg := f.runtime.Config()
	cfg.ConnConfig.Tracer = trace
	cfg.MaxConns = 1
	pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	service := *f.service
	service.Pool = pool
	got, e := service.ReadWorkflowEvent(f.ctx, f.principal, q)
	if !trace.changed || trace.err != nil {
		t.Fatal("concurrent change fixture did not execute", trace.err)
	}
	if e != nil || len(got.Basis.Fields) != 1 || string(got.Basis.Fields[0].Value) != `"alpha"` {
		t.Fatal("mixed pre-revocation event with later corrupted basis", e)
	}
	if _, e = service.ReadWorkflowEvent(f.ctx, f.principal, q); !errors.Is(e, applications.ErrDenied) {
		t.Fatal("new request reused old history permission", e)
	}
	owner := f.principal
	owner.UserID = f.other
	owner.SessionRef = f.other
	if _, e = service.ReadWorkflowEvent(f.ctx, owner, q); !errors.Is(e, ErrUnavailable) {
		t.Fatal("fresh authorized snapshot did not observe injected corruption", e)
	}
}
