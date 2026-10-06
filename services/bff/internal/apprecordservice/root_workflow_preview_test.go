package apprecordservice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type rootTaskFixture struct {
	rootProjection
	task fc.ExecutionTask
}

// Keep intentionally pending commands as evidence without feeding another
// package's global scheduler after this synthetic fixture has finished.
func rootTaskKeepDispatchPrivate(t *testing.T, f recordFixture) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := f.owner.Exec(ctx, `UPDATE applications.workflow_dispatch d
 SET next_attempt_at='9999-01-01T00:00:00Z'
 FROM applications.workflow_commands c
 WHERE c.command_id=d.command_id AND c.command_json->>'AppID'=$1`, f.app); err != nil {
			t.Errorf("preserve private pending-command fixture: %v", err)
		}
	})
}

func rootTaskSetup(t *testing.T, foreign bool) rootTaskFixture {
	t.Helper()
	base := rootCaptureSetup(t)
	f := base.recordFixture
	rootTaskKeepDispatchPrivate(t, f)
	if foreign {
		f.ownRecord = f.otherRecord
	}
	graph := rootCatalogGraph(t, f, false)
	h := rootCatalogReady(t, f, graph)
	instance := rootCatalogReserve(t, f, rootCatalogReserveInput(t, f, h))
	var version string
	if e := f.owner.QueryRow(f.ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, h.FlowID).Scan(&version); e != nil {
		t.Fatal(e)
	}
	p := rootProjection{recordFixture: f, head: h, instance: instance, version: version, first: graph.Nodes[1].ID, second: graph.Nodes[1].ID}
	c, payload := rootRecoveryAccept(t, p)
	task := p.task(t, p.first, p.actor, 1)
	receipt, body := p.receipt(t, c, "active", "", task)
	p.apply(t, c, payload, receipt, body, true)
	return rootTaskFixture{rootProjection: p, task: task}
}
func (f rootTaskFixture) request() WorkflowTaskRequest {
	return WorkflowTaskRequest{AppID: f.app, ViewID: f.view, RecordID: f.ownRecord, InstanceID: f.instance.ID, TaskID: f.task.ID}
}
func rootTaskPreview(t *testing.T, f rootTaskFixture) WorkflowTaskPreview {
	t.Helper()
	got, e := f.service.PreviewWorkflowTask(f.ctx, f.principal, f.request())
	if e != nil {
		t.Fatal(e)
	}
	return got
}
func rootTaskRedis(t *testing.T) *redis.Client {
	t.Helper()
	o, e := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	r := redis.NewClient(o)
	if e = r.Ping(context.Background()).Err(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	return r
}
func rootTaskBasis(t *testing.T, f rootTaskFixture, token string) querycontext.Metadata {
	t.Helper()
	if f.service.WorkflowBases == nil {
		t.Fatal("workflow basis store was not initialized")
	}
	m, e := f.service.WorkflowBases.Load(f.ctx, f.principal.SessionRef, token)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func rootTaskNoEvidenceWrites(t *testing.T, f rootTaskFixture) {
	t.Helper()
	for _, table := range []string{"workflow_evidence_blobs", "workflow_evidence_documents", "workflow_evidence_members"} {
		var n int
		if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications."+table+" WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 0 {
			t.Fatalf("preview wrote %s: %d %v", table, n, e)
		}
	}
	var pending int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1", f.app).Scan(&pending); e != nil || pending != 0 {
		t.Fatal("preview created a command fence", e)
	}
}
func TestRootWorkflowPreviewExposesOnlyCurrentReadableFields(t *testing.T) {
	f := rootTaskSetup(t, true)
	p := rootTaskPreview(t, f)
	if p.Record.ID != f.ownRecord || p.Record.AppID != f.app || p.Record.TableID != f.table || p.Record.ViewID != f.view || p.Record.CreatedBy != f.other || p.Record.RecordVersion != 1 || p.Record.SchemaVersion != 1 {
		t.Fatal("preview record identity or versions do not describe actual row")
	}
	if p.Task.ID != f.task.ID || p.Task.InstanceID != f.instance.ID || p.Task.NodeID != f.first || p.Task.ActivationEpoch != 1 || p.Task.Sequence != 1 {
		t.Fatal("preview task identity or version wrong")
	}
	if len(p.Record.Values) != 2 || p.Record.Values[f.public] != "beta" || p.Record.Values[f.reference] != f.other {
		t.Fatal("wrong visible actual record values")
	}
	if _, exists := p.Record.Values[f.secret]; exists {
		t.Fatal("own-scope secret exposed on foreign row")
	}
	ids := []string{}
	for _, field := range p.Fields {
		ids = append(ids, field.ID)
	}
	sort.Strings(ids)
	want := []string{f.public, f.reference}
	sort.Strings(want)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("field definitions leaked or missing: %v", ids)
	}
	display := p.Record.ReferenceDisplays[f.reference][f.other]
	if display.ID != f.other || display.Label == "" || display.Deleted {
		t.Fatal("visible source display missing")
	}
	raw, e := json.Marshal(p)
	if e != nil || bytes.Contains(raw, []byte("private beta")) || bytes.Contains(raw, []byte(f.secret)) || bytes.Contains(raw, []byte("evidenceHash")) || bytes.Contains(raw, []byte("Fingerprint")) {
		t.Fatal("public preview leaked hidden evidence")
	}
	token, e := base64.RawURLEncoding.DecodeString(p.BasisToken)
	if e != nil || len(token) != 32 || len(p.BasisToken) != 43 {
		t.Fatal("basis token is not random opaque 32 bytes")
	}
	rootTaskNoEvidenceWrites(t, f)
}
func TestRootWorkflowPreviewTokenBindsActualEvidenceTaskAndSession(t *testing.T) {
	f := rootTaskSetup(t, false)
	p := rootTaskPreview(t, f)
	m := rootTaskBasis(t, f, p.BasisToken)
	if m.View != "workflow-task" || m.Total != 0 || m.ProtocolVersion != 1 {
		t.Fatal("wrong immutable token discriminator")
	}
	var binding struct {
		AppID             string `json:"appId"`
		TableID           string `json:"tableId"`
		ViewID            string `json:"viewId"`
		RecordID          string `json:"recordId"`
		InstanceID        string `json:"instanceId"`
		TaskID            string `json:"taskId"`
		NodeID            string `json:"nodeId"`
		VersionID         string `json:"versionId"`
		DefinitionVersion int64  `json:"definitionVersion"`
		Sequence          int64  `json:"sequence"`
		ActivationEpoch   int64  `json:"activationEpoch"`
	}
	if json.Unmarshal(m.Criteria, &binding) != nil || binding.AppID != f.app || binding.TableID != f.table || binding.ViewID != f.view || binding.RecordID != f.ownRecord || binding.InstanceID != f.instance.ID || binding.TaskID != f.task.ID || binding.NodeID != f.first || binding.VersionID != f.version || binding.DefinitionVersion != 1 || binding.Sequence != 1 || binding.ActivationEpoch != 1 {
		t.Fatal("token not bound to fixed definition and actual task")
	}
	var rev struct {
		SchemaVersion int64 `json:"schemaVersion"`
		RecordVersion int64 `json:"recordVersion"`
	}
	if json.Unmarshal(m.Revision, &rev) != nil || rev.SchemaVersion != 1 || rev.RecordVersion != 1 {
		t.Fatal("token record/schema revision absent")
	}
	tx, facts, e := (&applications.Application{Pool: f.runtime}).BeginRecordRead(f.ctx, f.principal, f.app, f.view)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(f.ctx)
	b, e := captureWorkflowEvidence(f.ctx, tx, facts, f.ownRecord)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
	if m.Fingerprint != hex.EncodeToString(b.Hash[:]) || p.BasisToken == m.Fingerprint {
		t.Fatal("token stores wrong actual evidence or exposes deterministic hash")
	}
	if bytes.Contains(m.Criteria, []byte("private alpha")) || len(m.Criteria) > 2048 || len(m.Revision) > 128 {
		t.Fatal("token copied record payload or unbounded metadata")
	}
	if _, e = f.service.WorkflowBases.Load(f.ctx, recordOperationID(t, f.recordFixture), p.BasisToken); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatalf("token crossed session: %v", e)
	}
	another := rootTaskPreview(t, f)
	if another.BasisToken == p.BasisToken {
		t.Fatal("preview token reused deterministically")
	}
}
func TestRootWorkflowPreviewRequiresCurrentSessionMenuAndActualRowScope(t *testing.T) {
	for _, name := range []string{"inactive", "no-menu", "no-data", "own-on-foreign"} {
		t.Run(name, func(t *testing.T) {
			f := rootTaskSetup(t, true)
			want := applications.ErrDenied
			switch name {
			case "inactive":
				if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled' WHERE id=$1", f.actor); e != nil {
					t.Fatal(e)
				}
				want = session.ErrUnauthorized
			case "no-menu":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='menu.enter'", f.app); e != nil {
					t.Fatal(e)
				}
			case "no-data":
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read')", f.app); e != nil {
					t.Fatal(e)
				}
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read'", f.app); e != nil {
					t.Fatal(e)
				}
			case "own-on-foreign":
				want = applications.ErrMissing
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN(SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='own')", f.app); e != nil {
					t.Fatal(e)
				}
				if _, e := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='own'", f.app); e != nil {
					t.Fatal(e)
				}
				if _, e := f.owner.Exec(f.ctx, "UPDATE applications.grants SET row_scope='own' WHERE app_id=$1 AND action='data.read'", f.app); e != nil {
					t.Fatal(e)
				}
			}
			got, e := f.service.PreviewWorkflowTask(f.ctx, f.principal, f.request())
			if !errors.Is(e, want) || got.BasisToken != "" || len(got.Record.Values) != 0 {
				t.Fatalf("%s allowed preview: %v", name, e)
			}
			rootTaskNoEvidenceWrites(t, f)
		})
	}
}
func TestRootWorkflowPreviewTaskAssignmentIsNotOverriddenByAppOwner(t *testing.T) {
	f := rootTaskSetup(t, false)
	owner := session.Principal{UserID: f.other, SessionRef: recordOperationID(t, f.recordFixture), Record: session.Record{AuthVersion: "1"}}
	got, e := f.service.PreviewWorkflowTask(f.ctx, owner, f.request())
	if !errors.Is(e, applications.ErrDenied) || got.BasisToken != "" {
		t.Fatalf("owner bypassed actual assignee: %v", e)
	}
}
func TestRootWorkflowPreviewRejectsWrongResourceAndClosedTask(t *testing.T) {
	for _, key := range []string{"app", "view", "record", "instance", "task"} {
		t.Run(key, func(t *testing.T) {
			f := rootTaskSetup(t, false)
			req := f.request()
			id := recordOperationID(t, f.recordFixture)
			switch key {
			case "app":
				req.AppID = id
			case "view":
				req.ViewID = id
			case "record":
				req.RecordID = f.otherRecord
			case "instance":
				req.InstanceID = id
			case "task":
				req.TaskID = id
			}
			got, e := f.service.PreviewWorkflowTask(f.ctx, f.principal, req)
			if !errors.Is(e, applications.ErrMissing) || got.BasisToken != "" {
				t.Fatalf("scope splice %s: %v", key, e)
			}
		})
	}
	t.Run("closed", func(t *testing.T) {
		f := rootTaskSetup(t, false)
		c, p := f.accept(t, "agree", f.task.ID, "", 1, 1)
		r, b := f.receipt(t, c, "completed", "")
		f.apply(t, c, p, r, b, true)
		got, e := f.service.PreviewWorkflowTask(f.ctx, f.principal, f.request())
		if !errors.Is(e, ErrWorkflowTaskChanged) || got.BasisToken != "" {
			t.Fatalf("closed task still actionable: %v", e)
		}
	})
}
func TestRootWorkflowPreviewRefreshTracksRelatedDisplaysButNotUnrelatedChanges(t *testing.T) {
	f := rootTaskSetup(t, false)
	first := rootTaskPreview(t, f)
	a := rootTaskBasis(t, f, first.BasisToken)
	if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", f.actor, "unrelated-"+f.actor); e != nil {
		t.Fatal(e)
	}
	second := rootTaskPreview(t, f)
	b := rootTaskBasis(t, f, second.BasisToken)
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("unrelated source globally invalidated record basis")
	}
	if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", f.other, "related-"+f.other); e != nil {
		t.Fatal(e)
	}
	third := rootTaskPreview(t, f)
	c := rootTaskBasis(t, f, third.BasisToken)
	if c.Fingerprint == a.Fingerprint || third.Record.ReferenceDisplays[f.reference][f.other].Label != "related-"+f.other || third.Record.RecordVersion != first.Record.RecordVersion {
		t.Fatal("related display change was invisible without record version change")
	}
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_execution_events WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 1 {
		t.Fatal("preview advanced workflow", e)
	}
}
func TestRootWorkflowPreviewClosedDefinitionDoesNotBlockExistingTask(t *testing.T) {
	f := rootTaskSetup(t, false)
	rootCatalogTx(t, f.recordFixture, func(tx pgx.Tx) error {
		h, e := (workflowcatalog.Catalog{}).RequestCloseInTx(f.ctx, tx, f.app, f.head.FlowID, f.head.Revision)
		if e == nil && h.State != "closing" {
			t.Fatal("fixture should remain closing")
		}
		return e
	})
	p := rootTaskPreview(t, f)
	if p.Task.ID != f.task.ID {
		t.Fatal("closing disabled existing task")
	}
}
func TestRootWorkflowPreviewLRUAndRedisFailureNeverMutateBusinessState(t *testing.T) {
	f := rootTaskSetup(t, false)
	first := rootTaskPreview(t, f)
	for i := 0; i < 20; i++ {
		rootTaskPreview(t, f)
	}
	if _, e := f.service.WorkflowBases.Load(f.ctx, f.principal.SessionRef, first.BasisToken); !errors.Is(e, querycontext.ErrExpired) {
		t.Fatalf("basis LRU unbounded: %v", e)
	}
	prefix, e := f.service.WorkflowBases.Prefix(f.principal.SessionRef)
	if e != nil {
		t.Fatal(e)
	}
	client := rootTaskRedis(t)
	if n, e := client.ZCard(f.ctx, prefix+"lru").Result(); e != nil || n != 20 {
		t.Fatalf("session token count %d %v", n, e)
	}
	ttl, e := client.PTTL(f.ctx, prefix+"lru").Result()
	if e != nil || ttl < 29*time.Minute || ttl > 30*time.Minute {
		t.Fatal("wrong basis TTL", ttl, e)
	}
	broken := rootTaskRedis(t)
	if e = broken.Close(); e != nil {
		t.Fatal(e)
	}
	service := New(f.runtime, broken, "failed-basis")
	service.Limits = f.service.Limits
	got, e := service.PreviewWorkflowTask(f.ctx, f.principal, f.request())
	if e == nil || got.BasisToken != "" || len(got.Record.Values) != 0 {
		t.Fatal("Redis failure returned a successful partial preview")
	}
	rootTaskNoEvidenceWrites(t, f)
}
func TestRootWorkflowPreviewRetainsVisibleRemovedOptionAndExactNumber(t *testing.T) {
	f := rootTaskSetup(t, false)
	base := rootEvidenceStoreFixture{recordFixture: f.recordFixture}
	var defs []appfields.Field
	var raw []byte
	if e := f.owner.QueryRow(f.ctx, "SELECT fields_json FROM applications.logical_tables WHERE app_id=$1 AND id=$2", f.app, f.table).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if json.Unmarshal(raw, &defs) != nil {
		t.Fatal("fixture definitions")
	}
	amount := appfields.Field{ID: recordOperationID(t, f.recordFixture), Name: "Exact amount", Kind: "money", Default: json.RawMessage("null"), Config: json.RawMessage(`{"precision":38,"scale":18,"roundingPlaces":0,"roundingMode":"TOWARD_ZERO"}`)}
	option := recordOperationID(t, f.recordFixture)
	choice := appfields.Field{ID: recordOperationID(t, f.recordFixture), Name: "Choice", Kind: "single_select", Default: json.RawMessage("null"), Config: json.RawMessage(`{"options":[{"id":"` + option + `","label":"Original label"}]}`)}
	rootCaptureAddField(t, base, amount, map[string]any{"Type": "numeric", "Precision": 38, "Scale": 18})
	rootCaptureAddField(t, base, choice, map[string]any{"Type": "uuid"})
	defs = append(defs, amount, choice)
	rootCaptureDefinitions(t, base, defs)
	exact := "12345678901234567890.123456789012345678"
	q := "UPDATE " + rootCaptureRelation(base) + " SET " + rootCaptureColumn(amount.ID) + "=$2::numeric," + rootCaptureColumn(choice.ID) + "=$3::uuid WHERE id=$1"
	if _, e := f.owner.Exec(f.ctx, q, f.ownRecord, exact, option); e != nil {
		t.Fatal(e)
	}
	defs[len(defs)-1].Config = json.RawMessage(`{"options":[]}`)
	rootCaptureDefinitions(t, base, defs)
	var grant string
	if e := f.owner.QueryRow(f.ctx, "SELECT id::text FROM applications.grants WHERE app_id=$1 AND action='data.read' AND row_scope='all'", f.app).Scan(&grant); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{amount.ID, choice.ID} {
		if _, e := f.owner.Exec(f.ctx, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, id, f.table); e != nil {
			t.Fatal(e)
		}
	}
	p := rootTaskPreview(t, f)
	if p.Record.Values[amount.ID] != exact || p.Record.Values[choice.ID] != option {
		t.Fatal("preview re-normalized stored value")
	}
	d := p.Record.ReferenceDisplays[choice.ID][option]
	if d.ID != option || d.Label != "Original label" || !d.Deleted {
		t.Fatal("removed selected label missing")
	}
	for _, def := range p.Fields {
		if def.ID == choice.ID && strings.Contains(string(def.Config), option) {
			t.Fatal("tombstone made active again")
		}
	}
}
