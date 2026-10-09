package apprecordservice

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
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
