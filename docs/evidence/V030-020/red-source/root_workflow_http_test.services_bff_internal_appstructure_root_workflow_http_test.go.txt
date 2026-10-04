package appstructure

import (
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/appworkflows"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/session"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
 "github.com/jackc/pgx/v5"
)

type rootWorkflowManagementFixture struct {
 f *fixture
 table,view,field string
}
func rootWorkflowManagementSetup(t *testing.T) rootWorkflowManagementFixture {
 t.Helper();f:=setup(t)
 out:=data(t,f.call(t,"POST","/forms",map[string]any{"operationId":uuid(t,f.owner),"name":"Approval form","source":map[string]any{"kind":"new_table"},"directoryId":nil,"position":0,"expectedStructureVersion":0}),201)
 s:=rootWorkflowManagementFixture{f:f,table:out["table"].(map[string]any)["id"].(string),view:out["form"].(map[string]any)["id"].(string),field:uuid(t,f.owner)}
 body:=s.body(t,[]any{s.fieldValue("text","Original")});body["expectedSchemaVersion"]=0;body["expectedViewVersion"]=0
 data(t,f.call(t,"PUT","/forms/"+s.view+"/definition",body),200)
 return s
}
func(s rootWorkflowManagementFixture) fieldValue(kind,name string) map[string]any {
 return map[string]any{"id":s.field,"name":name,"kind":kind,"required":false,"default":nil,"config":map[string]any{"maxLength":nil},"presentation":map[string]any{"helpText":nil,"displayTimeZone":nil}}
}
func(s rootWorkflowManagementFixture) body(t *testing.T,fields []any) map[string]any {
 return map[string]any{"operationId":uuid(t,s.f.owner),"expectedSchemaVersion":1,"expectedViewVersion":1,"fields":fields,"layout":[]any{},"optionMappings":[]any{},"confirmationToken":nil}
}
func(s rootWorkflowManagementFixture) tx(t *testing.T,fn func(pgx.Tx)error){
 t.Helper();c:=context.Background();tx,e:=s.f.runtime.Begin(c);if e!=nil{t.Fatal(e)};defer tx.Rollback(c)
 if e=fn(tx);e!=nil{t.Fatal(e)};if e=tx.Commit(c);e!=nil{t.Fatal(e)}
}

func rootWorkflowHTTPHandler(s rootWorkflowManagementFixture) http.Handler {
 return &appworkflows.Service{Application:&appworkflows.Application{Pool:s.f.runtime,Limits:s.f.service.Application.Limits},Authenticator:s.f.service.Authenticator}
}
func rootWorkflowHTTPPath(s rootWorkflowManagementFixture,flow,suffix string)string {
 return "/api/v1/applications/"+s.f.app+"/forms/"+s.view+"/workflows/"+flow+"/"+suffix
}
func rootWorkflowHTTPGraph(t *testing.T,s rootWorkflowManagementFixture,assignee string)map[string]any{
 a,b,c:=uuid(t,s.f.owner),uuid(t,s.f.owner),uuid(t,s.f.owner)
 return map[string]any{"version":1,"nodes":[]any{
  map[string]any{"id":a,"kind":"start"},
  map[string]any{"id":b,"kind":"approval","approval":map[string]any{"mode":"all","assigneeIds":[]string{assignee},"editableFieldIds":[]string{s.field}}},
  map[string]any{"id":c,"kind":"end"},
 },"edges":[]any{map[string]any{"from":a,"to":b},map[string]any{"from":b,"to":c}}}
}
func rootWorkflowHTTPBody(t *testing.T,s rootWorkflowManagementFixture,assignee string)map[string]any{
 return map[string]any{"operationId":uuid(t,s.f.owner),"name":"Expense approval","expectedRevision":0,"expectedSchemaVersion":1,"graph":rootWorkflowHTTPGraph(t,s,assignee),"allowWithdraw":true}
}
func rootWorkflowHTTPRequest(t *testing.T,s rootWorkflowManagementFixture,handler http.Handler,method,path,raw string,login,csrf bool,as string,edit func(*http.Request))*httptest.ResponseRecorder{
 t.Helper()
 sid,token:=s.f.sid,s.f.csrf
 if as!=""{
  var e error;sid,token,e=s.f.session.Create(context.Background(),session.Record{UserID:as,SessionRef:uuid(t,s.f.owner),AuthVersion:"1"});if e!=nil{t.Fatal(e)}
  t.Cleanup(func(){s.f.session.Revoke(context.Background(),sid)})
 }
 r:=httptest.NewRequest(method,"https://weaveos.test"+path,strings.NewReader(raw))
 r.Header.Set("Content-Type","application/json");r.Header.Set("Origin","https://weaveos.test")
 if login{r.AddCookie(&http.Cookie{Name:session.SessionCookieName,Value:sid});r.AddCookie(&http.Cookie{Name:session.CSRFCookieName,Value:token})}
 if csrf{r.Header.Set("X-CSRF-Token",token)}
 if edit!=nil{edit(r)}
 w:=httptest.NewRecorder();handler.ServeHTTP(w,r);return w
}
func rootWorkflowHTTPCall(t *testing.T,s rootWorkflowManagementFixture,flow,method,suffix string,body any)*httptest.ResponseRecorder{
 t.Helper();raw,e:=json.Marshal(body);if e!=nil{t.Fatal(e)}
 return rootWorkflowHTTPRequest(t,s,rootWorkflowHTTPHandler(s),method,rootWorkflowHTTPPath(s,flow,suffix),string(raw),true,true,"",nil)
}
func rootWorkflowHTTPError(t *testing.T,w *httptest.ResponseRecorder,status int,code string){
 t.Helper();var v struct{Code string}
 if w.Code!=status||json.Unmarshal(w.Body.Bytes(),&v)!=nil||v.Code!=code{t.Fatalf("want %d %s got %d %s",status,code,w.Code,w.Body.String())}
}
func rootWorkflowHTTPCandidate(t *testing.T,s rootWorkflowManagementFixture,scope string)(string,string){
 t.Helper();c:=context.Background();user,group:=uuid(t,s.f.owner),uuid(t,s.f.owner)
 if _,e:=s.f.owner.Exec(c,"INSERT INTO auth.users(id,account) VALUES($1,$2)",user,"workflow-candidate-"+user);e!=nil{t.Fatal(e)}
 if scope==""{return user,group}
 if _,e:=s.f.owner.Exec(c,"INSERT INTO applications.permission_groups(id,app_id,name) VALUES($1,$2,'workflow readers')",group,s.f.app);e!=nil{t.Fatal(e)}
 if _,e:=s.f.owner.Exec(c,"INSERT INTO applications.group_members(app_id,group_id,user_id) VALUES($1,$2,$3)",s.f.app,group,user);e!=nil{t.Fatal(e)}
 if _,e:=s.f.owner.Exec(c,"INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'menu.enter','all')",s.f.app,group,s.view);e!=nil{t.Fatal(e)}
 var grant string
 if e:=s.f.owner.QueryRow(c,"INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) VALUES($1,$2,'form',$3,'data.read',$4) RETURNING id::text",s.f.app,group,s.view,scope).Scan(&grant);e!=nil{t.Fatal(e)}
 if _,e:=s.f.owner.Exec(c,"INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)",s.f.app,grant,s.field,s.table);e!=nil{t.Fatal(e)}
 return user,group
}
func rootWorkflowHTTPConfirm(t *testing.T,s rootWorkflowManagementFixture,flow string)workflowcatalog.Head{
 t.Helper();var h workflowcatalog.Head
 s.tx(t,func(tx pgx.Tx)error{var e error;h,e=(workflowcatalog.Catalog{}).ConfirmDeploymentInTx(context.Background(),tx,s.f.app,flow,1,"trusted-isolated-receipt");return e})
 return h
}
func TestRootWorkflowHTTPPersistAndReadCanonicalDefinition(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor)
 w:=rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body);got:=data(t,w,201)
 for _,k:=range []string{"operationId","id","revision","currentVersion","candidateVersion","state"}{if _,ok:=got[k];!ok{t.Fatalf("missing mutation field %s: %+v",k,got)}}
 if len(got)!=6||got["id"]!=flow||got["state"]!="disabled"||got["currentVersion"]!=float64(0)||got["candidateVersion"]!=float64(1){t.Fatalf("save implied deployment or returned nonminimal data %+v",got)}
 if w.Header().Get("Cache-Control")!="no-store"{t.Fatal("private configuration must not be cached")}
 read:=data(t,rootWorkflowHTTPCall(t,s,flow,"GET","definition",nil),200)
 if read["name"]!="Expense approval"||read["allowWithdraw"]!=true||read["schemaVersion"]!=float64(1){t.Fatalf("wrong definition %+v",read)}
 graph,ok:=read["graph"].(map[string]any);if !ok||graph["version"]!=float64(1){t.Fatalf("noncanonical graph %+v",read)}
 if _,ok:=graph["Nodes"];ok{t.Fatal("internal graph representation leaked")}
}
func TestRootWorkflowHTTPReplayIsAtomicAndPayloadBound(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor)
 first:=rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body);data(t,first,201)
 replay:=rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body);data(t,replay,201)
 var a,b struct{Data map[string]any};json.Unmarshal(first.Body.Bytes(),&a);json.Unmarshal(replay.Body.Bytes(),&b)
 x,_:=json.Marshal(a.Data);y,_:=json.Marshal(b.Data);if string(x)!=string(y){t.Fatal("replayed result changed")}
 var versions,events int
 if e:=s.f.owner.QueryRow(context.Background(),"SELECT count(*) FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2",s.f.app,flow).Scan(&versions);e!=nil{t.Fatal(e)}
 if e:=s.f.owner.QueryRow(context.Background(),"SELECT count(*) FROM auth.authentication_events WHERE change_summary->>'operationId'=$1",body["operationId"]).Scan(&events);e!=nil{t.Fatal(e)}
 if versions!=1||events!=1{t.Fatalf("replay repeated effects versions=%d audit=%d",versions,events)}
 body["name"]="different"
 rootWorkflowHTTPError(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body),409,"APPLICATION_OPERATION_CONFLICT")
}
func TestRootWorkflowHTTPRequiresRealSessionCSRFAndExpectedActor(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);raw,_:=json.Marshal(rootWorkflowHTTPBody(t,s,s.f.actor));h:=rootWorkflowHTTPHandler(s);path:=rootWorkflowHTTPPath(s,flow,"definition")
 rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,s,h,"PUT",path,string(raw),false,true,"",nil),401,"AUTH_UNAUTHENTICATED")
 rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,s,h,"PUT",path,string(raw),true,false,"",nil),403,"COMMON_CSRF_REJECTED")
 other:=uuid(t,s.f.owner)
 rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,s,h,"PUT",path,string(raw),true,true,"",func(r *http.Request){r.Header.Set("X-Expected-Actor-Id",other)}),409,"AUTH_SESSION_CHANGED")
}
func TestRootWorkflowHTTPOrdinaryDataAccessDoesNotGrantManagement(t *testing.T){
 s:=rootWorkflowManagementSetup(t);other,_:=rootWorkflowHTTPCandidate(t,s,"all");flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor);raw,_:=json.Marshal(body)
 rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,s,rootWorkflowHTTPHandler(s),"PUT",rootWorkflowHTTPPath(s,flow,"definition"),string(raw),true,true,other,nil),403,"APPLICATION_FORBIDDEN")
 var n int;if e:=s.f.owner.QueryRow(context.Background(),"SELECT count(*) FROM applications.workflow_definitions WHERE id=$1",flow).Scan(&n);e!=nil||n!=0{t.Fatalf("unauthorized definition created %d %v",n,e)}
}
func TestRootWorkflowHTTPRejectsCandidateWithoutExistingPermission(t *testing.T){
 s:=rootWorkflowManagementSetup(t);candidate,_:=rootWorkflowHTTPCandidate(t,s,"");flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,candidate)
 rootWorkflowHTTPError(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body),403,"WORKFLOW_APPROVER_FORBIDDEN")
 var n int;if e:=s.f.owner.QueryRow(context.Background(),"SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2",s.f.actor,body["operationId"]).Scan(&n);e!=nil||n!=0{t.Fatalf("rejected candidate left operation %d %v",n,e)}
}
func TestRootWorkflowHTTPOwnScopeCandidateAndRevocationRechecked(t *testing.T){
 s:=rootWorkflowManagementSetup(t);candidate,group:=rootWorkflowHTTPCandidate(t,s,"own");flow:=uuid(t,s.f.owner)
 data(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",rootWorkflowHTTPBody(t,s,candidate)),201)
 h:=rootWorkflowHTTPConfirm(t,s,flow)
 if _,e:=s.f.owner.Exec(context.Background(),"UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1 AND id=$2",s.f.app,group);e!=nil{t.Fatal(e)}
 rootWorkflowHTTPError(t,rootWorkflowHTTPCall(t,s,flow,"POST","enable",map[string]any{"operationId":uuid(t,s.f.owner),"expectedRevision":h.Revision}),403,"WORKFLOW_APPROVER_FORBIDDEN")
 s.tx(t,func(tx pgx.Tx)error{got,e:=(workflowcatalog.Catalog{}).GetInTx(context.Background(),tx,s.f.app,flow);if e==nil&&got.State!="disabled"{t.Fatal("revoked candidate flow enabled")};return e})
}
func TestRootWorkflowHTTPEnableRequiresConfirmedDeployment(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner)
 data(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",rootWorkflowHTTPBody(t,s,s.f.actor)),201)
 rootWorkflowHTTPError(t,rootWorkflowHTTPCall(t,s,flow,"POST","enable",map[string]any{"operationId":uuid(t,s.f.owner),"expectedRevision":1}),409,"WORKFLOW_NOT_READY")
 h:=rootWorkflowHTTPConfirm(t,s,flow)
 out:=data(t,rootWorkflowHTTPCall(t,s,flow,"POST","enable",map[string]any{"operationId":uuid(t,s.f.owner),"expectedRevision":h.Revision}),200)
 if out["state"]!="enabled"{t.Fatalf("did not enable confirmed definition %+v",out)}
 closed:=data(t,rootWorkflowHTTPCall(t,s,flow,"POST","close",map[string]any{"operationId":uuid(t,s.f.owner),"expectedRevision":out["revision"]}),200)
 if closed["state"]!="disabled"{t.Fatalf("empty flow not closed %+v",closed)}
}
func TestRootWorkflowHTTPCloseReportsClosingWithAcceptedInstance(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);data(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",rootWorkflowHTTPBody(t,s,s.f.actor)),201)
 h:=rootWorkflowHTTPConfirm(t,s,flow);out:=data(t,rootWorkflowHTTPCall(t,s,flow,"POST","enable",map[string]any{"operationId":uuid(t,s.f.owner),"expectedRevision":h.Revision}),200)
 record:=uuid(t,s.f.owner);c:=context.Background();physical:=pgx.Identifier{"appdata","t_"+strings.ReplaceAll(s.table,"-","")}.Sanitize()
 if _,e:=s.f.owner.Exec(c,"INSERT INTO "+physical+"(id,created_by) VALUES($1,$2)",record,s.f.actor);e!=nil{t.Fatal(e)}
 s.tx(t,func(tx pgx.Tx)error{_,e:=(workflowcatalog.Catalog{}).ReserveInTx(c,tx,workflowcatalog.ReserveInput{AppID:s.f.app,FlowID:flow,InstanceID:uuid(t,s.f.owner),RecordID:record,ActorID:s.f.actor,ExpectedRevision:int64(out["revision"].(float64)),ExpectedSchemaVersion:1,ExpectedRecordVersion:1});return e})
 closed:=data(t,rootWorkflowHTTPCall(t,s,flow,"POST","close",map[string]any{"operationId":uuid(t,s.f.owner),"expectedRevision":out["revision"]}),200)
 if closed["state"]!="closing"{t.Fatalf("accepted start omitted from close %+v",closed)}
}
func TestRootWorkflowHTTPStaleRevisionSchemaAndInvalidGraphAreAtomic(t *testing.T){
 for _,kind:=range []string{"schema","revision","graph"}{t.Run(kind,func(t *testing.T){
  s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor)
  code,status:="WORKFLOW_CONFLICT",409
  switch kind{case "schema":body["expectedSchemaVersion"]=2;case "revision":body["expectedRevision"]=2;case "graph":body["graph"].(map[string]any)["version"]=0;code,status="COMMON_INVALID_ARGUMENT",400}
  rootWorkflowHTTPError(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body),status,code)
  var n int;if e:=s.f.owner.QueryRow(context.Background(),"SELECT count(*) FROM applications.operations WHERE actor_user_id=$1 AND operation_id=$2",s.f.actor,body["operationId"]).Scan(&n);e!=nil||n!=0{t.Fatalf("invalid save retained operation %d %v",n,e)}
 })}
}
func TestRootWorkflowHTTPClosedJSONAndNoClientDeploymentProof(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor);base,_:=json.Marshal(body);h:=rootWorkflowHTTPHandler(s);path:=rootWorkflowHTTPPath(s,flow,"definition")
 for _,raw:=range []string{
  string(base)+"{}",
  strings.TrimSuffix(string(base),"}")+",\"actorId\":\""+s.f.actor+"\"}",
  strings.TrimSuffix(string(base),"}")+",\"deploymentId\":\"fake\"}",
  strings.TrimSuffix(string(base),"}")+",\"operationId\":\""+uuid(t,s.f.owner)+"\"}",
 }{
  rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,s,h,"PUT",path,raw,true,true,"",nil),400,"COMMON_INVALID_ARGUMENT")
 }
 rootWorkflowHTTPError(t,rootWorkflowHTTPRequest(t,s,h,"PUT",path+"?force=true",string(base),true,true,"",nil),400,"COMMON_INVALID_ARGUMENT")
 w:=rootWorkflowHTTPRequest(t,s,h,"PUT",path,string(base),true,true,"",func(r *http.Request){r.Header.Set("Content-Type","text/plain")})
 if w.Code!=415{t.Fatalf("content type %d %s",w.Code,w.Body.String())}
}
func TestRootWorkflowHTTPRevokedAccountCannotReplayOrRead(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor);data(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body),201)
 if _,e:=s.f.owner.Exec(context.Background(),"UPDATE auth.users SET status='disabled' WHERE id=$1",s.f.actor);e!=nil{t.Fatal(e)}
 rootWorkflowHTTPError(t,rootWorkflowHTTPCall(t,s,flow,"PUT","definition",body),401,"AUTH_UNAUTHENTICATED")
 rootWorkflowHTTPError(t,rootWorkflowHTTPCall(t,s,flow,"GET","definition",nil),401,"AUTH_UNAUTHENTICATED")
}
func TestRootWorkflowHTTPHostRoutesBeforeGenericForms(t *testing.T){
 s:=rootWorkflowManagementSetup(t);flow:=uuid(t,s.f.owner);body:=rootWorkflowHTTPBody(t,s,s.f.actor);raw,_:=json.Marshal(body)
 host:=&applications.Service{Application:&applications.Application{Pool:s.f.runtime},Authenticator:s.f.service.Authenticator,Definitions:s.f.service,Workflows:rootWorkflowHTTPHandler(s)}
 w:=rootWorkflowHTTPRequest(t,s,host,"PUT",rootWorkflowHTTPPath(s,flow,"definition"),string(raw),true,true,"",nil)
 data(t,w,201)
}
