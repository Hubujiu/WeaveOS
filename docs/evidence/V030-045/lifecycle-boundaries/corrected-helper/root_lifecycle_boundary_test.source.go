package apprecordservice

import (
 "context"
 "errors"
 "sync"
 "sync/atomic"
 "testing"
 "time"

 "github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/session"
 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
)

func TestRootLifecycleReplayExpiredBasisOfflineSingleConnection(t *testing.T) {
 f:=rootTaskSetup(t,false);q:=rootLifecycleActionReq(t,f,"withdraw");first:=rootLifecycleAccept(t,f,q)
 prefix,e:=f.service.WorkflowLifecycleBases.Prefix(f.principal.SessionRef);if e!=nil{t.Fatal(e)}
 if e=rootTaskRedis(t).Del(f.ctx,prefix+q.BasisToken).Err();e!=nil{t.Fatal(e)}
 cfg:=f.runtime.Config();cfg.MaxConns=1;cfg.MinConns=0
 pool,e:=pgxpool.NewWithConfig(f.ctx,cfg);if e!=nil{t.Fatal(e)};defer pool.Close()
 service:=*f.service;service.Pool=pool;service.RuntimeReady=func(context.Context)error{return errors.New("isolated offline")}
 ctx,cancel:=context.WithTimeout(f.ctx,3*time.Second);defer cancel()
 again,e:=service.AcceptWorkflowLifecycle(ctx,f.principal,q,applications.Metadata{RequestID:"replay-expired-offline"})
 if e!=nil||again!=first{t.Fatalf("persisted receipt must survive expired preview and engine outage without nested pool: %+v %v",again,e)}
 rootLifecycleAtomicCount(t,f,1,1,1)
 if _,e=f.owner.Exec(f.ctx,"UPDATE auth.users SET status='disabled' WHERE id=$1",f.actor);e!=nil{t.Fatal(e)}
 if _,e=service.AcceptWorkflowLifecycle(ctx,f.principal,q,applications.Metadata{RequestID:"inactive-replay"});!errors.Is(e,session.ErrUnauthorized){t.Fatalf("inactive actor recovered receipt: %v",e)}
}
func TestRootLifecycleConcurrentExactRequestsKeepSingleCommand(t *testing.T) {
 f:=rootTaskSetup(t,false);q:=rootLifecycleActionReq(t,f,"return")
 type outcome struct{r WorkflowOperationResult;e error};results:=make(chan outcome,8);start:=make(chan struct{});var wg sync.WaitGroup
 for i:=0;i<8;i++{wg.Add(1);go func(){defer wg.Done();<-start;r,e:=f.service.AcceptWorkflowLifecycle(f.ctx,f.principal,q,applications.Metadata{RequestID:"exact-concurrent"});results<-outcome{r,e}}()}
 close(start);wg.Wait();close(results);var first WorkflowOperationResult
 for v:=range results{if v.e!=nil{t.Fatal(v.e)};if first.CommandID==""{first=v.r};if v.r!=first||v.r.Status!="pending"{t.Fatal("duplicate accepted command identity")}}
 rootLifecycleAtomicCount(t,f,1,1,1)
}
func TestRootLifecycleDistinctConcurrentActionsRespectRecordFence(t *testing.T) {
 f:=rootTaskSetup(t,false);a:=rootLifecycleActionReq(t,f,"withdraw");b:=a;b.OperationID=recordOperationID(t,f.recordFixture);b.Action="return";b.TargetNodeID=f.task.NodeID
 start:=make(chan struct{});results:=make(chan error,2)
 for _,q:=range []WorkflowLifecycleActionRequest{a,b}{go func(q WorkflowLifecycleActionRequest){<-start;_,e:=f.service.AcceptWorkflowLifecycle(f.ctx,f.principal,q,applications.Metadata{RequestID:"distinct-concurrent"});results<-e}(q)}
 close(start);passed,blocked:=0,0
 for i:=0;i<2;i++{e:=<-results;if e==nil{passed++;continue};var d *appstructure.Error;if errors.As(e,&d)&&d.Code=="APPLICATION_RECORD_FENCED"{blocked++}else{t.Fatalf("unexpected competing-action error: %v",e)}}
 if passed!=1||blocked!=1{t.Fatalf("accepted %d fenced %d",passed,blocked)};rootLifecycleAtomicCount(t,f,1,1,1)
}
func TestRootLifecycleCommitUncertaintyPreservesAtomicity(t *testing.T) {
 for _,mode:=range []string{"rollback","lost-reply"}{t.Run(mode,func(t *testing.T){
  f:=rootTaskSetup(t,false);q:=rootLifecycleActionReq(t,f,"withdraw");service:=*f.service
  var dropped atomic.Bool;abort:=&abortRecordBeforeCommit{}
  if mode=="rollback"{service.Pool=recordCommitFaultPool(t,nil,abort)}else{service.Pool=recordCommitFaultPool(t,&dropped,nil)}
  r,e:=service.AcceptWorkflowLifecycle(f.ctx,f.principal,q,applications.Metadata{RequestID:mode})
  if r.CommandID!=""{t.Fatal("uncertain commit reported success")}
  if mode=="rollback"{if !abort.injected.Load()||!errors.Is(e,pgx.ErrTxCommitRollback){t.Fatalf("real rollback not reached: %v",e)};rootLifecycleAtomicCount(t,f,0,0,0)}else{if !dropped.Load()||!errors.Is(e,applications.ErrUnconfirmed){t.Fatalf("lost commit reply not classified: %v",e)};rootLifecycleAtomicCount(t,f,1,1,1)}
  if rootLifecycleAccept(t,f,q).Status!="pending"{t.Fatal("same request did not recover")};rootLifecycleAtomicCount(t,f,1,1,1)
 })}
}

func rootLifecycleAtomicCount(t *testing.T,f rootTaskFixture,pending,evidence,operations int){
 t.Helper()
 for _,v:=range []struct{sql string;want int}{
  {"SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1",pending},
  {"SELECT count(*) FROM applications.workflow_evidence_documents WHERE app_id=$1",evidence},
  {"SELECT count(*) FROM applications.operations WHERE app_id=$1 AND operation_kind IN('workflow.instance.withdraw','workflow.task.return')",operations},
  {"SELECT count(*) FROM applications.workflow_commands WHERE command_json->>'AppID'=$1",1+operations},
  {"SELECT count(*) FROM applications.workflow_dispatch d JOIN applications.workflow_commands c USING(command_id) WHERE c.command_json->>'AppID'=$1",pending},
 }{var n int;if e:=f.owner.QueryRow(f.ctx,v.sql,f.app).Scan(&n);e!=nil||n!=v.want{t.Fatalf("atomic lifecycle count got%d want%d %s: %v",n,v.want,v.sql,e)}}
}
