package personnel

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestQ36HTTPChangedCriteriaUsesOneRRAndNoHistoricalRows(t *testing.T) {
	f, trace := queryWeb(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("q36-rr-%d-", time.Now().UnixNano())
	ids := []string{}
	for i := 0; i < 2; i++ {
		id := newTarget(t, f.fixture)
		ids = append(ids, id)
		if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", id, prefix+fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	first := memberQueryResponse(t, f.request("POST", "/api/v1/personnel/members/search", queryBody(t, map[string]any{"search": prefix}), true, true))
	// A prior unrelated change forces old-baseline revalidation in the next RR.
	if _, err := f.owner.Exec(ctx, "UPDATE personnel.identities SET description='unrelated RR barrier' WHERE id=$1", f.i2); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{})
	release := make(chan struct{})
	trace.mu.Lock()
	trace.afterRevision = func() { close(reached); <-release }
	trace.mu.Unlock()
	body := map[string]any{"search": prefix, "queryVersion": first.QueryVersion, "filter": map[string]any{"operator": "and", "children": []any{map[string]any{"field": "status", "operator": "eq", "value": "active"}}}}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.request("POST", "/api/v1/personnel/members/search", queryBody(t, body), true, true) }()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("query did not reach real revision barrier")
	}
	if _, err := f.owner.Exec(ctx, "UPDATE auth.users SET account=$2 WHERE id=$1", ids[1], prefix+"changed"); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	result := memberQueryResponse(t, <-done)
	if result.Total != 2 || result.QueryVersion == first.QueryVersion {
		t.Fatal("new criteria baseline missing")
	}
	for _, m := range result.Items {
		if m.ID == ids[1] && m.Account != prefix+"1" {
			t.Fatal("new query/page mixed a later snapshot into old-baseline validation")
		}
	}
	body["queryVersion"] = result.QueryVersion
	queryError(t, f.request("POST", "/api/v1/personnel/members/search", queryBody(t, body), true, true), 409, "COMMON_QUERY_CHANGED")
	delete(body, "queryVersion")
	refreshed := memberQueryResponse(t, f.request("POST", "/api/v1/personnel/members/search", queryBody(t, body), true, true))
	for _, m := range refreshed.Items {
		if m.ID == ids[1] && m.Account != prefix+"changed" {
			t.Fatal("explicit refresh reused historical rows")
		}
	}
}
func TestQ36HTTPEventReferencesAndRealArchiveInvalidation(t *testing.T) {
	f, trace := queryWeb(t)
	ctx := context.Background()
	var actor string
	if err := f.owner.QueryRow(ctx, "SELECT account FROM auth.users WHERE id=$1", f.actor.UserID).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.owner.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary,occurred_at) VALUES('personnel_changed','success',$1,'MEMBER_IDENTITIES_UPDATED','q36-real-archive','member',$1,$2::jsonb,$3)`, f.actor.UserID, `{"before":{"identityIds":[]},"after":{"identityIds":["`+f.i2+`"]}}`, time.Date(2026, 9, 5+i, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	body := map[string]any{"search": actor, "from": "2026-09-01T00:00:00Z", "to": "2026-10-02T00:00:00Z", "pageSize": 1}
	request := func() *httptest.ResponseRecorder {
		return f.request("POST", "/api/v1/personnel/events/search", queryBody(t, body), true, true)
	}
	read := func(w *httptest.ResponseRecorder) EventsQueryPage {
		t.Helper()
		if w.Code != 200 {
			t.Fatalf("event query %d %s", w.Code, w.Body)
		}
		var v struct{ Data EventsQueryPage }
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v.Data
	}
	initial := read(request())
	if initial.Total != 2 || initial.Items[0].Display.Detail != "身份：无 → I2" {
		t.Fatalf("event safe baseline: %+v", initial)
	}
	body["queryVersion"] = initial.QueryVersion
	body["page"] = 2
	trace.take()
	read(request())
	if n := trace.take(); n > 1 {
		t.Fatalf("unchanged event query read full result rows: %d", n)
	}
	if _, err := f.owner.Exec(ctx, "UPDATE personnel.identities SET description='unused event metadata' WHERE id=$1", f.i2); err != nil {
		t.Fatal(err)
	}
	read(request())
	if _, err := f.owner.Exec(ctx, "UPDATE personnel.identities SET name='changed reference' WHERE id=$1", f.i2); err != nil {
		t.Fatal(err)
	}
	queryError(t, request(), 409, "COMMON_QUERY_CHANGED")
	if _, err := f.owner.Exec(ctx, "UPDATE personnel.identities SET name='I2' WHERE id=$1", f.i2); err != nil {
		t.Fatal(err)
	}
	read(request())
	cold, err := pgxpool.New(ctx, os.Getenv("WEAVEOS_TEST_ARCHIVE_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer cold.Close()
	defer cold.Exec(ctx, "DELETE FROM archive.authentication_events WHERE actor_user_id=$1", f.actor.UserID)
	moved, err := audit.Maintain(ctx, f.owner, cold, time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	if err != nil || moved.Archived < 2 {
		t.Fatalf("real background archive: %+v %v", moved, err)
	}
	queryError(t, request(), 409, "COMMON_QUERY_CHANGED")
	delete(body, "queryVersion")
	body["page"] = 1
	if next := read(request()); next.Total != 0 {
		t.Fatal("query silently used archive/historical rows")
	}
}

func TestQ36HTTPExplicitEmptyEventSortIsInvalid(t *testing.T) {
	f, _ := queryWeb(t)
	for _, body := range []string{`{"sortBy":"","sortDirection":""}`, `{"sortBy":"occurredAt","sortDirection":""}`} {
		queryError(t, f.request("POST", "/api/v1/personnel/events/search", body, true, true), 400, "COMMON_INVALID_ARGUMENT")
	}
}
