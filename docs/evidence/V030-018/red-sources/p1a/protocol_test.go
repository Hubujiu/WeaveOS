package flowcommands

import (
 "crypto/sha256"
 "errors"
 "fmt"
 "testing"
)

const otherID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
func rootCommand() Command {
 return Command{ProtocolVersion:1,
 CommandID:"10000000-0000-4000-8000-000000000001",
 AppID:"20000000-0000-4000-8000-000000000002",
 TableID:"30000000-0000-4000-8000-000000000003",
 RecordID:"40000000-0000-4000-8000-000000000004",
 InstanceID:"50000000-0000-4000-8000-000000000005",
 TaskID:"60000000-0000-4000-8000-000000000006",
 ActorID:"70000000-0000-4000-8000-000000000007",
 Action:"agree", RecordVersion:8, FenceEpoch:9, TaskEpoch:10,
 ExpectedSequence:11, PayloadHash:[32]byte{1}}
}
func rootReceipt(t *testing.T,c Command) Receipt {
 t.Helper()
 h,e:=Fingerprint(c);if e!=nil{t.Fatal(e)}
 return Receipt{CommandID:c.CommandID,CommandHash:h,Outcome:"success",Sequence:12,ProofID:otherID,ResultHash:[32]byte{2}}
}
func TestRootFingerprintGoldenAndDeterministic(t *testing.T) {
 c:=rootCommand()
 raw:=`{"ProtocolVersion":1,"CommandID":"10000000-0000-4000-8000-000000000001","AppID":"20000000-0000-4000-8000-000000000002","TableID":"30000000-0000-4000-8000-000000000003","RecordID":"40000000-0000-4000-8000-000000000004","InstanceID":"50000000-0000-4000-8000-000000000005","TaskID":"60000000-0000-4000-8000-000000000006","ActorID":"70000000-0000-4000-8000-000000000007","Action":"agree","RecordVersion":8,"FenceEpoch":9,"TaskEpoch":10,"ExpectedSequence":11,"PayloadHash":[1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}`
 want:=sha256.Sum256([]byte(raw))
 for i:=0;i<3;i++ {got,e:=Fingerprint(c);if e!=nil||got!=want{t.Fatalf("golden binding got %x want %x err %v",got,want,e)}}
}
func TestRootEverySemanticCommandFieldIsBound(t *testing.T) {
 edits:=[]func(*Command){
 func(c *Command){c.CommandID=otherID},func(c *Command){c.AppID=otherID},
 func(c *Command){c.TableID=otherID},func(c *Command){c.RecordID=otherID},
 func(c *Command){c.InstanceID=otherID},func(c *Command){c.TaskID=otherID},
 func(c *Command){c.ActorID=otherID},func(c *Command){c.Action="reject"},
 func(c *Command){c.RecordVersion++},func(c *Command){c.FenceEpoch++},
 func(c *Command){c.TaskEpoch++},func(c *Command){c.ExpectedSequence++},
 func(c *Command){c.PayloadHash[31]=3},
 }
 base:=rootCommand();original,e:=Fingerprint(base);if e!=nil{t.Fatal(e)}
 for i,edit:=range edits {t.Run(fmt.Sprint(i),func(t *testing.T){c:=base;edit(&c);got,e:=Fingerprint(c);if e!=nil||got==original{t.Fatalf("unbound field %d: %v",i,e)}})}
}
func TestRootInvalidCommandsFailClosed(t *testing.T) {
 edits:=[]func(*Command){
 func(c *Command){c.ProtocolVersion=0},func(c *Command){c.ProtocolVersion=2},
 func(c *Command){c.CommandID=""},func(c *Command){c.AppID="not-a-uuid"},
 func(c *Command){c.TableID="00000000-0000-0000-0000-000000000000"},
 func(c *Command){c.RecordID="BBBBBBBB-BBBB-4BBB-8BBB-BBBBBBBBBBBB"},
 func(c *Command){c.InstanceID=""},func(c *Command){c.ActorID=" "+otherID},
 func(c *Command){c.Action="complete"},func(c *Command){c.TaskID=""},
 func(c *Command){c.TaskEpoch=0},func(c *Command){c.RecordVersion=0},
 func(c *Command){c.FenceEpoch=-1},func(c *Command){c.ExpectedSequence=-1},
 func(c *Command){c.PayloadHash=[32]byte{}},
 func(c *Command){c.RecordVersion=9007199254740992},
 func(c *Command){c.FenceEpoch=9007199254740992},
 func(c *Command){c.TaskEpoch=9007199254740992},
 func(c *Command){c.ExpectedSequence=9007199254740992},
 func(c *Command){c.Action="start"},
 }
 for i,edit:=range edits {t.Run(fmt.Sprint(i),func(t *testing.T){c:=rootCommand();edit(&c);h,e:=Fingerprint(c);if !errors.Is(e,ErrInvalid)||h!=([32]byte{}){t.Fatalf("must reject invalid envelope: hash %x err %v",h,e)}})}
}
func TestRootSupportedActionsAndStartIdentity(t *testing.T) {
 for _,action:=range []string{"start","agree","reject","withdraw"}{t.Run(action,func(t *testing.T){
 c:=rootCommand();c.Action=action;if action=="start"{c.TaskID="";c.TaskEpoch=0;c.ExpectedSequence=0}
 if _,e:=Fingerprint(c);e!=nil{t.Fatal(e)}
 })}
}
func TestRootSuccessAndNoEffectPlans(t *testing.T) {
 c:=rootCommand();r:=rootReceipt(t,c)
 p,e:=PlanReceipt(c,r,11,nil)
 if e!=nil||p!=(ApplyPlan{Outcome:"success",Sequence:12}){t.Fatalf("success plan %+v %v",p,e)}
 r.Outcome="no_effect";r.Sequence=11
 p,e=PlanReceipt(c,r,11,nil)
 if e!=nil||p!=(ApplyPlan{Outcome:"no_effect",Sequence:11}){t.Fatalf("durable no-effect plan %+v %v",p,e)}
}
func TestRootResultIdentityAndPayloadMustMatch(t *testing.T) {
 c:=rootCommand();valid:=rootReceipt(t,c)
 edits:=[]func(*Receipt){func(r *Receipt){r.CommandID=otherID},func(r *Receipt){r.CommandHash[0]^=1}}
 for i,edit:=range edits{r:=valid;edit(&r);p,e:=PlanReceipt(c,r,11,nil);if !errors.Is(e,ErrConflict)||p!=(ApplyPlan{}){t.Fatalf("%d accepted unrelated result %+v %v",i,p,e)}}
 changed:=c;changed.RecordVersion++
 if _,e:=PlanReceipt(changed,valid,11,nil);!errors.Is(e,ErrConflict){t.Fatalf("receipt reused for changed record version: %v",e)}
}
func TestRootUnknownIsNeverAResult(t *testing.T) {
 c:=rootCommand();valid:=rootReceipt(t,c)
 for _,outcome:=range []string{"","pending","unknown","timeout","404","failed","cancelled"}{
 r:=valid;r.Outcome=outcome
 p,e:=PlanReceipt(c,r,11,nil);if !errors.Is(e,ErrInvalid)||p!=(ApplyPlan{}){t.Fatalf("unknown promoted to outcome: %q %+v %v",outcome,p,e)}
 }
}
func TestRootProofAndResultHashRequired(t *testing.T) {
 c:=rootCommand();valid:=rootReceipt(t,c)
 edits:=[]func(*Receipt){func(r *Receipt){r.ProofID=""},func(r *Receipt){r.ProofID="00000000-0000-0000-0000-000000000000"},func(r *Receipt){r.ResultHash=[32]byte{}},func(r *Receipt){r.Sequence=-1},func(r *Receipt){r.Sequence=9007199254740992}}
 for i,edit:=range edits{r:=valid;edit(&r);p,e:=PlanReceipt(c,r,11,nil);if !errors.Is(e,ErrInvalid)||p!=(ApplyPlan{}){t.Fatalf("%d accepted invalid proof %+v %v",i,p,e)}}
}
func TestRootSequenceCannotSkipOrRewind(t *testing.T) {
 c:=rootCommand();valid:=rootReceipt(t,c)
 for _,s:=range []int64{0,10,11,13,100}{r:=valid;r.Sequence=s;if _,e:=PlanReceipt(c,r,11,nil);!errors.Is(e,ErrOutOfOrder){t.Fatalf("success sequence %d: %v",s,e)}}
 for _,s:=range []int64{10,12}{r:=valid;r.Outcome="no_effect";r.Sequence=s;if _,e:=PlanReceipt(c,r,11,nil);!errors.Is(e,ErrOutOfOrder){t.Fatalf("no-effect sequence %d: %v",s,e)}}
 for _,s:=range []int64{10,12}{if _,e:=PlanReceipt(c,valid,s,nil);!errors.Is(e,ErrOutOfOrder){t.Fatalf("stale intent against %d: %v",s,e)}}
 for _,s:=range []int64{-1,9007199254740992}{if _,e:=PlanReceipt(c,valid,s,nil);!errors.Is(e,ErrInvalid){t.Fatalf("invalid current sequence %d: %v",s,e)}}
}
func TestRootExactAppliedReplaySurvivesLaterInstanceProgress(t *testing.T) {
 c:=rootCommand();r:=rootReceipt(t,c);before:=r
 p,e:=PlanReceipt(c,r,40,&r)
 if e!=nil||p!=(ApplyPlan{Outcome:"success",Sequence:12,Duplicate:true}){t.Fatalf("replay %+v %v",p,e)}
 if r!=before{t.Fatal("mutated durable input")}
}
func TestRootAppliedResultCannotBeRewritten(t *testing.T) {
 c:=rootCommand();original:=rootReceipt(t,c)
 edits:=[]func(*Receipt){func(r *Receipt){r.ResultHash[0]++},func(r *Receipt){r.ProofID=c.AppID},func(r *Receipt){r.Outcome="no_effect";r.Sequence=11},func(r *Receipt){r.Sequence=13},func(r *Receipt){r.CommandID=otherID}}
 for i,edit:=range edits{r:=original;edit(&r);p,e:=PlanReceipt(c,r,40,&original);if !errors.Is(e,ErrConflict)||p!=(ApplyPlan{}){t.Fatalf("%d rewrote durable outcome %+v %v",i,p,e)}}
}
func TestRootSequenceUpperBound(t *testing.T) {
 c:=rootCommand();c.ExpectedSequence=9007199254740991;r:=rootReceipt(t,c);r.Sequence=c.ExpectedSequence;r.Outcome="no_effect"
 if _,e:=PlanReceipt(c,r,c.ExpectedSequence,nil);e!=nil{t.Fatal(e)}
 r.Outcome="success"
 if _,e:=PlanReceipt(c,r,c.ExpectedSequence,nil);!errors.Is(e,ErrOutOfOrder){t.Fatalf("overflow must not wrap: %v",e)}
}
func TestRootInvalidCommandCannotBeRescuedByReceipt(t *testing.T) {
 c:=rootCommand();r:=rootReceipt(t,c);c.RecordVersion=0
 p,e:=PlanReceipt(c,r,11,nil);if !errors.Is(e,ErrInvalid)||p!=(ApplyPlan{}){t.Fatalf("invalid envelope accepted %+v %v",p,e)}
}
