package apprecordservice

import (
 "context"
 "errors"
 "testing"

 "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
 "github.com/jackc/pgx/v5"
)

func rootFormalCommand(t *testing.T, f recordFixture, tx pgx.Tx) flowcommands.Command {
 t.Helper()
 c := flowcommands.Command{ProtocolVersion:1,CommandID:recordOperationID(t,f),AppID:f.app,TableID:f.table,RecordID:f.ownRecord,InstanceID:recordOperationID(t,f),ActorID:f.actor,Action:"start",RecordVersion:1,ExpectedSequence:0}
 c.PayloadHash[0]=1
 if e:=tx.QueryRow(f.ctx,rootAcquireSQL,f.app,f.table,f.view,f.ownRecord,c.CommandID,int64(1),int64(1)).Scan(&c.FenceEpoch);e!=nil {t.Fatal(e)}
 return c
}
func rootFormalRows(t *testing.T,f recordFixture,id string,commands,dispatch,fences int) {
 t.Helper()
 for _,v:=range []struct{table string;want int}{{"workflow_commands",commands},{"workflow_dispatch",dispatch},{"record_command_fences",fences}} {
  var n int
  if e:=f.owner.QueryRow(f.ctx,"SELECT count(*) FROM applications."+v.table+" WHERE command_id=$1",id).Scan(&n);e!=nil||n!=v.want {t.Fatalf("%s got%d want%d err%v",v.table,n,v.want,e)}
 }
}
func rootFormalAccepted(t *testing.T,f recordFixture) flowcommands.Command {
 t.Helper()
 tx,e:=f.runtime.Begin(f.ctx);if e!=nil {t.Fatal(e)};defer tx.Rollback(f.ctx)
 c:=rootFormalCommand(t,f,tx)
 entry,e:=(flowcommands.Ledger{Namespace:"applications"}).AcceptInTx(f.ctx,tx,c)
 if e!=nil||entry.State!="pending" {t.Fatalf("accept %+v %v",entry,e)}
 if e=tx.Commit(f.ctx);e!=nil {t.Fatal(e)}
 return c
}
func TestRootFormalLedgerTablesExist(t *testing.T) {
 f:=newRecordFixture(t)
 var n int
 e:=f.owner.QueryRow(f.ctx,"SELECT count(*) FROM information_schema.tables WHERE table_schema='applications' AND table_name IN ('workflow_commands','workflow_dispatch')").Scan(&n)
 if e!=nil||n!=2 {t.Fatalf("formal durable ledger tables=%d err=%v",n,e)}
}
func TestRootFormalLedgerAcceptAndFenceCommitTogether(t *testing.T) {
 f:=newRecordFixture(t);c:=rootFormalAccepted(t,f);rootFormalRows(t,f,c.CommandID,1,1,1)
 tx,e:=f.runtime.Begin(f.ctx);if e!=nil {t.Fatal(e)};defer tx.Rollback(f.ctx)
 ledger:=flowcommands.Ledger{Namespace:"applications"}
 entry,e:=ledger.GetInTx(f.ctx,tx,c.CommandID)
 if e!=nil||entry.Command!=c||entry.State!="pending" {t.Fatalf("durable read %+v %v",entry,e)}
 if _,e=ledger.AcceptInTx(f.ctx,tx,c);e!=nil {t.Fatal(e)}
 if e=tx.Commit(f.ctx);e!=nil {t.Fatal(e)}
 rootFormalRows(t,f,c.CommandID,1,1,1)
}
func TestRootFormalLedgerAcceptAndFenceRollbackTogether(t *testing.T) {
 f:=newRecordFixture(t);tx,e:=f.runtime.Begin(f.ctx);if e!=nil {t.Fatal(e)};defer tx.Rollback(f.ctx)
 c:=rootFormalCommand(t,f,tx)
 if _,e=(flowcommands.Ledger{Namespace:"applications"}).AcceptInTx(f.ctx,tx,c);e!=nil {t.Fatal(e)}
 if e=tx.Rollback(f.ctx);e!=nil {t.Fatal(e)}
 rootFormalRows(t,f,c.CommandID,0,0,0)
}
func TestRootFormalLedgerApplyReleaseAtomicAndReplay(t *testing.T) {
 f:=newRecordFixture(t);c:=rootFormalAccepted(t,f);ledger:=flowcommands.Ledger{Namespace:"applications"}
 hash,e:=flowcommands.Fingerprint(c);if e!=nil {t.Fatal(e)}
 receipt:=flowcommands.Receipt{CommandID:c.CommandID,CommandHash:hash,Outcome:"success",Sequence:1,ProofID:recordOperationID(t,f)}
 receipt.ResultHash[0]=2
 calls:=0
 apply:=func(fail bool)func(context.Context,pgx.Tx,flowcommands.ApplyPlan)error {
  return func(ctx context.Context,tx pgx.Tx,plan flowcommands.ApplyPlan)error{
   calls++
   var released bool
   if e:=tx.QueryRow(ctx,rootReleaseSQL,f.app,f.table,f.ownRecord,c.CommandID,c.FenceEpoch,c.RecordVersion).Scan(&released);e!=nil {return e}
   if !released {return errors.New("matching accepted command fence was not released")}
   if fail {return errors.New("root injected failure after actual fence release")}
   return nil
  }
 }
 tx,e:=f.runtime.Begin(f.ctx);if e!=nil {t.Fatal(e)}
 _,e=ledger.ApplyInTx(f.ctx,tx,c,receipt,0,apply(true))
 if e==nil {tx.Rollback(f.ctx);t.Fatal("callback failure must not report success")}
 if e=tx.Rollback(f.ctx);e!=nil {t.Fatal(e)}
 rootFormalRows(t,f,c.CommandID,1,1,1)
 tx,e=f.runtime.Begin(f.ctx);if e!=nil {t.Fatal(e)};defer tx.Rollback(f.ctx)
 entry,e:=ledger.GetInTx(f.ctx,tx,c.CommandID)
 if e!=nil||entry.State!="pending"||entry.Receipt!=nil {t.Fatalf("failed apply persisted state %+v %v",entry,e)}
 entry,e=ledger.ApplyInTx(f.ctx,tx,c,receipt,0,apply(false))
 if e!=nil||entry.State!="success" {t.Fatalf("apply %+v %v",entry,e)}
 if e=tx.Commit(f.ctx);e!=nil {t.Fatal(e)}
 rootFormalRows(t,f,c.CommandID,1,0,0)
 tx,e=f.runtime.Begin(f.ctx);if e!=nil {t.Fatal(e)};defer tx.Rollback(f.ctx)
 entry,e=ledger.ApplyInTx(f.ctx,tx,c,receipt,1,func(context.Context,pgx.Tx,flowcommands.ApplyPlan)error{t.Fatal("terminal replay must not repeat effects");return nil})
 if e!=nil||entry.State!="success" {t.Fatalf("replay %+v %v",entry,e)}
 if e=tx.Commit(f.ctx);e!=nil {t.Fatal(e)}
 if calls!=2 {t.Fatalf("expected failed and successful callbacks only, got%d",calls)}
}
func TestRootFormalLedgerRuntimeCannotRewriteIdentityOrDeleteHistory(t *testing.T) {
 f:=newRecordFixture(t);c:=rootFormalAccepted(t,f)
 for _,q:=range []string{
  "UPDATE applications.workflow_commands SET command_json=command_json WHERE command_id=$1",
  "UPDATE applications.workflow_commands SET command_hash=command_hash WHERE command_id=$1",
  "UPDATE applications.workflow_commands SET command_id=command_id WHERE command_id=$1",
  "UPDATE applications.workflow_commands SET created_at=created_at WHERE command_id=$1",
  "DELETE FROM applications.workflow_commands WHERE command_id=$1",
 } {_,e:=f.runtime.Exec(f.ctx,q,c.CommandID);rootSQLState(t,e,"42501")}
 for _,q:=range []string{"TRUNCATE applications.workflow_commands CASCADE","TRUNCATE applications.workflow_dispatch"} {
  _,e:=f.runtime.Exec(f.ctx,q);rootSQLState(t,e,"42501")
 }
 rootFormalRows(t,f,c.CommandID,1,1,1)
}
func TestRootFormalLedgerDatabaseConstraints(t *testing.T) {
 f:=newRecordFixture(t)
 for _,tc:=range []struct{json,state,receipt string;size int}{
  {"{}","pending","null",32},
  {"[]","pending","null",32},
  {"{\"CommandID\":\"00000000-0000-4000-8000-000000000001\"}","pending","null",32},
 } {
  id:=recordOperationID(t,f)
  _,e:=f.owner.Exec(f.ctx,"INSERT INTO applications.workflow_commands(command_id,command_json,command_hash,state,receipt_json) VALUES($1,$2::jsonb,decode(repeat('aa',$3),'hex'),$4,NULL)",id,tc.json,tc.size,tc.state)
  rootSQLState(t,e,"23514")
 }
 id:=recordOperationID(t,f)
 _,e:=f.owner.Exec(f.ctx,"INSERT INTO applications.workflow_dispatch(command_id) VALUES($1)",id)
 rootSQLState(t,e,"23503")
}
func TestRootFormalLedgerCurrentRolesLeastPrivilege(t *testing.T) {
 f:=newRecordFixture(t)
 for _,v:=range []struct{role,table,privilege string;want bool}{
  {"auth_app","workflow_commands","SELECT",true},{"auth_app","workflow_commands","INSERT",true},{"auth_app","workflow_commands","DELETE",false},
  {"auth_app","workflow_dispatch","SELECT",true},{"auth_app","workflow_dispatch","INSERT",true},{"auth_app","workflow_dispatch","DELETE",true},{"auth_app","workflow_dispatch","UPDATE",false},
  {"auth_backup","workflow_commands","SELECT",true},{"auth_backup","workflow_commands","UPDATE",false},{"auth_backup","workflow_dispatch","SELECT",true},
  {"auth_reader","workflow_commands","SELECT",false},{"auth_maintenance","workflow_commands","DELETE",false},
 } {
  var got bool
  if e:=f.owner.QueryRow(f.ctx,"SELECT has_table_privilege($1,$2,$3)",v.role,"applications."+v.table,v.privilege).Scan(&got);e!=nil||got!=v.want {t.Fatalf("%s %s %s=%v want%v err%v",v.role,v.table,v.privilege,got,v.want,e)}
 }
}
