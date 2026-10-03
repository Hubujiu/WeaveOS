package audit

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"testing"
)

func TestB5ApplicationArchivePreservesAllFifteenColumnsAndConflictRetainsHot(t *testing.T) {
	hot, cold := clean(t)
	ctx := context.Background()
	var actor string
	if err := hot.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('archive-app') RETURNING id::text").Scan(&actor); err != nil {
		t.Fatal(err)
	}
	summary := `{"appId":"20000000-0000-4000-8000-000000000001","operationId":"30000000-0000-4000-8000-000000000001","beforePolicyRevision":0,"afterPolicyRevision":1,"changeCounts":{"applications":1}}`
	insert := `INSERT INTO auth.authentication_events(id,event_type,outcome,actor_user_id,subject_user_id,account_fingerprint,client_ip,user_agent,session_ref,reason_code,request_id,occurred_at,object_type,object_id,change_summary) VALUES($1,'application_changed','success',$2,$2,'test:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','192.0.2.42','synthetic-agent','40000000-0000-4000-8000-000000000001','APPLICATION_CREATED','b5-archive','2026-08-01T00:00:00Z','application','20000000-0000-4000-8000-000000000001',$3::jsonb)`
	if _, err := hot.Exec(ctx, insert, eventID, actor, summary); err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err := hot.QueryRow(ctx, "SELECT to_jsonb(e)::text FROM auth.authentication_events e WHERE id=$1", eventID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if result, err := Maintain(ctx, hot, cold, now); err != nil || result.Archived != 1 {
		t.Fatalf("application event must archive: %+v %v", result, err)
	}
	if err := cold.QueryRow(ctx, "SELECT to_jsonb(e)::text FROM archive.authentication_events e WHERE id=$1", eventID).Scan(&after); err != nil || before != after {
		t.Fatal("full fifteen-column JSON copy must match", err)
	}
	if _, err := hot.Exec(ctx, insert, eventID, actor, summary); err != nil {
		t.Fatal(err)
	}
	if _, err := Maintain(ctx, hot, cold, now); err != nil {
		t.Fatal("identical copy must deduplicate", err)
	}
	if _, err := hot.Exec(ctx, insert, eventID, actor, summary); err != nil {
		t.Fatal(err)
	}
	if _, err := cold.Exec(ctx, "UPDATE archive.authentication_events SET change_summary=jsonb_set(change_summary,'{operationId}','\"30000000-0000-4000-8000-000000000002\"') WHERE id=$1", eventID); err != nil {
		t.Fatal(err)
	}
	if _, err := Maintain(ctx, hot, cold, now); err == nil {
		t.Fatal("conflicting application summary must fail closed")
	}
	if count(t, hot, "auth.authentication_events") != 1 {
		t.Fatal("cold conflict must retain hot event")
	}
}
func TestB5ColdUnavailableRetainsHotApplicationAudit(t *testing.T) {
	hot, cold := clean(t)
	ctx := context.Background()
	var actor string
	if err := hot.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('archive-unavailable') RETURNING id::text").Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if _, err := hot.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,occurred_at,object_type,object_id,change_summary) VALUES('application_changed','success',$1,'APPLICATION_CREATED','cold-unavailable','2026-08-01T00:00:00Z','application','20000000-0000-4000-8000-000000000001','{"appId":"20000000-0000-4000-8000-000000000001","operationId":"30000000-0000-4000-8000-000000000001","beforePolicyRevision":0,"afterPolicyRevision":1,"changeCounts":{"applications":1}}')`, actor); err != nil {
		t.Fatal(err)
	}
	cold.Close()
	if _, err := Maintain(ctx, hot, cold, now); err == nil {
		t.Fatal("cold unavailability must fail")
	}
	if count(t, hot, "auth.authentication_events") != 1 {
		t.Fatal("cold unavailable must retain hot audit")
	}
}
func TestB5BothAuditConstraintsRejectUnsafeOrIncompleteApplicationSummary(t *testing.T) {
	hot, cold := clean(t)
	ctx := context.Background()
	var actor string
	if err := hot.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('audit-safe') RETURNING id::text").Scan(&actor); err != nil {
		t.Fatal(err)
	}
	valid := `{"appId":"20000000-0000-4000-8000-000000000001","operationId":"30000000-0000-4000-8000-000000000001","beforePolicyRevision":0,"afterPolicyRevision":1,"changeCounts":{"applications":1}}`
	for _, v := range []struct {
		db    *pgxpool.Pool
		table string
	}{{hot, "auth.authentication_events"}, {cold, "archive.authentication_events"}} {
		for _, bad := range []any{nil, `{}`, strings.Replace(valid, `"applications":1`, `"members":["secret-person"]`, 1), strings.Replace(valid, `"changeCounts":`, `"secret":"password","changeCounts":`, 1), strings.Replace(valid, `"afterPolicyRevision":1`, `"afterPolicyRevision":null`, 1)} {
			_, err := v.db.Exec(ctx, "INSERT INTO "+v.table+"(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary) VALUES('application_changed','success',$1,'APPLICATION_CREATED','unsafe','application','20000000-0000-4000-8000-000000000001',$2::jsonb)", actor, bad)
			if err == nil {
				t.Fatal("hot/cold must reject unsafe or incomplete application summary")
			}
		}
	}
}
