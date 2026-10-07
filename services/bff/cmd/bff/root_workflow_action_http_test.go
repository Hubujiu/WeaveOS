package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowprojection"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rootTaskHTTPFixture struct {
	ctx                                                                                           context.Context
	owner, runtime                                                                                *pgxpool.Pool
	store                                                                                         *session.Store
	server                                                                                        *httptest.Server
	client                                                                                        *http.Client
	actor, other, app, table, view, field, record, flow, version, instance, node, task, sid, csrf string
}

func (f *rootTaskHTTPFixture) id(t *testing.T) string {
	t.Helper()
	var id string
	if e := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}
func (f *rootTaskHTTPFixture) transaction(t *testing.T, fn func(pgx.Tx) error) {
	t.Helper()
	tx, e := f.runtime.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if e = fn(tx); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(f.ctx); e != nil {
		t.Fatal(e)
	}
}
func (f *rootTaskHTTPFixture) call(t *testing.T, method, path, body string, edit func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	r, e := http.NewRequestWithContext(f.ctx, method, f.server.URL+path, strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("Origin", f.server.URL)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", f.csrf)
	r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
	r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: f.csrf})
	if edit != nil {
		edit(r)
	}
	response, e := f.client.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.TLS == nil {
		t.Fatal("real HTTPS required")
	}
	raw, e := io.ReadAll(response.Body)
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	for k, v := range response.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(response.StatusCode)
	w.Write(raw)
	return w
}
func rootHTTPData(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]json.RawMessage {
	t.Helper()
	var envelope struct {
		Code string
		Data map[string]json.RawMessage
		Meta struct{ RequestID string }
	}
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &envelope) != nil || envelope.Code != "OK" || envelope.Data == nil || envelope.Meta.RequestID == "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("want%d actual%d %s", status, w.Code, w.Body.String())
	}
	return envelope.Data
}
func rootHTTPString(t *testing.T, m map[string]json.RawMessage, key string) string {
	t.Helper()
	var out string
	if json.Unmarshal(m[key], &out) != nil {
		t.Fatalf("bad %s", key)
	}
	return out
}
func rootHTTPError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var e struct {
		Code string
		Meta struct{ RequestID string }
	}
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != code || e.Meta.RequestID == "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("want%d/%s actual%d %s", status, code, w.Code, w.Body.String())
	}
}
func (f *rootTaskHTTPFixture) root() string {
	return "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records/" + f.record
}
func (f *rootTaskHTTPFixture) taskPath() string {
	return f.root() + "/workflow-instances/" + f.instance + "/tasks/" + f.task
}
func rootHTTPActionBody(op, action, token string) string {
	b, _ := json.Marshal(map[string]string{"operationId": op, "action": action, "basisToken": token})
	return string(b)
}
func rootHTTPResultBytes(result fc.ExecutionResult) []byte {
	b := []byte{'W', 'V', 'F', 'R', 'S', 'L', 0, 1}
	text := func(s string) { b = binary.BigEndian.AppendUint32(b, uint32(len(s))); b = append(b, s...) }
	text(result.InstanceID)
	text(result.EngineProcessID)
	text(result.State)
	text(result.Reason)
	b = binary.BigEndian.AppendUint64(b, uint64(result.SchemaVersion))
	b = binary.BigEndian.AppendUint64(b, uint64(result.RecordVersion))
	b = binary.BigEndian.AppendUint32(b, uint32(len(result.Tasks)))
	for _, task := range result.Tasks {
		text(task.ID)
		text(task.NodeID)
		text(task.AssigneeID)
		text(task.EngineTaskID)
		b = binary.BigEndian.AppendUint64(b, uint64(task.ActivationEpoch))
	}
	return b
}

// This fixture creates a confirmed existing task through trusted internal ports.
// It is HTTP ingress coverage, not evidence of a public start trigger or real engine.
func rootHTTPResourceSetup(t *testing.T) *rootTaskHTTPFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	f := &rootTaskHTTPFixture{ctx: ctx}
	var e error
	f.owner, e = pgxpool.New(ctx, os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.owner.Close)
	runtimeURL, e := url.Parse(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	q := runtimeURL.Query()
	q.Set("options", "-c role=auth_app")
	runtimeURL.RawQuery = q.Encode()
	f.runtime, e = pgxpool.New(ctx, runtimeURL.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.runtime.Close)
	f.actor, f.other, f.app = f.id(t), f.id(t), f.id(t)
	for _, id := range []string{f.actor, f.other} {
		if _, e = f.owner.Exec(ctx, "INSERT INTO auth.users(id,account) VALUES($1,$2)", id, "root-http-task-"+id); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = f.owner.Exec(ctx, "INSERT INTO applications.apps(id,name,owner_user_id) VALUES($1,'Root task HTTP',$2)", f.app, f.actor); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(ctx, "INSERT INTO applications.menu_resources VALUES($1,'application',$1)", f.app); e != nil {
		t.Fatal(e)
	}
	if _, e = f.owner.Exec(ctx, "SELECT applications.register_catalog_entry($1)", f.app); e != nil {
		t.Fatal(e)
	}
	generation := "task-http-" + strings.ReplaceAll(f.app, "-", "")
	f.store = session.NewStore(os.Getenv("WEAVEOS_TEST_REDIS_URL"), generation)
	t.Cleanup(func() { f.store.Close() })
	f.sid, f.csrf, e = f.store.Create(ctx, session.Record{UserID: f.actor, SessionRef: f.id(t), AuthVersion: "1"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.store.Revoke(context.Background(), f.sid) })
	f.server = httptest.NewUnstartedServer(nil)
	cfg := config{DatabaseURL: runtimeURL.String(), RedisURL: os.Getenv("WEAVEOS_TEST_REDIS_URL"), Origin: "https://" + f.server.Listener.Addr().String(), Generation: generation, AuditKeyID: "test", AuditKey: []byte("synthetic-audit-key-32-bytes-only!"), DefinitionKeyID: "test", DefinitionKey: []byte("isolated-definition-test-32bytes!"), SchemaLimits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}}
	h, close, e := buildHandler(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(close)
	f.server.Config.Handler = h
	f.server.StartTLS()
	t.Cleanup(f.server.Close)
	f.client = f.server.Client()
	form := rootHTTPData(t, f.call(t, "POST", "/api/v1/applications/"+f.app+"/forms", `{"operationId":"`+f.id(t)+`","name":"Approval","source":{"kind":"new_table"},"directoryId":null,"position":0,"expectedStructureVersion":0}`, nil), 201)
	var view, table struct{ ID string }
	if json.Unmarshal(form["form"], &view) != nil || json.Unmarshal(form["table"], &table) != nil {
		t.Fatal("form setup")
	}
	f.view, f.table = view.ID, table.ID
	f.field = f.id(t)
	definition := `{"operationId":"` + f.id(t) + `","expectedSchemaVersion":0,"expectedViewVersion":0,"fields":[{"id":"` + f.field + `","name":"Reason","kind":"text","required":false,"default":null,"config":{"maxLength":null},"presentation":{"helpText":null,"displayTimeZone":null}}],"layout":[],"optionMappings":[],"confirmationToken":null}`
	rootHTTPData(t, f.call(t, "PUT", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/definition", definition, nil), 200)
	created := rootHTTPData(t, f.call(t, "POST", "/api/v1/applications/"+f.app+"/forms/"+f.view+"/records", `{"operationId":"`+f.id(t)+`","expectedSchemaVersion":1,"values":{"`+f.field+`":"original"}}`, nil), 201)
	f.record = rootHTTPString(t, created, "id")
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = f.owner.Exec(cleanup, "UPDATE applications.workflow_dispatch SET next_attempt_at='9999-12-31T00:00:00Z' WHERE command_id IN(SELECT command_id FROM applications.workflow_commands WHERE command_json->>'AppID'=$1)", f.app)
	})
	return f
}
func rootHTTPTaskSetup(t *testing.T) *rootTaskHTTPFixture {
	t.Helper()
	return rootHTTPTaskSetupEditable(t, false)
}

func rootHTTPTaskSetupEditable(t *testing.T, editable bool) *rootTaskHTTPFixture {
	t.Helper()
	f := rootHTTPResourceSetup(t)
	ctx := f.ctx
	var e error
	f.flow, f.instance, f.node, f.task = f.id(t), f.id(t), f.id(t), f.id(t)
	start, end := f.id(t), f.id(t)
	graph := flowgraph.Graph{Version: 1, Nodes: []flowgraph.Node{{ID: start, Kind: "start"}, {ID: f.node, Kind: "approval", Approval: &flowgraph.Approval{Mode: "all", AssigneeIDs: []string{f.actor}}}, {ID: end, Kind: "end"}}, Edges: []flowgraph.Edge{{From: start, To: f.node}, {From: f.node, To: end}}}
	if editable {
		graph.Nodes[1].Approval.EditableFieldIDs = []string{f.field}
	}
	f.transaction(t, func(tx pgx.Tx) error {
		catalog := wc.Catalog{}
		head, e := catalog.PutVersionInTx(ctx, tx, wc.VersionInput{AppID: f.app, TableID: f.table, ViewID: f.view, FlowID: f.flow, ActorID: f.actor, Name: "Approval", ExpectedSchemaVersion: 1, Graph: graph, AllowWithdraw: true})
		if e != nil {
			return e
		}
		head, e = catalog.ConfirmDeploymentInTx(ctx, tx, f.app, f.flow, 1, "root-HTTP-fixture-deployment")
		if e != nil {
			return e
		}
		head, e = catalog.EnableInTx(ctx, tx, f.app, f.flow, head.Revision)
		if e != nil {
			return e
		}
		_, e = catalog.ReserveInTx(ctx, tx, wc.ReserveInput{AppID: f.app, FlowID: f.flow, InstanceID: f.instance, RecordID: f.record, ActorID: f.actor, ExpectedRevision: head.Revision, ExpectedSchemaVersion: 1, ExpectedRecordVersion: 1})
		return e
	})
	if e = f.owner.QueryRow(ctx, "SELECT version_id::text FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=1", f.app, f.flow).Scan(&f.version); e != nil {
		t.Fatal(e)
	}
	command := fc.Command{ProtocolVersion: 2, CommandID: f.id(t), AppID: f.app, TableID: f.table, ViewID: f.view, RecordID: f.record, InstanceID: f.instance, ActorID: f.actor, Action: "start", RecordVersion: 1, SchemaVersion: 1, FlowID: f.flow, DefinitionVersion: 1, VersionID: f.version}
	payload := fc.ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("HTTP fixture existing task")), Start: &fc.ExecutionStart{AllowWithdraw: true, Approvers: map[string][]string{f.node: {f.actor}}}, Routes: map[string]bool{}}
	raw, e := fc.EncodeExecutionPayload("start", payload)
	if e != nil {
		t.Fatal(e)
	}
	command.PayloadHash = sha256.Sum256(raw)
	f.transaction(t, func(tx pgx.Tx) error {
		if e := tx.QueryRow(ctx, "SELECT applications.acquire_record_command_fence($1,$2,$3,$4,$5,$6,$7)", f.app, f.table, f.view, f.record, command.CommandID, 1, 1).Scan(&command.FenceEpoch); e != nil {
			return e
		}
		_, e := (fc.Ledger{Namespace: "applications"}).AcceptExecutionInTx(ctx, tx, command, payload)
		return e
	})
	body := rootHTTPResultBytes(fc.ExecutionResult{InstanceID: f.instance, EngineProcessID: "root-http-" + f.instance, State: "active", SchemaVersion: 1, RecordVersion: 1, Tasks: []fc.ExecutionTask{{ID: f.task, NodeID: f.node, AssigneeID: f.actor, EngineTaskID: "root-http-task-" + f.task, ActivationEpoch: 1}}})
	hash, e := fc.Fingerprint(command)
	if e != nil {
		t.Fatal(e)
	}
	receipt := fc.Receipt{CommandID: command.CommandID, CommandHash: hash, Outcome: "success", Sequence: 1, ProofID: f.id(t), ResultHash: sha256.Sum256(body)}
	f.transaction(t, func(tx pgx.Tx) error {
		_, e := (workflowprojection.Store{}).ApplyInTx(ctx, tx, command, payload, receipt, body)
		return e
	})
	return f
}
func TestRootWorkflowActionHTTPRealHostPreviewAcceptedAndStatus(t *testing.T) {
	for _, action := range []string{"agree", "reject"} {
		t.Run(action, func(t *testing.T) {
			f := rootHTTPTaskSetup(t)
			preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
			token := rootHTTPString(t, preview, "basisToken")
			if len(preview) != 5 || token == "" || string(preview["editableFieldIds"]) != "[]" {
				t.Fatal("preview DTO not exact")
			}
			op := f.id(t)
			body := rootHTTPActionBody(op, action, token)
			w := f.call(t, "POST", f.taskPath()+"/actions", body, nil)
			accepted := rootHTTPData(t, w, 202)
			if !strings.Contains(w.Body.String(), "操作处理中") {
				t.Fatal("pending response falsely says success")
			}
			if len(accepted) != 4 || rootHTTPString(t, accepted, "status") != "pending" || rootHTTPString(t, accepted, "operationId") != op {
				t.Fatal("accepted action presented final success")
			}
			location := "/api/v1/application-workflow-operations/" + op
			if w.Header().Get("Location") != location {
				t.Fatal("missing exact polling location")
			}
			first := rootHTTPString(t, accepted, "commandId")
			status := rootHTTPData(t, f.call(t, "GET", location, "", nil), 200)
			if rootHTTPString(t, status, "commandId") != first || rootHTTPString(t, status, "status") != "pending" {
				t.Fatal("status route guessed terminal state")
			}
			again := rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", body, nil), 202)
			if rootHTTPString(t, again, "commandId") != first {
				t.Fatal("HTTP retry duplicated command")
			}
			generic := rootHTTPData(t, f.call(t, "GET", "/api/v1/application-operations/"+op, "", nil), 200)
			var httpStatus int
			var receipt map[string]json.RawMessage
			if json.Unmarshal(generic["httpStatus"], &httpStatus) != nil || httpStatus != 202 || json.Unmarshal(generic["result"], &receipt) != nil || rootHTTPString(t, receipt, "status") != "pending" {
				t.Fatal("generic operation lost pending acceptance meaning")
			}
			var count int
			if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1", f.app).Scan(&count); e != nil || count != 2 {
				t.Fatal("wrong command count", e)
			}
		})
	}
}
func TestRootWorkflowActionHTTPRejectsAuthenticationCSRFAndActorMismatch(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	for _, item := range []struct {
		name   string
		status int
		code   string
		edit   func(*http.Request)
	}{
		{"no-session", 401, "AUTH_UNAUTHENTICATED", func(r *http.Request) { r.Header.Del("Cookie") }},
		{"csrf", 403, "COMMON_CSRF_REJECTED", func(r *http.Request) { r.Header.Set("X-CSRF-Token", "invalid") }},
		{"actor", 409, "AUTH_SESSION_CHANGED", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }},
	} {
		t.Run(item.name, func(t *testing.T) {
			rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", token), item.edit), item.status, item.code)
		})
	}
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_evidence_documents WHERE app_id=$1", f.app).Scan(&n); e != nil || n != 0 {
		t.Fatal("rejected HTTP wrote evidence", e)
	}
}
func TestRootWorkflowActionHTTPClosedBodyAndExactRoutes(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	op := f.id(t)
	good := rootHTTPActionBody(op, "agree", token)
	invalid := []string{strings.TrimSuffix(good, "}") + `,"actorId":"` + f.actor + `"}`, strings.TrimSuffix(good, "}") + `,"action":"agree"}`, strings.TrimSuffix(good, "}") + `,"routes":{}}`, strings.TrimSuffix(good, "}") + `,"values":{}}`, good + `{}`, `null`, `{"operationId":"` + op + `","action":"start","basisToken":"` + token + `"}`, `{"operationId":"` + op + `","action":null,"basisToken":"` + token + `"}`, `{"operationId":"` + op + `","action":"agree","basisToken":42}`, strings.Repeat(" ", 4097) + good}
	for i, body := range invalid {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", body, nil), 400, "COMMON_VALIDATION_FAILED")
		})
	}
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", good, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }), 415, "COMMON_UNSUPPORTED_MEDIA_TYPE")
	rootHTTPError(t, f.call(t, "GET", f.taskPath()+"?x=1", "", nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions?x=1", good, nil), 400, "COMMON_VALIDATION_FAILED")
	rootHTTPError(t, f.call(t, "PATCH", f.taskPath()+"/actions", good, nil), 404, "API_NOT_FOUND")
	rootHTTPError(t, f.call(t, "GET", f.taskPath()+"/unknown", "", nil), 404, "API_NOT_FOUND")
	rootHTTPError(t, f.call(t, "GET", "/api/v1/application-workflow-operations/"+f.id(t), "", nil), 404, "APPLICATION_NOT_FOUND")
	var n int
	if e := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_kind LIKE 'workflow.task.%'", f.app).Scan(&n); e != nil || n != 0 {
		t.Fatal("invalid requests claimed operations", e)
	}
}
func TestRootWorkflowActionHTTPStaleBasisRequiresRefresh(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	preview := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	token := rootHTTPString(t, preview, "basisToken")
	rootHTTPData(t, f.call(t, "PATCH", f.root(), `{"operationId":"`+f.id(t)+`","expectedSchemaVersion":1,"expectedRecordVersion":1,"changes":{"`+f.field+`":"changed after viewing"}}`, nil), 200)
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", token), nil), 409, "WORKFLOW_BASIS_CHANGED")
	rootHTTPError(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", strings.Repeat("A", 43)), nil), 409, "WORKFLOW_BASIS_EXPIRED")
	fresh := rootHTTPData(t, f.call(t, "GET", f.taskPath(), "", nil), 200)
	rootHTTPData(t, f.call(t, "POST", f.taskPath()+"/actions", rootHTTPActionBody(f.id(t), "agree", rootHTTPString(t, fresh, "basisToken")), nil), 202)
}
