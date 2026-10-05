//go:build workflowrpc_integration

package workflowrpc

import (
 "context"
 "crypto/sha256"
 "net"
 "os"
 "reflect"
 "testing"
 "time"
 fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
 fg "github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
 pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
 "google.golang.org/grpc"
 "google.golang.org/grpc/credentials/insecure"
)

func rootRealExecution(t *testing.T)(*ExecutionClient,fc.Command,fc.ExecutionPayload){
 t.Helper();target:=os.Getenv("WEAVEOS_RPC_TEST_TARGET");host,_,e:=net.SplitHostPort(target)
 if e!=nil||host!="b3-workflow"{t.Fatal("only the dedicated unexposed b3-workflow fixture is allowed")}
 conn,e:=grpc.NewClient(target,grpc.WithTransportCredentials(insecure.NewCredentials()),grpc.WithDisableRetry());if e!=nil{t.Fatal(e)};t.Cleanup(func(){conn.Close()})
 deploy,e:=NewClient(conn,15*time.Second);if e!=nil{t.Fatal(e)}
 c,p:=rootExecutionInput()
 graph:=fg.Graph{Version:1,Nodes:[]fg.Node{
  {ID:rootExecutionID(1),Kind:"start"},
  {ID:rootExecutionID(2),Kind:"approval",Approval:&fg.Approval{Mode:"all",AssigneeIDs:[]string{rootExecutionID(8),rootExecutionID(9)}}},
  {ID:rootExecutionID(3),Kind:"approval",Approval:&fg.Approval{Mode:"all",AssigneeIDs:[]string{rootExecutionID(10),rootExecutionID(11),rootExecutionID(12)}}},
  {ID:rootExecutionID(4),Kind:"end"},
 },Edges:[]fg.Edge{{From:rootExecutionID(1),To:rootExecutionID(2)},{From:rootExecutionID(2),To:rootExecutionID(3)},{From:rootExecutionID(3),To:rootExecutionID(4)}}}
 raw,e:=fg.CompileBPMN(graph,nil,c.VersionID);if e!=nil{t.Fatal(e)}
 if _,e=deploy.Deploy(context.Background(),&pb.DeployRequest{AppId:c.AppID,FlowId:c.FlowID,VersionId:c.VersionID,Version:1,BpmnXml:raw});e!=nil{t.Fatal(e)}
 client,e:=NewExecutionClient(conn,15*time.Second);if e!=nil{t.Fatal(e)};return client,c,p
}
func rootAction(c fc.Command,r *ExecutionConfirmed,id int,action string,task *fc.ExecutionTask)(fc.Command,fc.ExecutionPayload){
 c.CommandID=rootExecutionID(id);c.Action=action;c.ExpectedSequence=r.Receipt.Sequence;c.FenceEpoch++
 c.TaskID="";c.TaskEpoch=0;c.TargetNodeID=""
 if task!=nil{c.TaskID=task.ID;c.TaskEpoch=task.ActivationEpoch;c.ActorID=task.AssigneeID}else{c.ActorID=rootExecutionID(106)}
 p:=fc.ExecutionPayload{EvidenceHash:sha256.Sum256([]byte("root-interop-evidence")),Routes:map[string]bool{}}
 b,e:=fc.EncodeExecutionPayload(action,p);if e!=nil{panic(e)};c.PayloadHash=sha256.Sum256(b);return c,p
}
func rootLookupMatches(t *testing.T,client *ExecutionClient,c fc.Command,want *ExecutionConfirmed){
 t.Helper();got,found,e:=client.Lookup(context.Background(),c)
 if e!=nil||!found||!reflect.DeepEqual(got,want){t.Fatalf("durable lookup differs: %v %v",found,e)}
}
func TestRootGoJavaPostgresExecutionLifecycleInterop(t *testing.T){
 started:=time.Now();client,c,p:=rootRealExecution(t);ctx:=context.Background()
 if r,found,e:=client.Lookup(ctx,c);e!=nil||found||r!=nil{t.Fatalf("initial lookup: %v",e)}
 r,e:=client.Execute(ctx,c,p);if e!=nil{t.Fatal(e)}
 if r.Result.State!="active"||len(r.Result.Tasks)!=2||r.Receipt.Sequence!=1{t.Fatal("start did not activate real first node")}
 rootLookupMatches(t,client,c,r)
 again,e:=client.Execute(ctx,c,p);if e!=nil||!reflect.DeepEqual(r,again){t.Fatal("start replay changed facts")}
 for i:=0;i<5;i++{
  if len(r.Result.Tasks)==0{t.Fatal("completed before all required approvals")}
  c,p=rootAction(c,r,210+i,"agree",&r.Result.Tasks[0]);if i==1{c.RecordVersion=2}
  r,e=client.Execute(ctx,c,p);if e!=nil{t.Fatal(e)};rootLookupMatches(t,client,c,r)
  expected:=[]int{1,3,2,1,0}[i];if len(r.Result.Tasks)!=expected{t.Fatalf("approval %d tasks=%d want=%d",i,len(r.Result.Tasks),expected)}
  if r.Result.RecordVersion!=c.RecordVersion{t.Fatal("latest record version lost")}
 }
 if r.Result.State!="completed"||r.Receipt.Sequence!=6{t.Fatal("real flow did not complete")}
 t.Logf("isolated deployment/start/replay/5 approvals/lookups elapsed=%s",time.Since(started))
}
func TestRootGoJavaPostgresExecutionCancellationInterop(t *testing.T){
 client,c,p:=rootRealExecution(t);c.CommandID=rootExecutionID(301);c.InstanceID=rootExecutionID(302)
 ctx:=context.Background();r,e:=client.EstablishNoEffect(ctx,c,p);if e!=nil{t.Fatal(e)}
 if r.Receipt.Outcome!="no_effect"||r.Result.State!="unchanged"||r.Result.Reason!="cancelled"{t.Fatal("cancellation not proven")}
 rootLookupMatches(t,client,c,r);again,e:=client.Execute(ctx,c,p);if e!=nil||!reflect.DeepEqual(r,again){t.Fatal("cancelled command was executed")}
}
func TestRootGoJavaPostgresExecutionReturnAndWithdrawInterop(t *testing.T){
 client,c,p:=rootRealExecution(t);c.CommandID=rootExecutionID(401);c.InstanceID=rootExecutionID(402);ctx:=context.Background()
 r,e:=client.Execute(ctx,c,p);if e!=nil{t.Fatal(e)}
 old:=append([]fc.ExecutionTask(nil),r.Result.Tasks...)
 c,p=rootAction(c,r,403,"return",&r.Result.Tasks[0]);c.TargetNodeID=rootExecutionID(2)
 r,e=client.Execute(ctx,c,p);if e!=nil{t.Fatal(e)}
 if len(r.Result.Tasks)!=2||r.Result.Tasks[0].ActivationEpoch<=old[0].ActivationEpoch{t.Fatal("return did not reactivate")}
 for _,a:=range old{for _,b:=range r.Result.Tasks{if a.ID==b.ID{t.Fatal("old task reused")}}}
 rootLookupMatches(t,client,c,r)
 c,p=rootAction(c,r,404,"withdraw",nil);r,e=client.Execute(ctx,c,p);if e!=nil{t.Fatal(e)}
 if r.Result.State!="withdrawn"||len(r.Result.Tasks)!=0{t.Fatal("withdraw not confirmed")};rootLookupMatches(t,client,c,r)
}
