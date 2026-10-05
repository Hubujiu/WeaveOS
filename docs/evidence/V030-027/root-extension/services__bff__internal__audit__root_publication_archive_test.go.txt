package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rootPublicationSummary() map[string]any {
	return map[string]any{"appId": "20000000-0000-4000-8000-000000000001", "flowId": "20000000-0000-4000-8000-000000000002", "operationId": "30000000-0000-4000-8000-000000000001", "version": 1, "status": "pending", "action": "workflow.publish.requested"}
}
func TestRootPublicationHotColdConstraintsPreserveSafeShape(t *testing.T) {
	hot, cold := clean(t)
	ctx := context.Background()
	var actor string
	if e := hot.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('workflow-audit-'||gen_random_uuid()) RETURNING id::text").Scan(&actor); e != nil {
		t.Fatal(e)
	}
	for _, target := range []struct {
		pool  *pgxpool.Pool
		table string
	}{{hot, "auth.authentication_events"}, {cold, "archive.authentication_events"}} {
		insert := func(summary map[string]any, reason, object string) error {
			raw, _ := json.Marshal(summary)
			outcome := "success"; if summary["status"] == "blocked" { outcome = "failure" }
			_, e := target.pool.Exec(ctx, "INSERT INTO "+target.table+"(id,occurred_at,event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary) VALUES(gen_random_uuid(),'2026-10-04T00:00:00Z','application_structure_changed',$5,$1,$2,'workflow-shape',$3,'20000000-0000-4000-8000-000000000003',$4::jsonb)", actor, reason, object, raw, outcome)
			return e
		}
		if e := insert(rootPublicationSummary(), "WORKFLOW_PUBLICATION_REQUESTED", "form"); e != nil {
			t.Fatalf("%s rejected valid management metadata: %v", target.table, e)
		}
		for _, pair := range [][2]string{{"confirmed","WORKFLOW_PUBLICATION_CONFIRMED"},{"superseded","WORKFLOW_PUBLICATION_SUPERSEDED"},{"blocked","WORKFLOW_PUBLICATION_BLOCKED"}} {
   m:=rootPublicationSummary(); m["status"]=pair[0];m["action"]="workflow.publish."+pair[0]
   if e:=insert(m,pair[1],"form");e!=nil{t.Fatalf("%s rejected legitimate publication terminal %s: %v",target.table,pair[0],e)}
  }
		for name, mutate := range map[string]func(map[string]any){
			"extra":                  func(m map[string]any) { m["secret"] = "must-not-archive" },
			"missing":                func(m map[string]any) { delete(m, "flowId") },
			"null":                   func(m map[string]any) { m["action"] = nil },
			"string revision":        func(m map[string]any) { m["version"] = "1" },
			"fraction":               func(m map[string]any) { m["version"] = 1.5 },
			"zero":                   func(m map[string]any) { m["version"] = 0 },
			"overflow":               func(m map[string]any) { m["version"] = int64(9007199254740992) },
			"unknown state":          func(m map[string]any) { m["status"] = "approved" },
			"invalid id":             func(m map[string]any) { m["flowId"] = "not-a-uuid" },
			"zero id":                func(m map[string]any) { m["flowId"] = "00000000-0000-0000-0000-000000000000" },
			"action reason mismatch": func(m map[string]any) { m["action"] = "workflow.enable" },
		} {
			m := rootPublicationSummary()
			mutate(m)
			e := insert(m, "WORKFLOW_PUBLICATION_REQUESTED", "form")
			var pgerr *pgconn.PgError
			if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
				t.Fatalf("%s %s wanted23514 got%v", target.table, name, e)
			}
		}
		for _, args := range [][2]string{{"WORKFLOW_UNKNOWN", "form"}, {"WORKFLOW_PUBLICATION_REQUESTED", "application"}} {
			e := insert(rootPublicationSummary(), args[0], args[1])
			var pgerr *pgconn.PgError
			if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
				t.Fatalf("unsafe reason/object accepted %v", e)
			}
		}
		// Old structural summaries remain valid for old actions, but must not be
		// used to omit the identity of a new workflow-management action.
		old := map[string]any{"appId": "20000000-0000-4000-8000-000000000001", "operationId": "30000000-0000-4000-8000-000000000002", "structureVersion": 1, "schemaVersion": 1, "viewVersion": 1, "changeCount": 1}
		if e := insert(old, "DEFINITION_SAVE", "form"); e != nil {
			t.Fatalf("legacy structure audit broken %v", e)
		}
		e := insert(old, "WORKFLOW_PUBLICATION_REQUESTED", "form")
		var pgerr *pgconn.PgError
		if !errors.As(e, &pgerr) || pgerr.Code != "23514" {
			t.Fatalf("workflow action escaped through legacy shape %v", e)
		}
	}
}
func TestRootPublicationArchiveCopiesAllColumnsAndRetainsHotOnConflict(t *testing.T) {
	hot, cold := clean(t)
	ctx := context.Background()
	var actor string
	if e := hot.QueryRow(ctx, "INSERT INTO auth.users(account) VALUES('workflow-copy-'||gen_random_uuid()) RETURNING id::text").Scan(&actor); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(rootPublicationSummary())
	insert := `INSERT INTO auth.authentication_events(id,event_type,outcome,actor_user_id,subject_user_id,account_fingerprint,client_ip,user_agent,session_ref,reason_code,request_id,occurred_at,object_type,object_id,change_summary)
 VALUES($1,'application_structure_changed','success',$2,$2,'test:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','192.0.2.42','synthetic-agent','40000000-0000-4000-8000-000000000001','WORKFLOW_PUBLICATION_REQUESTED','workflow-archive','2026-08-01T00:00:00Z','form','20000000-0000-4000-8000-000000000003',$3::jsonb)`
	if _, e := hot.Exec(ctx, insert, eventID, actor, raw); e != nil {
		t.Fatal(e)
	}
	var before, after string
	if e := hot.QueryRow(ctx, "SELECT to_jsonb(e)::text FROM auth.authentication_events e WHERE id=$1", eventID).Scan(&before); e != nil {
		t.Fatal(e)
	}
	result, e := Maintain(ctx, hot, cold, now)
	if e != nil || result.Archived != 1 {
		t.Fatalf("workflow audit archive %+v %v", result, e)
	}
	if e = cold.QueryRow(ctx, "SELECT to_jsonb(e)::text FROM archive.authentication_events e WHERE id=$1", eventID).Scan(&after); e != nil || before != after {
		t.Fatalf("archive changed event %v", e)
	}
	if _, e = hot.Exec(ctx, insert, eventID, actor, raw); e != nil {
		t.Fatal(e)
	}
	if _, e = cold.Exec(ctx, "UPDATE archive.authentication_events SET change_summary=jsonb_set(change_summary,'{version}','2'::jsonb) WHERE id=$1", eventID); e != nil {
		t.Fatal(e)
	}
	if _, e = Maintain(ctx, hot, cold, now); e == nil {
		t.Fatal("conflicting workflow archive must fail closed")
	}
	if count(t, hot, "auth.authentication_events") != 1 {
		t.Fatal("archive conflict deleted hot workflow audit")
	}
}
