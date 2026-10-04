package apprecordservice

import (
 "errors"
 "strings"
 "testing"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgconn"
)
const rootAcquireSQL="SELECT applications.acquire_record_command_fence($1,$2,$3,$4,$5,$6,$7)"
const rootReleaseSQL="SELECT applications.release_record_command_fence($1,$2,$3,$4,$5,$6)"
func rootFenceAcquire(t *testing.T,f recordFixture,command string)int64{
 t.Helper();var epoch int64
 e:=f.runtime.QueryRow(f.ctx,rootAcquireSQL,f.app,f.table,f.view,f.ownRecord,command,int64(1),int64(1)).Scan(&epoch)
 if e!=nil||epoch<1{t.Fatalf("real controlled acquire must return positive durable epoch: %d %v",epoch,e)};return epoch
}
func rootFenceRelease(t *testing.T,f recordFixture,command string,epoch int64,want bool){
 t.Helper();var released bool;e:=f.runtime.QueryRow(f.ctx,rootReleaseSQL,f.app,f.table,f.ownRecord,command,epoch,int64(1)).Scan(&released)
 if e!=nil||released!=want{t.Fatalf("release=%v want%v err%v",released,want,e)}
}
func rootFenceCount(t *testing.T,f recordFixture,want int){
 t.Helper();var got int;e:=f.owner.QueryRow(f.ctx,"SELECT count(*) FROM applications.record_command_fences WHERE app_id=$1 AND table_id=$2 AND record_id=$3",f.app,f.table,f.ownRecord).Scan(&got)
 if e!=nil||got!=want{t.Fatalf("fence rows%d want%d err%v",got,want,e)}
}
func rootSQLState(t *testing.T,e error,want string){
 t.Helper();var p *pgconn.PgError;if !errors.As(e,&p)||p.Code!=want{t.Fatalf("SQLSTATE want%s got%v",want,e)}
}
func TestRootCommandFenceMetadataExists(t *testing.T){
 f:=newRecordFixture(t);var n int
 e:=f.owner.QueryRow(f.ctx,`SELECT count(*) FROM information_schema.columns WHERE table_schema='applications' AND (table_name,column_name) IN (('record_command_fences','fence_epoch'))`).Scan(&n)
 if e!=nil||n!=1{t.Fatalf("durable epoch metadata count%d err%v",n,e)}
 var hasSequence bool;e=f.owner.QueryRow(f.ctx,"SELECT to_regclass('applications.record_command_fence_epoch_seq') IS NOT NULL").Scan(&hasSequence);if e!=nil||!hasSequence{t.Fatalf("durable epoch sequence absent: %v",e)}
 for _,columns:=range [][]string{{"app_id","table_id","record_id"},{"command_id"}} {var exists bool;e=f.owner.QueryRow(f.ctx,`SELECT EXISTS(SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='applications' AND c.relname='record_command_fences' AND i.indisunique AND (SELECT array_agg(a.attname::text ORDER BY k.ord) FROM unnest(i.indkey::smallint[]) WITH ORDINALITY k(attnum,ord) JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=k.attnum)=$1::text[])`,columns).Scan(&exists);if e!=nil||!exists{t.Fatalf("unique fence key %v missing: %v",columns,e)}}
}
func TestRootCommandFenceAcquireReplayAndRelease(t *testing.T){
 f:=newRecordFixture(t);cmd:=recordOperationID(t,f);epoch:=rootFenceAcquire(t,f,cmd)
 if replay:=rootFenceAcquire(t,f,cmd);replay!=epoch{t.Fatalf("same command changed epoch %d ->%d",epoch,replay)}
 rootFenceCount(t,f,1);rootFenceRelease(t,f,cmd,epoch,true);rootFenceCount(t,f,0)
}
func TestRootCommandFenceOtherCommandCannotReplace(t *testing.T){
 f:=newRecordFixture(t);cmd:=recordOperationID(t,f);epoch:=rootFenceAcquire(t,f,cmd)
 var other int64;e:=f.runtime.QueryRow(f.ctx,rootAcquireSQL,f.app,f.table,f.view,f.ownRecord,recordOperationID(t,f),int64(1),int64(1)).Scan(&other)
 rootSQLState(t,e,"55000");rootFenceCount(t,f,1)
 if got:=rootFenceAcquire(t,f,cmd);got!=epoch{t.Fatal("blocked competitor changed epoch")}
}
func TestRootCommandFenceOldTokenCannotReleaseNewOccupant(t *testing.T){
 f:=newRecordFixture(t);first:=recordOperationID(t,f);old:=rootFenceAcquire(t,f,first);rootFenceRelease(t,f,first,old,true)
 next:=recordOperationID(t,f);epoch:=rootFenceAcquire(t,f,next);if epoch<=old{t.Fatal("epoch reused")}
 rootFenceRelease(t,f,first,old,false);rootFenceRelease(t,f,next,old,false);rootFenceCount(t,f,1)
 rootFenceRelease(t,f,next,epoch,true);rootFenceCount(t,f,0)
}
func TestRootCommandFenceAcquireCallerRollback(t *testing.T){
 f:=newRecordFixture(t);tx,e:=f.runtime.Begin(f.ctx);if e!=nil{t.Fatal(e)};defer tx.Rollback(f.ctx)
 var epoch int64;e=tx.QueryRow(f.ctx,rootAcquireSQL,f.app,f.table,f.view,f.ownRecord,recordOperationID(t,f),int64(1),int64(1)).Scan(&epoch)
 if e!=nil||epoch<1{t.Fatalf("acquire %d %v",epoch,e)};if e=tx.Rollback(f.ctx);e!=nil{t.Fatal(e)};rootFenceCount(t,f,0)
}
func TestRootCommandFenceReleaseCallerRollback(t *testing.T){
 f:=newRecordFixture(t);cmd:=recordOperationID(t,f);epoch:=rootFenceAcquire(t,f,cmd)
 tx,e:=f.runtime.Begin(f.ctx);if e!=nil{t.Fatal(e)};defer tx.Rollback(f.ctx);var released bool
 e=tx.QueryRow(f.ctx,rootReleaseSQL,f.app,f.table,f.ownRecord,cmd,epoch,int64(1)).Scan(&released)
 if e!=nil||!released{t.Fatalf("release %v %v",released,e)};if e=tx.Rollback(f.ctx);e!=nil{t.Fatal(e)};rootFenceCount(t,f,1)
}
func TestRootCommandFenceStaleVersionsAndForeignResources(t *testing.T){
 f:=newRecordFixture(t)
 for _,v:=range []struct{app,table,view,record string;schema,version int64;state string}{
 {f.app,f.table,f.view,f.ownRecord,0,1,"40001"},
 {f.app,f.table,f.view,f.ownRecord,1,2,"40001"},
 {recordOperationID(t,f),f.table,f.view,f.ownRecord,1,1,"23503"},
 {f.app,f.table,recordOperationID(t,f),f.ownRecord,1,1,"23503"},
 {f.app,f.table,f.view,recordOperationID(t,f),1,1,"23503"},
 }{var epoch int64;e:=f.runtime.QueryRow(f.ctx,rootAcquireSQL,v.app,v.table,v.view,v.record,recordOperationID(t,f),v.schema,v.version).Scan(&epoch);rootSQLState(t,e,v.state)}
 rootFenceCount(t,f,0)
}
func TestRootCommandFenceNullAndZeroInputsFailClosed(t *testing.T){
 f:=newRecordFixture(t);base:=[]any{f.app,f.table,f.view,f.ownRecord,recordOperationID(t,f),int64(1),int64(1)}
 for i:=range base{args:=append([]any(nil),base...);args[i]=nil;var epoch int64;e:=f.runtime.QueryRow(f.ctx,rootAcquireSQL,args...).Scan(&epoch);rootSQLState(t,e,"23514")}
 for i:=0;i<5;i++{args:=append([]any(nil),base...);args[i]="00000000-0000-0000-0000-000000000000";var epoch int64;e:=f.runtime.QueryRow(f.ctx,rootAcquireSQL,args...).Scan(&epoch);rootSQLState(t,e,"23514")}
 for _,version:=range []int64{0,-1,9007199254740992}{args:=append([]any(nil),base...);args[6]=version;var epoch int64;e:=f.runtime.QueryRow(f.ctx,rootAcquireSQL,args...).Scan(&epoch);rootSQLState(t,e,"23514")}
 rootFenceCount(t,f,0)
}
func TestRootCommandFenceBlocksRealEditButNotOtherRecord(t *testing.T){
 f:=newRecordFixture(t);cmd:=recordOperationID(t,f);epoch:=rootFenceAcquire(t,f,cmd)
 req:=EditRequest{AppID:f.app,ViewID:f.view,RecordID:f.ownRecord,OperationID:recordOperationID(t,f),ExpectedSchemaVersion:1,ExpectedRecordVersion:1,Changes:map[string]any{f.public:"blocked"}}
 _,e:=f.service.Edit(f.ctx,f.principal,req,applications.Metadata{RequestID:"root-real-command-fenced"})
 var blocked *appstructure.Error;if !errors.As(e,&blocked)||blocked.Code!="APPLICATION_RECORD_FENCED"{t.Fatalf("ordinary record write bypassed durable fence: %v",e)}
 other:=req;other.RecordID=f.otherRecord;other.OperationID=recordOperationID(t,f);other.Changes=map[string]any{f.public:"other is writable"}
 changed,e:=f.service.Edit(f.ctx,f.principal,other,applications.Metadata{RequestID:"root-other-record-unblocked"});if e!=nil||changed.RecordVersion!=2{t.Fatalf("another record blocked %+v %v",changed,e)}
 rootFenceRelease(t,f,cmd,epoch,true)
 req.Changes=map[string]any{f.public:"released"};changed,e=f.service.Edit(f.ctx,f.principal,req,applications.Metadata{RequestID:"root-after-confirmed-release"})
 if e!=nil||changed.RecordVersion!=2{t.Fatalf("released record cannot edit %+v %v",changed,e)}
}
func TestRootCommandFenceSchemaChangeRollsBackDDL(t *testing.T){
 f:=newRecordFixture(t);rootFenceAcquire(t,f,recordOperationID(t,f))
 tx,e:=f.owner.Begin(f.ctx);if e!=nil{t.Fatal(e)};defer tx.Rollback(f.ctx)
 relation:=pgx.Identifier{"appdata","t_"+strings.ReplaceAll(f.table,"-","")}.Sanitize()
 if _,e=tx.Exec(f.ctx,"ALTER TABLE "+relation+" ADD COLUMN root_command_guard_probe integer DEFAULT 7");e!=nil{t.Fatal(e)}
 _,e=tx.Exec(f.ctx,"UPDATE applications.logical_tables SET schema_version=schema_version+1 WHERE app_id=$1 AND id=$2",f.app,f.table)
 rootSQLState(t,e,"55000");var p *pgconn.PgError;_ =errors.As(e,&p)
 if p.ConstraintName!="active_workflow_command_blocks_schema_change"{t.Fatalf("wrong schema blocker %s",p.ConstraintName)}
 _=tx.Rollback(f.ctx)
 var exists bool;e=f.owner.QueryRow(f.ctx,"SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=to_regclass($1) AND attname='root_command_guard_probe' AND NOT attisdropped)",relation).Scan(&exists)
 if e!=nil||exists{t.Fatalf("DDL escaped rollback %v %v",exists,e)}
 var version int64;e=f.owner.QueryRow(f.ctx,"SELECT schema_version FROM applications.logical_tables WHERE id=$1",f.table).Scan(&version)
 if e!=nil||version!=1{t.Fatalf("schema changed %d %v",version,e)}
}
func TestRootCommandFenceRuntimeHasNoDirectWritePrivilege(t *testing.T){
 f:=newRecordFixture(t)
 for _,privilege:=range []string{"INSERT","UPDATE","DELETE"}{var allowed bool;e:=f.runtime.QueryRow(f.ctx,"SELECT has_table_privilege(current_user,'applications.record_command_fences',$1)",privilege).Scan(&allowed)
 if e!=nil||allowed{t.Fatalf("runtime direct %s allowed %v err%v",privilege,allowed,e)}}
 var allowed bool;e:=f.runtime.QueryRow(f.ctx,"SELECT has_function_privilege(current_user,'applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint)','EXECUTE')").Scan(&allowed)
 if e!=nil||!allowed{t.Fatalf("finite acquire capability unavailable %v %v",allowed,e)}
}
func TestRootCommandFenceReleaseRequiresPositiveExactToken(t *testing.T){
 f:=newRecordFixture(t);cmd:=recordOperationID(t,f);epoch:=rootFenceAcquire(t,f,cmd)
 rootFenceRelease(t,f,cmd,epoch+1,false)
 for _,v:=range []any{nil,int64(0),int64(-1)}{var yes bool;e:=f.runtime.QueryRow(f.ctx,rootReleaseSQL,f.app,f.table,f.ownRecord,cmd,v,int64(1)).Scan(&yes);rootSQLState(t,e,"23514")}
 rootFenceCount(t,f,1)
}
func TestRootCommandFenceCounterExhaustionDoesNotWrap(t *testing.T){
 f:=newRecordFixture(t)
 var present bool;if e:=f.owner.QueryRow(f.ctx,"SELECT to_regclass('applications.record_command_fence_epoch_seq') IS NOT NULL").Scan(&present);e!=nil||!present{t.Fatalf("persistent epoch sequence absent: %v",e)}
 var last int64;var called bool;if e:=f.owner.QueryRow(f.ctx,"SELECT last_value,is_called FROM applications.record_command_fence_epoch_seq").Scan(&last,&called);e!=nil{t.Fatal(e)}
 // Restore only this isolated test sequence; no other test runs in parallel.
 defer func(){_,_=f.owner.Exec(f.ctx,"SELECT setval('applications.record_command_fence_epoch_seq',$1,$2)",last,called)}()
 if _,e:=f.owner.Exec(f.ctx,"SELECT setval('applications.record_command_fence_epoch_seq',9007199254740991,true)");e!=nil{t.Fatal(e)}
 var epoch int64;e:=f.runtime.QueryRow(f.ctx,rootAcquireSQL,f.app,f.table,f.view,f.ownRecord,recordOperationID(t,f),int64(1),int64(1)).Scan(&epoch)
 if e==nil{t.Fatal("exhausted fence epoch wrapped")};rootFenceCount(t,f,0)
}

func TestRootCommandFenceDoesNotBlockViewOnlyLayout(t *testing.T){
 f:=newRecordFixture(t);rootFenceAcquire(t,f,recordOperationID(t,f))
 if _,e:=f.owner.Exec(f.ctx,"UPDATE applications.form_views SET layout='[]'::jsonb,view_version=view_version+1 WHERE app_id=$1 AND id=$2",f.app,f.view);e!=nil{t.Fatalf("pure view layout blocked: %v",e)}
 rootFenceCount(t,f,1)
}
