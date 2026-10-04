package appstructure

import (
 "context"
 "encoding/json"
 "testing"
)

func TestRootWorkflowHTTPAuditRetainsExactManagementIdentity(t *testing.T) {
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor)
 data(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body),201)
 var event,reason,actor,objectType,objectID,outcome string;var raw []byte
 e:=s.f.owner.QueryRow(context.Background(),`SELECT event_type,reason_code,actor_user_id::text,object_type,object_id::text,outcome,change_summary
 FROM auth.authentication_events WHERE change_summary->>'operationId'=$1`,body["operationId"]).Scan(&event,&reason,&actor,&objectType,&objectID,&outcome,&raw)
 if e!=nil{t.Fatal(e)}
 var summary map[string]any
 if json.Unmarshal(raw,&summary)!=nil{t.Fatalf("invalid audit JSON %s",raw)}
 if event!="application_structure_changed"||reason!="WORKFLOW_DEFINITION_SAVE"||actor!=s.f.actor||objectType!="form"||objectID!=s.view||outcome!="success"{
  t.Fatalf("audit identity %s %s %s %s %s %s",event,reason,actor,objectType,objectID,outcome)
 }
 if len(summary)!=6||summary["appId"]!=s.f.app||summary["flowId"]!=flow||summary["operationId"]!=body["operationId"]||summary["revision"]!=float64(1)||summary["state"]!="disabled"||summary["action"]!="workflow.definition.save"{
  t.Fatalf("audit lost flow/revision/state/action or included extra data: %s",raw)
 }
}
