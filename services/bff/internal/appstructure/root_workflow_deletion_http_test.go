package appstructure

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func deletionHTTPBody(t *testing.T, p rootPublicationFixture) (string, string) {
	t.Helper()
	in := deletionInput(t, p)
	raw, _ := json.Marshal(map[string]any{"operationId": in.OperationID, "expectedRevision": in.ExpectedRevision})
	return string(raw), in.OperationID
}
func TestRootDeletionHTTPAcceptedReplayAndCommittedStatus(t *testing.T) {
	p := deletionFixture(t)
	peer := &deletionPeer{}
	p.app.DeletionClient = peer
	raw, op := deletionHTTPBody(t, p)
	path := rootWorkflowHTTPPath(p.s, p.flow, "delete")
	statusPath := rootWorkflowHTTPPath(p.s, p.flow, "deletions/"+op)
	post := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, true, true, "", nil)
	pending := data(t, post, 202)
	if len(pending) != 7 || pending["operationId"] != op || pending["flowId"] != p.flow || pending["status"] != "pending" || pending["reason"] != nil || pending["deletedVersions"] != nil || pending["deletedAt"] != nil || pending["completedAt"] != nil {
		t.Fatal("acceptance is not exact pending DTO", pending)
	}
	if post.Header().Get("Location") != statusPath {
		t.Fatal("accepted response has no original status location", post.Header())
	}
	if replay := data(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, true, true, "", nil), 202); !reflect.DeepEqual(replay, pending) {
		t.Fatal("original request changed pending receipt")
	}
	if got := data(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "GET", statusPath, "", true, false, "", nil), 200); !reflect.DeepEqual(got, pending) {
		t.Fatal("status endpoint does not read durable intent")
	}
	var initial string
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT row_to_json(o)::text FROM applications.operations o WHERE actor_user_id=$1 AND operation_id=$2", p.s.f.actor, op).Scan(&initial); e != nil {
		t.Fatal("deletion did not reserve original application operation namespace", e)
	}
	dispatchDeletion(t, p.app)
	completed := data(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "GET", statusPath, "", true, false, "", nil), 200)
	if len(completed) != 7 || completed["status"] != "deleted" || completed["deletedVersions"] != float64(1) || completed["deletedAt"] == nil || completed["completedAt"] == nil {
		t.Fatal("status fabricated or lost completion", completed)
	}
	p.app.DeletionClient = nil
	if replay := data(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, true, true, "", nil), 200); !reflect.DeepEqual(replay, completed) {
		t.Fatal("completed receipt lost after runtime unavailable")
	}
	var after string
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT row_to_json(o)::text FROM applications.operations o WHERE actor_user_id=$1 AND operation_id=$2", p.s.f.actor, op).Scan(&after); e != nil || after != initial {
		t.Fatal("worker overwrote original acceptance operation", e)
	}
}
func TestRootDeletionHTTPRejectsInvalidBodyWithoutWrites(t *testing.T) {
	for _, kind := range []string{"extra", "missing", "null", "duplicate", "zero-operation", "unsafe-revision", "fractional", "string-revision"} {
		t.Run(kind, func(t *testing.T) {
			p := deletionFixture(t)
			p.app.DeletionClient = &deletionPeer{}
			raw, op := deletionHTTPBody(t, p)
			switch kind {
			case "extra":
				raw = strings.TrimSuffix(raw, "}") + `,"engineReceipt":{}}`
			case "missing":
				raw = `{"operationId":"` + op + `"}`
			case "null":
				raw = `{"operationId":"` + op + `","expectedRevision":null}`
			case "duplicate":
				raw = strings.TrimSuffix(raw, "}") + `,"expectedRevision":1}`
			case "zero-operation":
				raw = strings.ReplaceAll(raw, op, "00000000-0000-0000-0000-000000000000")
			case "unsafe-revision":
				raw = `{"operationId":"` + op + `","expectedRevision":9007199254740992}`
			case "fractional":
				raw = `{"operationId":"` + op + `","expectedRevision":1.5}`
			case "string-revision":
				raw = `{"operationId":"` + op + `","expectedRevision":"1"}`
			}
			w := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", rootWorkflowHTTPPath(p.s, p.flow, "delete"), raw, true, true, "", nil)
			if w.Code != 400 {
				t.Fatalf("invalid %s returned %d: %s", kind, w.Code, w.Body.String())
			}
			var count int
			if e := p.s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.workflow_deletions WHERE flow_id=$1", p.flow).Scan(&count); e != nil || count != 0 {
				t.Fatal("invalid request mutated state", count, e)
			}
		})
	}
}
func TestRootDeletionHTTPCurrentIdentityAndOwnership(t *testing.T) {
	p := deletionFixture(t)
	p.app.DeletionClient = &deletionPeer{}
	raw, op := deletionHTTPBody(t, p)
	path := rootWorkflowHTTPPath(p.s, p.flow, "delete")
	statusPath := rootWorkflowHTTPPath(p.s, p.flow, "deletions/"+op)
	for _, c := range []struct {
		login, csrf bool
		status      int
	}{{false, false, 401}, {true, false, 403}} {
		w := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, c.login, c.csrf, "", nil)
		if w.Code != c.status {
			t.Fatal("authentication boundary", w.Code, c.status)
		}
	}
	w := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, true, true, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", uuid(t, p.s.f.owner)) })
	if w.Code != 409 {
		t.Fatal("expected actor not enforced", w.Code)
	}
	// Application ownership is immutable. Use the existing Bootstrap Admin
	// capability to exercise actual loss of management without altering ownership.
	var other string
	if e := p.s.f.owner.QueryRow(context.Background(), `WITH new_admin AS (
 INSERT INTO auth.users(account,is_bootstrap_admin) SELECT 'deletion-admin-'||gen_random_uuid(),true
 WHERE NOT EXISTS(SELECT 1 FROM auth.users WHERE is_bootstrap_admin) RETURNING id)
 SELECT id::text FROM new_admin UNION ALL SELECT id::text FROM auth.users WHERE is_bootstrap_admin LIMIT 1`).Scan(&other); e != nil {
		t.Fatal(e)
	}
	data(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, true, true, other, nil), 202)
	if got := rootWorkflowHTTPRequest(t, p.s, p.handler, "GET", statusPath, "", true, false, "", nil); got.Code != 404 {
		t.Fatal("another current manager stole original actor result", got.Code)
	}
	if _, e := p.s.f.owner.Exec(context.Background(), "UPDATE auth.users SET is_bootstrap_admin=false WHERE id=$1", other); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = p.s.f.owner.Exec(context.Background(), "UPDATE auth.users SET is_bootstrap_admin=true,status='active' WHERE id=$1", other)
	})
	if got := rootWorkflowHTTPRequest(t, p.s, p.handler, "GET", statusPath, "", true, false, other, nil); got.Code != 403 {
		t.Fatal("revoked manager could read original deletion", got.Code)
	}
	if _, e := p.s.f.owner.Exec(context.Background(), "UPDATE auth.users SET status='disabled' WHERE id=$1", other); e != nil {
		t.Fatal(e)
	}
	dispatchDeletion(t, p.app)
	if s, _ := deletionState(t, p); s != "deleted" {
		t.Fatal("already authorized durable command required old live session", s)
	}
}
func TestRootDeletionHTTPOperationNamespaceAndChangedScope(t *testing.T) {
	p := deletionFixture(t)
	p.app.DeletionClient = &deletionPeer{}
	raw, op := deletionHTTPBody(t, p)
	path := rootWorkflowHTTPPath(p.s, p.flow, "delete")
	var old string
	if e := p.s.f.owner.QueryRow(context.Background(), "SELECT operation_id::text FROM applications.operations WHERE app_id=$1 AND operation_kind='workflow.definition.save' LIMIT 1", p.s.f.app).Scan(&old); e != nil {
		t.Fatal(e)
	}
	if got := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, strings.ReplaceAll(raw, op, old), true, true, "", nil); got.Code != 409 {
		t.Fatal("deletion reused another operation kind", got.Code, got.Body.String())
	}
	data(t, rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, true, true, "", nil), 202)
	for _, changed := range []string{strings.Replace(path, p.s.view, uuid(t, p.s.f.owner), 1), strings.Replace(path, p.flow, uuid(t, p.s.f.owner), 1)} {
		if got := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", changed, raw, true, true, "", nil); got.Code != 409 {
			t.Fatal("same operation changed original scope", got.Code)
		}
	}
	if got := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", rootWorkflowHTTPPath(p.s, p.flow, "enable"), raw, true, true, "", nil); got.Code != 409 {
		t.Fatal("another operation kind reused deletion identity", got.Code)
	}
}
func TestRootDeletionHTTPRejectsStaleScopeAndUnavailableRuntime(t *testing.T) {
	for _, kind := range []string{"stale", "wrong-view", "unavailable"} {
		t.Run(kind, func(t *testing.T) {
			p := deletionFixture(t)
			p.app.DeletionClient = &deletionPeer{}
			raw, op := deletionHTTPBody(t, p)
			path := rootWorkflowHTTPPath(p.s, p.flow, "delete")
			want := 409
			switch kind {
			case "stale":
				raw = `{"operationId":"` + op + `","expectedRevision":2}`
			case "wrong-view":
				path = strings.Replace(path, p.s.view, uuid(t, p.s.f.owner), 1)
				want = 404
			case "unavailable":
				p.app.DeletionClient = nil
				want = 503
			}
			if got := rootWorkflowHTTPRequest(t, p.s, p.handler, "POST", path, raw, true, true, "", nil); got.Code != want {
				t.Fatal("wrong deletion rejection", kind, got.Code, want, got.Body.String())
			}
			var n int
			if e := p.s.f.owner.QueryRow(context.Background(), "SELECT count(*) FROM applications.workflow_deletions WHERE flow_id=$1", p.flow).Scan(&n); e != nil || n != 0 {
				t.Fatal("rejected deletion persisted state", n, e)
			}
		})
	}
}
