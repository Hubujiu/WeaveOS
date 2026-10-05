package workflowrpc

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "strings"
 "testing"
 "time"
 pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
 "google.golang.org/grpc"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
 "google.golang.org/protobuf/proto"
)
const appID="10000000-0000-4000-8000-000000000001"
const flowID="20000000-0000-4000-8000-000000000001"
const versionID="30000000-0000-4000-8000-000000000001"
func request() *pb.DeployRequest{return &pb.DeployRequest{AppId:appID,FlowId:flowID,VersionId:versionID,Version:1,BpmnXml:[]byte("<exact bytes>")}}
func digest(b []byte)string{h:=sha256.Sum256(b);return hex.EncodeToString(h[:])}
func receipt(q *pb.DeployRequest)*pb.DeploymentReceipt{return &pb.DeploymentReceipt{AppId:q.AppId,FlowId:q.FlowId,VersionId:q.VersionId,Version:q.Version,BpmnSha256:digest(q.BpmnXml),EngineDeploymentId:"deployment-1",ProcessDefinitionId:"definition-1"}}
func lookupRequest(q *pb.DeployRequest)*pb.LookupRequest{return &pb.LookupRequest{AppId:q.AppId,FlowId:q.FlowId,VersionId:q.VersionId,Version:q.Version,BpmnSha256:digest(q.BpmnXml)}}
type connection struct{ calls int; invoke func(context.Context,string,any,any)error }
func(c *connection)Invoke(ctx context.Context,method string,in,out any,_ ...grpc.CallOption)error{c.calls++;return c.invoke(ctx,method,in,out)}
func(c *connection)NewStream(context.Context,*grpc.StreamDesc,string,...grpc.CallOption)(grpc.ClientStream,error){return nil,errors.New("unexpected streaming")}
func client(t *testing.T,c *connection)*Client{t.Helper();v,e:=NewClient(c,time.Second);if e!=nil{t.Fatal(e)};return v}
func TestRootExactDeploymentReceipt(t *testing.T){
 q:=request();c:=&connection{invoke:func(ctx context.Context,m string,in,out any)error{
  if m!="/weaveos.workflow.v1.DeploymentService/Deploy"{t.Fatalf("method %s",m)}
  if !proto.Equal(q,in.(*pb.DeployRequest)){t.Fatal("request changed")}
  if _,ok:=ctx.Deadline();!ok{t.Fatal("no deadline")}
  proto.Merge(out.(proto.Message),receipt(q));return nil
 }}
 r,e:=client(t,c).Deploy(context.Background(),q);if e!=nil||!proto.Equal(r,receipt(q)){t.Fatalf("receipt %v error %v",r,e)}
 if c.calls!=1{t.Fatalf("calls %d",c.calls)}
}
func TestRootRejectsUnboundDeploymentReceipts(t *testing.T){
 mutations:=[]func(*pb.DeploymentReceipt){
  func(r *pb.DeploymentReceipt){r.AppId=flowID},func(r *pb.DeploymentReceipt){r.FlowId=appID},func(r *pb.DeploymentReceipt){r.VersionId=appID},
  func(r *pb.DeploymentReceipt){r.Version=2},func(r *pb.DeploymentReceipt){r.BpmnSha256=strings.Repeat("0",64)},
  func(r *pb.DeploymentReceipt){r.EngineDeploymentId=""},func(r *pb.DeploymentReceipt){r.ProcessDefinitionId=" "},
 }
 for i,mutate:=range mutations{t.Run(string(rune('a'+i)),func(t *testing.T){q:=request();bad:=receipt(q);mutate(bad)
  c:=&connection{invoke:func(_ context.Context,_ string,_,out any)error{proto.Merge(out.(proto.Message),bad);return nil}}
  if r,e:=client(t,c).Deploy(context.Background(),q);e==nil||r!=nil{t.Fatalf("untrusted receipt accepted: %v %v",r,e)}
 })}
}
func TestRootInvalidInputsNeverInvokeNetwork(t *testing.T){
 qs:=[]*pb.DeployRequest{nil,{},request(),request(),request(),request(),request(),request()}
 qs[2].AppId="AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA";qs[3].Version=0;qs[4].Version=9007199254740992
 qs[5].BpmnXml=[]byte{0xc3,0x28};qs[6].BpmnXml=make([]byte,1024*1024+1);qs[7].VersionId="00000000-0000-0000-0000-000000000000"
 c:=&connection{invoke:func(context.Context,string,any,any)error{return errors.New("network must not run")}};v:=client(t,c)
 for _,q:=range qs{if _,e:=v.Deploy(context.Background(),q);e==nil{t.Fatal("invalid request accepted")}}
 for _,q:=range []*pb.LookupRequest{nil,{}, {AppId:appID,FlowId:flowID,VersionId:versionID,Version:1,BpmnSha256:"invalid"}}{
  if _,e:=v.Lookup(context.Background(),q);e==nil{t.Fatal("invalid lookup accepted")}
 }
 if c.calls!=0{t.Fatalf("invalid requests reached network %d",c.calls)}
}
func TestRootLookupDistinguishesConfirmedUnobservedAndMalformed(t *testing.T){
 q:=request()
 cases:=[]struct{name string;r *pb.LookupResponse;bad bool}{
  {"confirmed",&pb.LookupResponse{Result:&pb.LookupResponse_Confirmed{Confirmed:receipt(q)}},false},
  {"not observed",&pb.LookupResponse{Result:&pb.LookupResponse_NotObserved{NotObserved:&pb.NotObserved{}}},false},
  {"unset",&pb.LookupResponse{},true},
  {"wrong context",&pb.LookupResponse{Result:&pb.LookupResponse_Confirmed{Confirmed:receipt(&pb.DeployRequest{AppId:flowID,FlowId:flowID,VersionId:versionID,Version:1,BpmnXml:q.BpmnXml})}},true},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  c:=&connection{invoke:func(_ context.Context,m string,in,out any)error{
   if m!="/weaveos.workflow.v1.DeploymentService/Lookup"||!proto.Equal(in.(*pb.LookupRequest),lookupRequest(q)){t.Fatal("lookup changed")}
   proto.Merge(out.(proto.Message),tc.r);return nil}}
  got,e:=client(t,c).Lookup(context.Background(),lookupRequest(q));if tc.bad{if e==nil||got!=nil{t.Fatal("malformed lookup accepted")}}else if e!=nil||!proto.Equal(got,tc.r){t.Fatalf("%v %v",got,e)}
 })}
}
func TestRootTransportErrorsPreserveUnknownWithoutRetry(t *testing.T){
 for _,code:=range []codes.Code{codes.Unavailable,codes.DeadlineExceeded,codes.Canceled,codes.AlreadyExists}{
  c:=&connection{invoke:func(context.Context,string,any,any)error{return status.Error(code,"synthetic")}}
  r,e:=client(t,c).Deploy(context.Background(),request())
  if r!=nil||status.Code(e)!=code||c.calls!=1{t.Fatalf("%v %v calls=%d",r,e,c.calls)}
 }
}
func TestRootDeadlineIsBoundedByParentAndClient(t *testing.T){
 for _,parentShorter:=range []bool{false,true}{
  timeout:=100*time.Millisecond;parent:=context.Background();cancel:=func(){}
  if parentShorter{parent,cancel=context.WithTimeout(parent,20*time.Millisecond)}
  c:=&connection{invoke:func(ctx context.Context,_ string,_,_ any)error{
   d,ok:=ctx.Deadline();if !ok{t.Fatal("missing deadline")}
   max:=timeout;if parentShorter{max=20*time.Millisecond};if time.Until(d)>max{t.Fatal("deadline extended")}
   <-ctx.Done();return status.FromContextError(ctx.Err()).Err()
  }}
  v,e:=NewClient(c,timeout);if e!=nil{t.Fatal(e)};_,e=v.Deploy(parent,request());cancel()
  if status.Code(e)!=codes.DeadlineExceeded||c.calls!=1{t.Fatalf("%v calls=%d",e,c.calls)}
 }
}
func TestRootConstructorRejectsUnboundedConfiguration(t *testing.T){
 c:=&connection{}
 for _,d:=range []time.Duration{0,-time.Second,31*time.Second}{if _,e:=NewClient(c,d);e==nil{t.Fatalf("accepted timeout %v",d)}}
 if _,e:=NewClient(nil,time.Second);e==nil{t.Fatal("accepted nil connection")}
}
