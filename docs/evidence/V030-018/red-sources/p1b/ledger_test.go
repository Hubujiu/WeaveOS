package flowcommands

import (
 "context"
 "errors"
 "os"
 "reflect"
 "testing"
 "github.com/jackc/pgx/v5"
)
type rootLedgerDB struct {ctx context.Context; db *pgx.Conn; ledger Ledger}
func rootLedgerFixture(t *testing.T) rootLedgerDB {
 t.Helper();dsn:=os.Getenv("WEAVEOS_TEST_DATABASE_URL");if dsn==""{t.Fatal("isolated PostgreSQL 18 required")}
 ctx:=context.Background();db,e:=pgx.Connect(ctx,dsn);if e!=nil{t.Fatal("connect isolated PostgreSQL failed")};t.Cleanup(func(){_ = db.Close(context.Background())})
 _,e=db.Exec(ctx,`
 CREATE TEMP TABLE workflow_commands(
 command_id uuid PRIMARY KEY,command_json jsonb NOT NULL,
 command_hash bytea NOT NULL CHECK(octet_length(command_hash)=32),
 state text NOT NULL CHECK(state IN ('pending','success','no_effect')),
 receipt_json jsonb,created_at timestamptz NOT NULL DEFAULT now(),
 CHECK((state='pending')=(receipt_json IS NULL)));
 CREATE TEMP TABLE workflow_dispatch(
 command_id uuid PRIMARY KEY REFERENCES workflow_commands(command_id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now());
 CREATE TEMP TABLE root_flow_effects(id int PRIMARY KEY,sequence bigint NOT NULL,fenced boolean NOT NULL);
 INSERT INTO root_flow_effects VALUES(1,11,true);
 CREATE TEMP TABLE root_flow_audit(command_id uuid PRIMARY KEY,outcome text NOT NULL);
 `);if e!=nil{t.Fatal(e)}
 return rootLedgerDB{ctx,db,Ledger{Namespace:"pg_temp"}}
}
func (f rootLedgerDB) begin(t *testing.T)pgx.Tx{t.Helper();tx,e:=f.db.Begin(f.ctx);if e!=nil{t.Fatal(e)};t.Cleanup(func(){_=tx.Rollback(context.Background())});return tx}
func (f rootLedgerDB) counts(t *testing.T,commands,dispatch,audit int,seq int64,fenced bool){
 t.Helper();for _,v:=range []struct{q string;want int}{{"SELECT count(*) FROM pg_temp.workflow_commands",commands},{"SELECT count(*) FROM pg_temp.workflow_dispatch",dispatch},{"SELECT count(*) FROM pg_temp.root_flow_audit",audit}}{var got int;if e:=f.db.QueryRow(f.ctx,v.q).Scan(&got);e!=nil||got!=v.want{t.Fatalf("%s got %d want %d err %v",v.q,got,v.want,e)}}
 var gotSeq int64;var gotFence bool;e:=f.db.QueryRow(f.ctx,"SELECT sequence,fenced FROM pg_temp.root_flow_effects WHERE id=1").Scan(&gotSeq,&gotFence)
 if e!=nil||gotSeq!=seq||gotFence!=fenced{t.Fatalf("effects got %d/%v want %d/%v err %v",gotSeq,gotFence,seq,fenced,e)}
}
func (f rootLedgerDB) accept(t *testing.T,c Command){
 t.Helper();tx:=f.begin(t);v,e:=f.ledger.AcceptInTx(f.ctx,tx,c);if e!=nil||v.State!="pending"||v.Command!=c||v.Receipt!=nil{t.Fatalf("accept %+v %v",v,e)};if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
}
func (f rootLedgerDB) apply(c Command,calls *int,fail bool)func(context.Context,pgx.Tx,ApplyPlan)error {
 return func(ctx context.Context,tx pgx.Tx,p ApplyPlan)error{
 *calls++
 if _,e:=tx.Exec(ctx,"UPDATE pg_temp.root_flow_effects SET sequence=$1,fenced=false WHERE id=1",p.Sequence);e!=nil{return e}
 if _,e:=tx.Exec(ctx,"INSERT INTO pg_temp.root_flow_audit VALUES($1,$2)",c.CommandID,p.Outcome);e!=nil{return e}
 if fail{return errors.New("root fault after real projection and audit writes")}
 return nil
 }
}
func TestRootLedgerAcceptAndExactReplay(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);f.counts(t,1,1,0,11,true)
 tx:=f.begin(t);v,e:=f.ledger.AcceptInTx(f.ctx,tx,c);if e!=nil||v.Command!=c||v.State!="pending"{t.Fatalf("replay %+v %v",v,e)};if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 tx=f.begin(t);read,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);if e!=nil||!reflect.DeepEqual(read,v){t.Fatalf("durable read %+v %v",read,e)};_=tx.Rollback(f.ctx)
 f.counts(t,1,1,0,11,true)
}
func TestRootLedgerConflictDoesNotOverwrite(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);changed:=c;changed.PayloadHash[1]=7
 tx:=f.begin(t);if _,e:=f.ledger.AcceptInTx(f.ctx,tx,changed);!errors.Is(e,ErrConflict){t.Fatalf("same key different payload: %v",e)};_=tx.Rollback(f.ctx)
 tx=f.begin(t);v,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);if e!=nil||v.Command!=c{t.Fatalf("original overwritten %+v %v",v,e)};_=tx.Rollback(f.ctx);f.counts(t,1,1,0,11,true)
}
func TestRootLedgerCallerRollbackRemovesCommandAndDispatch(t *testing.T){
 f:=rootLedgerFixture(t);tx:=f.begin(t)
 if _,e:=f.ledger.AcceptInTx(f.ctx,tx,rootCommand());e!=nil{t.Fatal(e)}
 if e:=tx.Rollback(f.ctx);e!=nil{t.Fatal(e)}
 f.counts(t,0,0,0,11,true)
}
func TestRootLedgerDispatchFailureCannotCommitPartialAcceptance(t *testing.T){
 f:=rootLedgerFixture(t)
 if _,e:=f.db.Exec(f.ctx,"ALTER TABLE pg_temp.workflow_dispatch ADD CONSTRAINT root_deny_dispatch CHECK(false)");e!=nil{t.Fatal(e)}
 tx:=f.begin(t);if _,e:=f.ledger.AcceptInTx(f.ctx,tx,rootCommand());e==nil{t.Fatal("dispatch failure hidden")};_=tx.Rollback(f.ctx)
 f.counts(t,0,0,0,11,true)
}
func TestRootLedgerApplyAndDuplicateAreAtomic(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);r:=rootReceipt(t,c);calls:=0
 tx:=f.begin(t);v,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,false));if e!=nil||v.State!="success"||v.Receipt==nil||*v.Receipt!=r{t.Fatalf("apply %+v %v",v,e)}
 if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 f.counts(t,1,0,1,12,false)
 tx=f.begin(t);again,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,40,f.apply(c,&calls,false));if e!=nil||!reflect.DeepEqual(again,v){t.Fatalf("duplicate %+v %v",again,e)};if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 if calls!=1{t.Fatalf("duplicate callback executed %d times",calls)}
 tx=f.begin(t);replay,e:=f.ledger.AcceptInTx(f.ctx,tx,c);if e!=nil||!reflect.DeepEqual(replay,v){t.Fatalf("terminal accept replay %+v %v",replay,e)};if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 f.counts(t,1,0,1,12,false)
}
func TestRootLedgerOuterRollbackPreservesFenceAndPending(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);r:=rootReceipt(t,c);calls:=0
 tx:=f.begin(t);if _,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,false));e!=nil{t.Fatal(e)};if e:=tx.Rollback(f.ctx);e!=nil{t.Fatal(e)}
 f.counts(t,1,1,0,11,true)
 tx=f.begin(t);v,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);if e!=nil||v.State!="pending"||v.Receipt!=nil{t.Fatalf("terminal escaped rollback %+v %v",v,e)};_=tx.Rollback(f.ctx)
}
func TestRootLedgerProjectionFailureCannotFinalize(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);r:=rootReceipt(t,c);calls:=0
 tx:=f.begin(t);if _,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,true));e==nil{t.Fatal("projection failure hidden")};_=tx.Rollback(f.ctx)
 f.counts(t,1,1,0,11,true)
 tx=f.begin(t);if _,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,false));e!=nil{t.Fatal(e)};if e:=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 f.counts(t,1,0,1,12,false)
}
func TestRootLedgerUnknownWrongAndOutOfOrderNeverProduceEffects(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);base:=rootReceipt(t,c);calls:=0
 for _,edit:=range []func(*Receipt){func(r *Receipt){r.Outcome="timeout"},func(r *Receipt){r.CommandHash[0]^=1},func(r *Receipt){r.Sequence=13}}{
 r:=base;edit(&r);tx:=f.begin(t);if _,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,false));e==nil{t.Fatal("invalid receipt applied")};_=tx.Rollback(f.ctx)
 }
 if calls!=0{t.Fatalf("invalid receipt invoked effects %d",calls)}
 f.counts(t,1,1,0,11,true)
}
func TestRootLedgerDurableNoEffectOnly(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);r:=rootReceipt(t,c);r.Outcome="no_effect";r.Sequence=11;calls:=0
 tx:=f.begin(t);v,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,false));if e!=nil||v.State!="no_effect"{t.Fatalf("no-effect %+v %v",v,e)};if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 f.counts(t,1,0,1,11,false)
}
func TestRootLedgerMissingIsNotNoEffect(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();r:=rootReceipt(t,c);calls:=0;tx:=f.begin(t)
 if _,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);!errors.Is(e,ErrMissing){t.Fatalf("missing get: %v",e)}
 if _,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,false));!errors.Is(e,ErrMissing){t.Fatalf("missing apply: %v",e)}
 _=tx.Rollback(f.ctx);if calls!=0{t.Fatal("missing invoked callback")};f.counts(t,0,0,0,11,true)
}
func TestRootLedgerInvalidEnvelopeHasNoWrites(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();c.RecordVersion=0;tx:=f.begin(t)
 if _,e:=f.ledger.AcceptInTx(f.ctx,tx,c);!errors.Is(e,ErrInvalid){t.Fatalf("invalid accepted: %v",e)};_=tx.Rollback(f.ctx);f.counts(t,0,0,0,11,true)
}
func TestRootLedgerMissingEffectCannotFinalize(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c);tx:=f.begin(t)
 if _,e:=f.ledger.ApplyInTx(f.ctx,tx,c,rootReceipt(t,c),11,nil);!errors.Is(e,ErrInvalid){t.Fatalf("missing callback accepted: %v",e)};_=tx.Rollback(f.ctx);f.counts(t,1,1,0,11,true)
}

func TestRootLedgerReconnectReadsOriginalDurableResult(t *testing.T){
 dsn:=os.Getenv("WEAVEOS_TEST_DATABASE_URL");if dsn==""{t.Fatal("isolated PostgreSQL required")}
 ctx:=context.Background();db,e:=pgx.Connect(ctx,dsn);if e!=nil{t.Fatal("connect failed")}
 defer db.Close(ctx)
 var suffix string;if e=db.QueryRow(ctx,"SELECT replace(gen_random_uuid()::text,'-','')").Scan(&suffix);e!=nil{t.Fatal(e)}
 name:="root_flow_"+suffix;schema:=pgx.Identifier{name}.Sanitize()
 if _,e=db.Exec(ctx,"CREATE SCHEMA "+schema);e!=nil{t.Fatal(e)}
 defer func(){_,_=db.Exec(ctx,"DROP SCHEMA "+schema+" CASCADE")}()
 if _,e=db.Exec(ctx,"CREATE TABLE "+schema+`.workflow_commands(command_id uuid PRIMARY KEY,command_json jsonb NOT NULL,command_hash bytea NOT NULL CHECK(octet_length(command_hash)=32),state text NOT NULL CHECK(state IN ('pending','success','no_effect')),receipt_json jsonb,created_at timestamptz NOT NULL DEFAULT now(),CHECK((state='pending')=(receipt_json IS NULL)));CREATE TABLE `+schema+`.workflow_dispatch(command_id uuid PRIMARY KEY REFERENCES `+schema+`.workflow_commands(command_id),created_at timestamptz NOT NULL DEFAULT now())`);e!=nil{t.Fatal(e)}
 l:=Ledger{Namespace:name};c:=rootCommand();tx,e:=db.Begin(ctx);if e!=nil{t.Fatal(e)}
 if _,e=l.AcceptInTx(ctx,tx,c);e!=nil{_=tx.Rollback(ctx);t.Fatal(e)};if e=tx.Commit(ctx);e!=nil{t.Fatal(e)}
 second,e:=pgx.Connect(ctx,dsn);if e!=nil{t.Fatal("reconnect failed")};defer second.Close(ctx)
 tx,e=second.Begin(ctx);if e!=nil{t.Fatal(e)}
 v,e:=l.GetInTx(ctx,tx,c.CommandID);if e!=nil||v.Command!=c||v.State!="pending"{_=tx.Rollback(ctx);t.Fatalf("new connection lost durable command %+v %v",v,e)}
 if _,e=l.AcceptInTx(ctx,tx,c);e!=nil{_=tx.Rollback(ctx);t.Fatal(e)};if e=tx.Commit(ctx);e!=nil{t.Fatal(e)}
 var n int;if e=second.QueryRow(ctx,"SELECT count(*) FROM "+schema+".workflow_dispatch").Scan(&n);e!=nil||n!=1{t.Fatalf("reconnect re-enqueued command %d %v",n,e)}
}

func TestRootLedgerInvalidPortsFailBeforeWrites(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand()
 if _,e:=f.ledger.AcceptInTx(f.ctx,nil,c);!errors.Is(e,ErrInvalid){t.Fatalf("nil tx: %v",e)}
 tx:=f.begin(t);if _,e:=(Ledger{}).AcceptInTx(f.ctx,tx,c);!errors.Is(e,ErrInvalid){t.Fatalf("empty namespace: %v",e)};_=tx.Rollback(f.ctx)
 f.counts(t,0,0,0,11,true)
}
func TestRootLedgerCorruptStoredCommandFailsClosed(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootCommand();f.accept(t,c)
 if _,e:=f.db.Exec(f.ctx,"UPDATE pg_temp.workflow_commands SET command_json='{}'::jsonb WHERE command_id=$1",c.CommandID);e!=nil{t.Fatal(e)}
 tx:=f.begin(t);if _,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);e==nil{t.Fatal("corrupt stored command returned as valid")};_=tx.Rollback(f.ctx)
 f.counts(t,1,1,0,11,true)
}
