package appstructure

import(
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "os"
 "net/http"
 "strings"
 "sync"
 "testing"
 "time"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/appworkflows"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/session"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
 pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgconn"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
 "google.golang.org/protobuf/proto"
)
// This peer exercises application orchestration only; V026 separately proves real Java RPC.
type rootPublicationPeer struct{
 mu sync.Mutex
 requests []*pb.DeployRequest
 receipts map[string]*pb.DeploymentReceipt
 after func(*pb.DeployRequest)error
}
func(p *rootPublicationPeer)Deploy(ctx context.Context,q *pb.DeployRequest)(*pb.DeploymentReceipt,error){
 p.mu.Lock();if p.receipts==nil{p.receipts=map[string]*pb.DeploymentReceipt{}}
 h:=sha256.Sum256(q.BpmnXml)
 r:=p.receipts[q.VersionId]
 if r==nil{r=&pb.DeploymentReceipt{AppId:q.AppId,FlowId:q.FlowId,VersionId:q.VersionId,Version:q.Version,BpmnSha256:hex.EncodeToString(h[:]),EngineDeploymentId:"engine-"+q.VersionId,ProcessDefinitionId:"definition-"+q.VersionId};p.receipts[q.VersionId]=r}
 p.requests=append(p.requests,proto.Clone(q).(*pb.DeployRequest));after:=p.after;p.mu.Unlock()
 if after!=nil{if e:=after(q);e!=nil{return nil,e}}
 return proto.Clone(r).(*pb.DeploymentReceipt),nil
}
func(p *rootPublicationPeer)Lookup(ctx context.Context,q *pb.LookupRequest)(*pb.LookupResponse,error){
 p.mu.Lock();defer p.mu.Unlock()
 if r:=p.receipts[q.VersionId];r!=nil{return &pb.LookupResponse{Result:&pb.LookupResponse_Confirmed{Confirmed:proto.Clone(r).(*pb.DeploymentReceipt)}},nil}
 return &pb.LookupResponse{Result:&pb.LookupResponse_NotObserved{NotObserved:&pb.NotObserved{}}},nil
}
type rootPublicationFixture struct{s rootWorkflowManagementFixture;flow string;peer *rootPublicationPeer;app *appworkflows.Application;handler http.Handler}
func rootPublicationSetup(t *testing.T)rootPublicationFixture{
 t.Helper();s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner)
 data(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",rootWorkflowHTTPBody(t,s,s.f.actor)),201)
 peer:=&rootPublicationPeer{receipts:map[string]*pb.DeploymentReceipt{}}
 app:=&appworkflows.Application{Pool:s.f.runtime,Limits:s.f.service.Application.Limits,DeploymentClient:peer}
 return rootPublicationFixture{s,flow,peer,app,&appworkflows.Service{Application:app,Authenticator:s.f.service.Authenticator}}
}
func(p rootPublicationFixture)publish(t *testing.T,id string,revision int64)map[string]any{
 t.Helper();b,_:=json.Marshal(map[string]any{"operationId":id,"expectedRevision":revision,"expectedSchemaVersion":1})
 w:=rootWorkflowHTTPRequest(t,p.s,p.handler,"POST",rootWorkflowHTTPPath(p.s,p.flow,"publish"),string(b),true,true,"",nil)
 got:=data(t,w,202);if got["status"]!="pending"&&got["status"]!="unknown"{t.Fatalf("acceptance falsely terminal %v",got)}
 if w.Header().Get("Location")!=rootWorkflowHTTPPath(p.s,p.flow,"publications/"+id){t.Fatal("missing publication location")}
 return got
}
func(p rootPublicationFixture)result(t *testing.T,id string)map[string]any{
 t.Helper();w:=rootWorkflowHTTPRequest(t,p.s,p.handler,"GET",rootWorkflowHTTPPath(p.s,p.flow,"publications/"+id),"",true,false,"",nil)
 got:=data(t,w,200);if len(got)!=5||got["operationId"]!=id||got["flowId"]!=p.flow{t.Fatalf("nonminimal or wrong publication result %v",got)}
 if w.Header().Get("Cache-Control")!="no-store"{t.Fatal("publication must not be cached")};return got
}
func(p rootPublicationFixture)dispatch(t *testing.T){t.Helper();ok,e:=p.app.DispatchPublication(context.Background());if e!=nil||!ok{t.Fatalf("dispatch %v %v",ok,e)}}
func(p rootPublicationFixture)head(t *testing.T)workflowcatalog.Head{
 t.Helper();var h workflowcatalog.Head;p.s.tx(t,func(tx pgx.Tx)error{var e error;h,e=(workflowcatalog.Catalog{}).GetInTx(context.Background(),tx,p.s.f.app,p.flow);return e});return h
}
func TestRootPublicationSchemaAndRestrictedRoles(t *testing.T){
 s:=rootWorkflowManagementSetup(t)
 for _,name:=range []string{"workflow_publications","workflow_engine_receipts"}{
  var exists bool
  if e:=s.f.owner.QueryRow(context.Background(),"SELECT to_regclass($1) IS NOT NULL","applications."+name).Scan(&exists);e!=nil{t.Fatal(e)}
  if !exists{t.Fatalf("missing required durable relation %s",name)}
  var runtimeRead,backupRead,publicRead bool
  if e:=s.f.owner.QueryRow(context.Background(),"SELECT has_table_privilege('auth_app',$1,'SELECT'),has_table_privilege('auth_backup',$1,'SELECT'),EXISTS(SELECT 1 FROM information_schema.role_table_grants WHERE table_schema='applications' AND table_name=split_part($1,'.',2) AND grantee='PUBLIC' AND privilege_type='SELECT')","applications."+name).Scan(&runtimeRead,&backupRead,&publicRead);e!=nil{t.Fatal(e)}
  if !runtimeRead||!backupRead||publicRead{t.Fatalf("unsafe grants %s %v %v %v",name,runtimeRead,backupRead,publicRead)}
 }
 var exists bool
 if e:=s.f.owner.QueryRow(context.Background(),"SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='applications' AND table_name='workflow_definitions' AND column_name='close_epoch')").Scan(&exists);e!=nil{t.Fatal(e)}
 if !exists{t.Fatal("missing close epoch")}
}
func TestRootPublicationAcceptsDurablyWithoutPublishingOnSave(t *testing.T){
 p:=rootPublicationSetup(t);if p.head(t).CurrentVersion!=0||len(p.peer.requests)!=0{t.Fatal("save published")}
 id:=uuid(t,p.s.f.owner);got:=p.publish(t,id,1)
 if got["version"]!=float64(1)||p.head(t).CurrentVersion!=0||len(p.peer.requests)!=0{t.Fatalf("acceptance published early %v",got)}
 var state string;if e:=p.s.f.owner.QueryRow(context.Background(),"SELECT status FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2",p.s.f.actor,id).Scan(&state);e!=nil{t.Fatal(e)}
 if state!="pending"{t.Fatal(state)}
 p.dispatch(t);if p.result(t,id)["status"]!="confirmed"||p.head(t).CurrentVersion!=1{t.Fatal("confirmed publication missing")}
 var count int;if e:=p.s.f.owner.QueryRow(context.Background(),"SELECT count(*) FROM applications.workflow_engine_receipts WHERE app_id=$1 AND flow_id=$2",p.s.f.app,p.flow).Scan(&count);e!=nil{t.Fatal(e)};if count!=1{t.Fatal("missing full receipt")}
}
func TestRootPublicationReplayAndFingerprint(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1);p.publish(t,id,1)
 b,_:=json.Marshal(map[string]any{"operationId":id,"expectedRevision":2,"expectedSchemaVersion":1})
 w:=rootWorkflowHTTPRequest(t,p.s,p.handler,"POST",rootWorkflowHTTPPath(p.s,p.flow,"publish"),string(b),true,true,"",nil)
 rootWorkflowHTTPError(t,w,409,"APPLICATION_OPERATION_CONFLICT");p.dispatch(t)
 ok,e:=p.app.DispatchPublication(context.Background());if e!=nil||ok||len(p.peer.requests)!=1{t.Fatalf("duplicate work %v %v %d",ok,e,len(p.peer.requests))}
}
func TestRootPublicationRPCDoesNotHoldApplicationOrFlowLocks(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 p.peer.after=func(q *pb.DeployRequest)error{
  ctx,cancel:=context.WithTimeout(context.Background(),time.Second);defer cancel()
  tx,e:=p.s.f.owner.Begin(ctx);if e!=nil{t.Fatal(e)};defer tx.Rollback(context.Background())
  for _,sql:=range []string{"SELECT id FROM applications.apps WHERE id=$1 FOR UPDATE NOWAIT","SELECT id FROM applications.workflow_definitions WHERE app_id=$1 FOR UPDATE NOWAIT"}{
   if _,e=tx.Exec(ctx,sql,p.s.f.app);e!=nil{t.Fatalf("RPC held database gate: %v",e)}
  };return nil
 }
 p.dispatch(t);if p.result(t,id)["status"]!="confirmed"{t.Fatal("not confirmed")}
}
func TestRootPublicationLostResponseRecoversSameIntent(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 p.peer.after=func(*pb.DeployRequest)error{return status.Error(codes.Unavailable,"synthetic response loss")}
 p.dispatch(t);if p.result(t,id)["status"]!="unknown"||p.head(t).CurrentVersion!=0{t.Fatal("lost response misreported")}
 if _,e:=p.s.f.owner.Exec(context.Background(),"UPDATE applications.workflow_publications SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE actor_user_id=$1 AND operation_id=$2",p.s.f.actor,id);e!=nil{t.Fatal(e)}
 p.peer.after=nil;p.dispatch(t)
 if p.result(t,id)["status"]!="confirmed"||p.head(t).CurrentVersion!=1||len(p.peer.requests)!=1{t.Fatal("durable lookup recovery failed")}
}
func TestRootPublicationNewCandidateCannotBeOverwrittenByLateReceipt(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 p.peer.after=func(*pb.DeployRequest)error{b:=rootWorkflowHTTPBody(t,p.s,p.s.f.actor);b["expectedRevision"]=1;b["name"]="New candidate";data(t,rootWorkflowHTTPCall(t,p.s,p.flow,"PUT","definition",b),200);return nil}
 p.dispatch(t);h:=p.head(t)
 if p.result(t,id)["status"]!="superseded"||h.CurrentVersion!=0||h.CandidateVersion!=2{t.Fatalf("late receipt promoted %v",h)}
}
func TestRootPublicationSchemaChangeBlocksActivationPermanently(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 p.peer.after=func(*pb.DeployRequest)error{data(t,p.s.f.call(t,"PUT","/forms/"+p.s.view+"/definition",p.s.body(t,[]any{})),200);return nil}
 p.dispatch(t);got:=p.result(t,id)
 if got["status"]!="blocked"||got["reason"]==nil||p.head(t).CurrentVersion!=0{t.Fatalf("incompatible publication activated %v",got)}
 ok,e:=p.app.DispatchPublication(context.Background());if e!=nil||ok{t.Fatal("blocked request retried automatically")}
}
func TestRootPublicationCloseEpochRejectsLateReceipt(t *testing.T){
 p:=rootPublicationSetup(t);first:=uuid(t,p.s.f.owner);p.publish(t,first,1);p.dispatch(t)
 h:=p.head(t);data(t,rootWorkflowHTTPCall(t,p.s,p.flow,"POST","enable",map[string]any{"operationId":uuid(t,p.s.f.owner),"expectedRevision":h.Revision}),200)
 h=p.head(t);b:=rootWorkflowHTTPBody(t,p.s,p.s.f.actor);b["expectedRevision"]=h.Revision;data(t,rootWorkflowHTTPCall(t,p.s,p.flow,"PUT","definition",b),200)
 h=p.head(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,h.Revision)
 p.peer.after=func(*pb.DeployRequest)error{current:=p.head(t);data(t,rootWorkflowHTTPCall(t,p.s,p.flow,"POST","close",map[string]any{"operationId":uuid(t,p.s.f.owner),"expectedRevision":current.Revision}),200);return nil}
 p.dispatch(t);h=p.head(t)
 if p.result(t,id)["status"]!="blocked"||h.CurrentVersion!=1||h.State!="disabled"{t.Fatalf("closed flow activated %v",h)}
 var epoch int64;if e:=p.s.f.owner.QueryRow(context.Background(),"SELECT close_epoch FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2",p.s.f.app,p.flow).Scan(&epoch);e!=nil{t.Fatal(e)};if epoch!=1{t.Fatalf("close epoch did not record transition %d",epoch)}
}
func TestRootPublicationRequiresSessionCsrfAndExactActor(t *testing.T){
 p:=rootPublicationSetup(t);b,_:=json.Marshal(map[string]any{"operationId":uuid(t,p.s.f.owner),"expectedRevision":1,"expectedSchemaVersion":1});path:=rootWorkflowHTTPPath(p.s,p.flow,"publish")
 rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,p.s,p.handler,"POST",path,string(b),false,true,"",nil),401,"AUTH_UNAUTHENTICATED")
 rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,p.s,p.handler,"POST",path,string(b),true,false,"",nil),403,"COMMON_CSRF_REJECTED")
 other:=uuid(t,p.s.f.owner);rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,p.s,p.handler,"POST",path,string(b),true,true,"",func(r *http.Request){r.Header.Set("X-Expected-Actor-Id",other)}),409,"AUTH_SESSION_CHANGED")
}
func TestRootPublicationStrictPayloadRejectsExtraAndDuplicateKeys(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner)
 valid:=`{"operationId":"`+id+`","expectedRevision":1,"expectedSchemaVersion":1}`
 for _,raw:=range []string{strings.TrimSuffix(valid,"}")+`,"bpmnXml":"untrusted"}`,strings.TrimSuffix(valid,"}")+`,"expectedRevision":1}`,strings.Replace(valid,`"expectedRevision":1`,`"expectedRevision":null`,1)}{
  rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,p.s,p.handler,"POST",rootWorkflowHTTPPath(p.s,p.flow,"publish"),raw,true,true,"",nil),400,"COMMON_INVALID_ARGUMENT")
 }
 if len(p.peer.requests)!=0{t.Fatal("invalid input reached engine")}
}

func TestRootPublicationAuthorityReturnRequiresFreshExplicitIntent(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 p.peer.after=func(*pb.DeployRequest)error{_,e:=p.s.f.owner.Exec(context.Background(),"UPDATE auth.users SET status='disabled',auth_version=auth_version+1 WHERE id=$1",p.s.f.actor);return e}
 p.dispatch(t);var state string
 if e:=p.s.f.owner.QueryRow(context.Background(),"SELECT status FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2",p.s.f.actor,id).Scan(&state);e!=nil{t.Fatal(e)}
 if state!="blocked"||p.head(t).CurrentVersion!=0{t.Fatal("revoked actor publication activated")}
 if _,e:=p.s.f.owner.Exec(context.Background(),"UPDATE auth.users SET status='active' WHERE id=$1",p.s.f.actor);e!=nil{t.Fatal(e)}
 p.peer.after=nil;worked,e:=p.app.DispatchPublication(context.Background());if e!=nil||worked||p.head(t).CurrentVersion!=0{t.Fatal("permission restoration silently activated blocked intent")}
 sid,csrf,e:=p.s.f.session.Create(context.Background(),session.Record{UserID:p.s.f.actor,SessionRef:uuid(t,p.s.f.owner),AuthVersion:"2"});if e!=nil{t.Fatal(e)}
 t.Cleanup(func(){p.s.f.session.Revoke(context.Background(),sid)});p.s.f.sid,p.s.f.csrf=sid,csrf
 fresh:=uuid(t,p.s.f.owner);p.publish(t,fresh,1);p.dispatch(t)
 if p.result(t,fresh)["status"]!="confirmed"||p.result(t,id)["status"]!="blocked"||len(p.peer.requests)!=1{t.Fatal("fresh explicit publication did not recover original immutable deployment")}
}
func TestRootPublicationBadReceiptNeverAdvancesVersion(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 p.peer.after=func(q *pb.DeployRequest)error{p.peer.mu.Lock();defer p.peer.mu.Unlock();p.peer.receipts[q.VersionId].BpmnSha256=strings.Repeat("0",64);return nil}
 p.dispatch(t);if p.result(t,id)["status"]!="unknown"||p.head(t).CurrentVersion!=0{t.Fatal("unbound engine reply trusted")}
}
func TestRootPublicationExpiredLeaseCannotDoubleConfirm(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 entered:=make(chan struct{});release:=make(chan struct{});done:=make(chan error,1)
 p.peer.after=func(*pb.DeployRequest)error{close(entered);<-release;return nil}
 go func(){_,e:=p.app.DispatchPublication(context.Background());done<-e}()
 select{case <-entered:case <-time.After(5*time.Second):t.Fatal("worker never reached RPC")}
 if _,e:=p.s.f.owner.Exec(context.Background(),"UPDATE applications.workflow_publications SET lease_until=clock_timestamp()-interval '1 second',next_attempt_at=clock_timestamp()-interval '1 second' WHERE actor_user_id=$1 AND operation_id=$2",p.s.f.actor,id);e!=nil{close(release);t.Fatal(e)}
 p.dispatch(t);close(release);select{case e:=<-done:if e!=nil{t.Fatal(e)};case <-time.After(5*time.Second):t.Fatal("stale worker did not finish")}
 if p.result(t,id)["status"]!="confirmed"||p.head(t).CurrentVersion!=1||p.head(t).Revision!=2{t.Fatal("stale lease changed confirmed result")}
 var n int;if e:=p.s.f.owner.QueryRow(context.Background(),"SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$1 AND reason_code='WORKFLOW_PUBLICATION_CONFIRMED'",id).Scan(&n);e!=nil{t.Fatal(e)};if n!=1{t.Fatalf("terminal audit duplicated %d",n)}
}
func TestRootPublicationAuditFailureRollsBackActivationAndRecovers(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 name:="root_pub_"+strings.ReplaceAll(id,"-","")
 c:=context.Background()
 sql:="CREATE FUNCTION applications."+name+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.change_summary->>'operationId'='"+id+"' AND NEW.reason_code='WORKFLOW_PUBLICATION_CONFIRMED' THEN RAISE EXCEPTION 'synthetic terminal audit failure'; END IF; RETURN NEW; END $$"
 if _,e:=p.s.f.owner.Exec(c,sql);e!=nil{t.Fatal(e)}
 if _,e:=p.s.f.owner.Exec(c,"CREATE TRIGGER "+name+" BEFORE INSERT ON auth.authentication_events FOR EACH ROW EXECUTE FUNCTION applications."+name+"()");e!=nil{t.Fatal(e)}
 t.Cleanup(func(){p.s.f.owner.Exec(context.Background(),"DROP TRIGGER IF EXISTS "+name+" ON auth.authentication_events; DROP FUNCTION IF EXISTS applications."+name+"()")})
 _,e:=p.app.DispatchPublication(c);if e==nil{t.Fatal("failed terminal audit was reported as completed processing")}
 if p.head(t).CurrentVersion!=0{t.Fatal("activation survived failed audit")}
 var n int;if e=p.s.f.owner.QueryRow(c,"SELECT count(*) FROM applications.workflow_engine_receipts WHERE app_id=$1 AND flow_id=$2",p.s.f.app,p.flow).Scan(&n);e!=nil{t.Fatal(e)};if n!=0{t.Fatal("partial application receipt survived rollback")}
 if _,e=p.s.f.owner.Exec(c,"DROP TRIGGER "+name+" ON auth.authentication_events");e!=nil{t.Fatal(e)}
 if _,e=p.s.f.owner.Exec(c,"UPDATE applications.workflow_publications SET lease_until=CASE WHEN lease_token IS NULL THEN NULL ELSE clock_timestamp()-interval '1 second' END,next_attempt_at=clock_timestamp()-interval '1 second' WHERE actor_user_id=$1 AND operation_id=$2",p.s.f.actor,id);e!=nil{t.Fatal(e)}
 p.dispatch(t);if p.result(t,id)["status"]!="confirmed"||p.head(t).CurrentVersion!=1{t.Fatal("original engine receipt did not recover")}
}

func TestRootPublicationBackgroundLoopRetriesWithoutBrowserResubmission(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 p.peer.after=func(*pb.DeployRequest)error{return status.Error(codes.Unavailable,"synthetic response loss")}
 ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
 done:=make(chan error,1);go func(){done<-p.app.RunPublications(ctx)}()
 ticker:=time.NewTicker(50*time.Millisecond);defer ticker.Stop()
 for{
  select{
  case <-ctx.Done():t.Fatal("background retry never confirmed accepted publication")
  case <-ticker.C:
   if p.result(t,id)["status"]=="confirmed"{cancel();select{case <-done:case <-time.After(2*time.Second):t.Fatal("worker ignored cancellation")};if len(p.peer.requests)!=1{t.Fatal("recovery duplicated engine deployment")};return}
  }
 }
}

func TestRootPublicationAuditUsesExactIdentityAndFiniteStates(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1);p.publish(t,id,1);p.dispatch(t)
 rows,e:=p.s.f.owner.Query(context.Background(),"SELECT event_type,outcome,actor_user_id::text,object_type,object_id::text,reason_code,change_summary FROM auth.authentication_events WHERE change_summary->>'operationId'=$1 ORDER BY occurred_at,id",id);if e!=nil{t.Fatal(e)};defer rows.Close()
 seen:=map[string]bool{};n:=0
 for rows.Next(){var event,outcome,actor,objectType,objectID,reason string;var raw []byte;if e:=rows.Scan(&event,&outcome,&actor,&objectType,&objectID,&reason,&raw);e!=nil{t.Fatal(e)}
  var m map[string]any;if e:=json.Unmarshal(raw,&m);e!=nil{t.Fatal(e)}
  if event!="application_structure_changed"||outcome!="success"||actor!=p.s.f.actor||objectType!="form"||objectID!=p.s.view||len(m)!=6||m["appId"]!=p.s.f.app||m["flowId"]!=p.flow||m["operationId"]!=id||m["version"]!=float64(1){t.Fatalf("unsafe publication audit %s",raw)}
  if reason=="WORKFLOW_PUBLICATION_REQUESTED"{if m["status"]!="pending"||m["action"]!="workflow.publish.requested"{t.Fatal("bad requested audit")}}else if reason=="WORKFLOW_PUBLICATION_CONFIRMED"{if m["status"]!="confirmed"||m["action"]!="workflow.publish.confirmed"{t.Fatal("bad confirmed audit")}}else{t.Fatalf("unexpected reason %s",reason)}
  if seen[reason]{t.Fatal("duplicate audit")};seen[reason]=true;n++
 }
 if e:=rows.Err();e!=nil{t.Fatal(e)};if n!=2{t.Fatalf("want acceptance+terminal audit got %d",n)}
}
func TestRootPublicationMigrationDownRefusesDurableHistory(t *testing.T){
 p:=rootPublicationSetup(t);id:=uuid(t,p.s.f.owner);p.publish(t,id,1)
 source,e:=os.ReadFile("../../../../db/migrations/00017_workflow_publications.sql");if e!=nil{t.Fatal(e)}
 parts:=strings.SplitN(string(source),"-- +goose Down",2);if len(parts)!=2{t.Fatal("missing reversible migration boundary")}
 tx,e:=p.s.f.owner.Begin(context.Background());if e!=nil{t.Fatal(e)}
 _,e=tx.Exec(context.Background(),parts[1]);tx.Rollback(context.Background())
 var pgerr *pgconn.PgError;if !errors.As(e,&pgerr)||pgerr.Code!="55000"{t.Fatalf("Down must refuse history without deleting it: %v",e)}
 if p.result(t,id)["status"]!="pending"{t.Fatal("migration Down changed durable request")}
 p.dispatch(t)
}
