package flowcommands

import (
 "errors"
 "testing"
)

// Real PostgreSQL and the same Ledger port; these are not engine execution proofs.
func TestRootV2LedgerRoundTripAndExactTerminalReplay(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootV2Command();f.accept(t,c)
 tx:=f.begin(t);got,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);if e!=nil||got.Command!=c||got.State!="pending"{t.Fatalf("durable v2 identity %+v %v",got,e)};_ =tx.Rollback(f.ctx)
 calls:=0;r:=rootReceipt(t,c);tx=f.begin(t);out,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,false));if e!=nil||out.State!="success"{t.Fatalf("apply %+v %v",out,e)};if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 tx=f.begin(t);again,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,50,f.apply(c,&calls,false));if e!=nil||again.Receipt==nil||*again.Receipt!=r{t.Fatalf("replay %+v %v",again,e)};if e=tx.Commit(f.ctx);e!=nil{t.Fatal(e)}
 if calls!=1{t.Fatal("duplicate applied effects")};f.counts(t,1,0,1,12,false)
}
func TestRootV2LedgerBindingConflictKeepsOriginal(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootV2Command();f.accept(t,c)
 for _,edit:=range []func(*Command){func(c *Command){c.FlowID=otherID},func(c *Command){c.SchemaVersion++},func(c *Command){c.DefinitionVersion++},func(c *Command){c.Action="return";c.TargetNodeID=otherID}}{
  bad:=c;edit(&bad);tx:=f.begin(t);if _,e:=f.ledger.AcceptInTx(f.ctx,tx,bad);!errors.Is(e,ErrConflict){t.Fatalf("identity collision accepted %v",e)};_ =tx.Rollback(f.ctx)
 }
 tx:=f.begin(t);got,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);if e!=nil||got.Command!=c{t.Fatal("original v2 command overwritten")};_ =tx.Rollback(f.ctx);f.counts(t,1,1,0,11,true)
}
func TestRootV2LedgerProjectionFailureRollsBack(t *testing.T){
 f:=rootLedgerFixture(t);c:=rootV2Command();f.accept(t,c);r:=rootReceipt(t,c);calls:=0
 tx:=f.begin(t);if _,e:=f.ledger.ApplyInTx(f.ctx,tx,c,r,11,f.apply(c,&calls,true));e==nil{t.Fatal("projection fault hidden")};if e:=tx.Rollback(f.ctx);e!=nil{t.Fatal(e)};f.counts(t,1,1,0,11,true)
 tx=f.begin(t);got,e:=f.ledger.GetInTx(f.ctx,tx,c.CommandID);if e!=nil||got.State!="pending"||got.Receipt!=nil{t.Fatal("failed projection finalized command")};_ =tx.Rollback(f.ctx)
}
