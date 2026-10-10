package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordhttp"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func rootLifecycleBody(op string, version string) string {
	return `{"operationId":"` + op + `","expectedSchemaVersion":1,"expectedRecordVersion":` + version + `}`
}
func TestRootRecordLifecycleHTTPDeleteRestore(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records/" + f.record
	op := f.id(t)
	body := rootLifecycleBody(op, "1")
	w := f.call(t, "POST", path+"/deletion", body, nil)
	got := rootHTTPData(t, w, 200)
	t.Log("V072_SCHEMA RecordLifecycleResult " + string(mustLifecycleJSON(t, got)))
	if len(got) != 5 || string(got["deleted"]) != "true" || string(got["recordVersion"]) != "2" || rootHTTPString(t, got, "operationId") != op {
		t.Fatal("incorrect closed delete receipt", w.Body.String())
	}
	rootHTTPData(t, f.call(t, "POST", path+"/deletion", body, nil), 200)
	rootHTTPError(t, f.call(t, "GET", path, "", nil), 404, "APPLICATION_NOT_FOUND")
	state := rootHTTPData(t, f.call(t, "GET", path+"/lifecycle", "", nil), 200)
	t.Log("V072_SCHEMA RecordLifecycleState " + string(mustLifecycleJSON(t, state)))
	if len(state) != 4 || string(state["deleted"]) != "true" {
		t.Fatal("incorrect bounded state", state)
	}
	restored := rootHTTPData(t, f.call(t, "POST", path+"/restoration", rootLifecycleBody(f.id(t), "2"), nil), 200)
	t.Log("V072_SCHEMA RecordLifecycleResult " + string(mustLifecycleJSON(t, restored)))
	if string(restored["deleted"]) != "false" || string(restored["recordVersion"]) != "3" {
		t.Fatal("incorrect restore", restored)
	}
	rootHTTPData(t, f.call(t, "GET", path, "", nil), 200)
}
func TestRootRecordLifecycleHTTPClosedInputAndIdentity(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records/" + f.record + "/deletion"
	base := rootLifecycleBody(f.id(t), "1")
	for _, tc := range []struct {
		name, body, suffix string
		edit               func(*http.Request)
		status             int
		code               string
	}{
		{"unknown", strings.Replace(base, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":1,"deleted":false`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"duplicate", strings.Replace(base, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":0,"expectedSchemaVersion":1`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"missing", strings.Replace(base, `,"expectedRecordVersion":1`, "", 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"null", strings.Replace(base, `"expectedRecordVersion":1`, `"expectedRecordVersion":null`, 1), "", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"query", base, "?force=true", nil, 400, "COMMON_VALIDATION_FAILED"},
		{"media", base, "", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE"},
		{"actor", base, "", func(r *http.Request) { r.Header.Set("X-Expected-Actor-Id", f.other) }, 409, "AUTH_SESSION_CHANGED"},
		{"session", base, "", func(r *http.Request) { r.Header.Del("Cookie") }, 401, "AUTH_UNAUTHENTICATED"},
		{"csrf", base, "", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, 403, "COMMON_CSRF_REJECTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := f.call(t, "POST", path+tc.suffix, tc.body, tc.edit)
			var env struct{ Code string }
			json.Unmarshal(w.Body.Bytes(), &env)
			if w.Code != tc.status || env.Code != tc.code {
				t.Fatalf("got%d %s want%d %s", w.Code, w.Body.String(), tc.status, tc.code)
			}
		})
	}
}

func TestRootRecordLifecycleHTTPPreservesReadableHistory(t *testing.T) {
	f := rootHTTPResourceSetup(t)
	path := "/api/v1/applications/" + f.app + "/forms/" + f.view + "/records/" + f.record
	before := rootHTTPData(t, f.call(t, "GET", path+"/history", "", nil), 200)
	var items []json.RawMessage
	if e := json.Unmarshal(before["items"], &items); e != nil || len(items) == 0 {
		t.Fatalf("actual create history missing: %v", e)
	}
	rootHTTPData(t, f.call(t, "POST", path+"/deletion", rootLifecycleBody(f.id(t), "1"), nil), 200)
	after := rootHTTPData(t, f.call(t, "GET", path+"/history", "", nil), 200)
	if string(after["items"]) != string(before["items"]) {
		t.Fatalf("deletion changed retained history: %s -> %s", before["items"], after["items"])
	}
}

func TestRootRecordLifecycleHTTPUnknownCommitAndPostcommitRevocation(t *testing.T) {
	for _, mode := range []string{"unknown", "revoke"} {
		t.Run(mode, func(t *testing.T) {
			f := rootHTTPResourceSetup(t)
			op := f.id(t)
			cfg := f.runtime.Config()
			var dropped atomic.Bool
			revoked := make(chan error, 1)
			if mode == "unknown" {
				cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
					c, e := (&net.Dialer{}).DialContext(ctx, network, address)
					if e != nil {
						return nil, e
					}
					return &rootPresetLostCommit{Conn: c, dropped: &dropped}, nil
				}
			} else {
				cfg.ConnConfig.Tracer = &rootPresetCommitHook{fn: func(ctx context.Context) {
					ok, e := f.store.Revoke(ctx, f.sid)
					if !ok && e == nil {
						e = errors.New("expected current session revoke")
					}
					revoked <- e
				}}
			}
			pool, e := pgxpool.NewWithConfig(f.ctx, cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer pool.Close()
			handler := &apprecordhttp.Service{Records: &apprecordservice.Service{Pool: pool, Limits: rootPresetService(f).Limits}, Authenticator: session.Authenticator{Sessions: f.store, DB: f.runtime}}
			server := httptest.NewTLSServer(handler)
			defer server.Close()
			handler.Authenticator.Origin = server.URL
			request, e := http.NewRequestWithContext(f.ctx, "POST", server.URL+"/api/v1/applications/"+f.app+"/forms/"+f.view+"/records/"+f.record+"/deletion", strings.NewReader(rootLifecycleBody(op, "1")))
			if e != nil {
				t.Fatal(e)
			}
			request.Header.Set("Origin", server.URL)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-CSRF-Token", f.csrf)
			request.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
			request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: f.csrf})
			response, e := server.Client().Do(request)
			if e != nil {
				t.Fatal(e)
			}
			defer response.Body.Close()
			raw, e := io.ReadAll(response.Body)
			if e != nil {
				t.Fatal(e)
			}
			var env struct {
				Code string
				Data map[string]json.RawMessage
				Meta map[string]string
			}
			if json.Unmarshal(raw, &env) != nil || env.Meta["requestId"] == "" || response.Header.Get("Cache-Control") != "no-store" || response.TLS == nil {
				t.Fatal("invalid real HTTPS response", string(raw))
			}
			if mode == "unknown" {
				if !dropped.Load() || response.StatusCode != 503 || env.Code != "APPLICATION_OPERATION_UNCONFIRMED" || len(env.Data) != 1 || rootHTTPString(t, env.Data, "operationId") != op || response.Header.Get("Location") != "" || len(response.Cookies()) != 0 {
					t.Fatal("ambiguous commit falsely successful", response.StatusCode, string(raw))
				}
			} else {
				if response.StatusCode != 200 || env.Code != "OK" || len(env.Data) != 5 || response.Header.Get("Location") != "" {
					t.Fatal("confirmed commit disguised as failure", response.StatusCode, string(raw))
				}
				select {
				case e := <-revoked:
					if e != nil {
						t.Fatal(e)
					}
				default:
					t.Fatal("successful response without synchronous postcommit revoke hook")
				}
				if len(response.Cookies()) != 2 {
					t.Fatal("revoked session not cleared")
				}
				for _, cookie := range response.Cookies() {
					if cookie.MaxAge >= 0 {
						t.Fatal("revoked session renewed")
					}
				}
			}
			saved, e := (&applications.Application{Pool: f.runtime}).Operation(f.ctx, rootPresetActor(f, f.actor), op)
			if e != nil || saved.Status != "confirmed" {
				t.Fatal("actual committed result missing", e)
			}
			var n int
			if e = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.record_lifecycle WHERE changed_by=$1 AND app_id=$2", f.actor, f.app).Scan(&n); e != nil || n != 1 {
				t.Fatal("actual commit count", n, e)
			}
		})
	}
}

func mustLifecycleJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
