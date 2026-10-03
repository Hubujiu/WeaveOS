package appstructure

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func candidateFixture(t *testing.T) *fixture {
	f := setup(t)
	options, e := redis.ParseURL(os.Getenv("WEAVEOS_TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { client.Close() })
	f.service.Application.CandidateRedis = client
	f.service.Application.CandidateNamespace = "v013-candidate-" + f.actor
	return f
}
func candidateCall(t *testing.T, f *fixture, q string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://weaveos.test/api/v1/applications/"+f.app+"/member-candidates"+q, nil)
	r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
	w := httptest.NewRecorder()
	f.service.ServeHTTP(w, r)
	return w
}
func TestOwnerMemberCandidatesUseOpaqueBoundCursorAndMinimalLiveData(t *testing.T) {
	f := candidateFixture(t)
	prefix := "candidate-" + uuid(t, f.owner) + "-"
	for i := 0; i < 3; i++ {
		if _, e := f.owner.Exec(context.Background(), "INSERT INTO auth.users(account) VALUES($1)", prefix+fmt.Sprint(i)); e != nil {
			t.Fatal(e)
		}
	}
	f.owner.Exec(context.Background(), "INSERT INTO auth.users(account,status) VALUES($1,'disabled')", prefix+"inactive")
	w := candidateCall(t, f, "?q="+url.QueryEscape(" "+prefix+" ")+"&pageSize=2")
	d := data(t, w, 200)
	items := d["items"].([]any)
	if len(items) != 2 || len(items[0].(map[string]any)) != 3 || items[0].(map[string]any)["label"] != prefix+"0" || items[0].(map[string]any)["status"] != "active" {
		t.Fatal("minimal sorted active candidates", d)
	}
	var envelope struct {
		Meta struct {
			Pagination struct {
				NextPageToken *string
				HasMore       bool
			}
		}
	}
	json.Unmarshal(w.Body.Bytes(), &envelope)
	if !envelope.Meta.Pagination.HasMore || envelope.Meta.Pagination.NextPageToken == nil {
		t.Fatal("standard pagination")
	}
	token := *envelope.Meta.Pagination.NextPageToken
	w = candidateCall(t, f, "?q="+url.QueryEscape(prefix)+"&pageSize=2&pageToken="+url.QueryEscape(token))
	d = data(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &envelope)
	if len(d["items"].([]any)) != 1 || envelope.Meta.Pagination.HasMore || envelope.Meta.Pagination.NextPageToken != nil {
		t.Fatal("end cursor contract")
	}
	for _, query := range []string{"?q=other&pageToken=" + token, "?q=" + prefix + "&pageToken=" + token + "x", "?pageSize=51", "?pageSize=0", "?pageSize=2&pageSize=3", "?unknown=x"} {
		w = candidateCall(t, f, query)
		if w.Code != 400 {
			t.Fatalf("malformed/bound cursor must reject %s: %d %s", query, w.Code, w.Body.String())
		}
	}
}
func TestActualGroupMembersRetainIdsAndInactiveDisplay(t *testing.T) {
	f := setup(t)
	var group, member string
	f.owner.QueryRow(context.Background(), "INSERT INTO applications.permission_groups(app_id,name) VALUES($1,'group') RETURNING id::text", f.app).Scan(&group)
	f.owner.QueryRow(context.Background(), "INSERT INTO auth.users(account,status) VALUES('inactive-display-'||gen_random_uuid(),'disabled') RETURNING id::text").Scan(&member)
	if _, e := f.owner.Exec(context.Background(), "INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)", f.app, group, member); e != nil {
		t.Fatal(e)
	}
	v, e := (&applications.Application{Pool: f.runtime}).Configuration(context.Background(), session.Principal{UserID: f.actor, Record: session.Record{AuthVersion: "1"}}, f.app, group, "members")
	if e != nil {
		t.Fatal(e)
	}
	d := v.(map[string]any)
	raw, _ := json.Marshal(d)
	var out struct {
		MemberIDs []string
		Members   []struct {
			ID, Label, Status string
			Selectable        bool
		}
	}
	json.Unmarshal(raw, &out)
	if len(out.MemberIDs) != 1 || out.MemberIDs[0] != member || len(out.Members) != 1 || out.Members[0].ID != member || out.Members[0].Label == "" || out.Members[0].Status != "disabled" || out.Members[0].Selectable {
		t.Fatalf("real inactive group display %s", raw)
	}
}

func TestCandidateCursorBindsActorAppExpiryAndLiveAuthorization(t *testing.T) {
	f := candidateFixture(t)
	app := f.service.Application
	cursor := candidateCursor{Actor: f.actor, App: f.app, Query: "v013-", Account: "v013-before", ID: f.actor}
	token, err := app.storeCursor(context.Background(), cursor)
	if err != nil {
		t.Fatal(err)
	}
	for _, facts := range []candidateCursor{{Actor: uuid(t, f.owner), App: f.app, Query: cursor.Query}, {Actor: f.actor, App: uuid(t, f.owner), Query: cursor.Query}} {
		if _, err := app.loadCursor(context.Background(), token, facts); err == nil {
			t.Fatal("cross-scope real Redis cursor accepted")
		}
	}
	app.Now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	if w := candidateCall(t, f, "?q=v013-&pageToken="+token); w.Code != 400 {
		t.Fatalf("expired cursor %d %s", w.Code, w.Body.String())
	}
	app.Now = nil
	other := setup(t)
	r := httptest.NewRequest("GET", "https://weaveos.test/api/v1/applications/"+other.app+"/member-candidates", nil)
	r.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: f.sid})
	w := httptest.NewRecorder()
	f.service.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("ordinary member must not enumerate account candidates: %d %s", w.Code, w.Body.String())
	}
	if _, err = f.owner.Exec(context.Background(), "UPDATE auth.users SET auth_version=auth_version+1 WHERE id=$1", f.actor); err != nil {
		t.Fatal(err)
	}
	if w := candidateCall(t, f, "?q=v013-&pageToken="+token); w.Code != 401 {
		t.Fatalf("changed auth_version must revoke candidate read: %d %s", w.Code, w.Body.String())
	}
}

func TestCandidateSearchTreatsWildcardCharactersAsLiteralPrefix(t *testing.T) {
	f := candidateFixture(t)
	prefix := "literal-" + uuid(t, f.owner) + "%_"
	for _, account := range []string{prefix + "yes", strings.ReplaceAll(strings.ReplaceAll(prefix, "%", "X"), "_", "Y") + "no"} {
		if _, err := f.owner.Exec(context.Background(), "INSERT INTO auth.users(account) VALUES($1)", account); err != nil {
			t.Fatal(err)
		}
	}
	out := data(t, candidateCall(t, f, "?q="+url.QueryEscape(prefix)), 200)
	items := out["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["label"] != prefix+"yes" {
		t.Fatalf("prefix must not interpret SQL wildcard characters: %+v", out)
	}
}
