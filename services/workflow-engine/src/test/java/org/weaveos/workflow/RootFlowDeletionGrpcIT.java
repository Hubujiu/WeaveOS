package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import io.grpc.*;
import io.grpc.stub.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import java.time.Instant;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.*;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.workflow.v1.*;

/** Frozen V067 RPC contract, real authenticated transport and real engine/database. */
class RootFlowDeletionGrpcIT {
 final RootFlowDeletionRegistryTest f=new RootFlowDeletionRegistryTest();
 static final String TOKEN="v067_synthetic_internal_test_identity_only";
 Server server; ManagedChannel channel; DeploymentGrpcService adapter;
 DeploymentServiceGrpc.DeploymentServiceBlockingStub stub,anonymous;
 FlowDeletionRequest request(){return FlowDeletionRequest.newBuilder().setAppId(f.n.APP).setFlowId(f.n.FLOW).setOperationId(f.OP).build();}
 @BeforeEach void setup()throws Exception{
  f.setup();adapter=new DeploymentGrpcService(f.f.registry,f.deletion);
  server=NettyServerBuilder.forPort(0).addService(ServerInterceptors.intercept(adapter,new ServiceTokenInterceptor(TOKEN))).build().start();
  channel=NettyChannelBuilder.forAddress("127.0.0.1",server.getPort()).usePlaintext().disableRetry().build();
  anonymous=DeploymentServiceGrpc.newBlockingStub(channel).withDeadlineAfter(10,TimeUnit.SECONDS);
  Metadata headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+TOKEN);
  stub=anonymous.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));
 }
 @AfterEach void cleanup()throws Exception{
  if(channel!=null)channel.shutdownNow().awaitTermination(10,TimeUnit.SECONDS);
  if(server!=null)server.shutdownNow().awaitTermination(10,TimeUnit.SECONDS);
  f.cleanup();
 }
 void code(Status.Code expected,org.junit.jupiter.api.function.Executable action){var e=assertThrows(StatusRuntimeException.class,action);assertEquals(expected,e.getStatus().getCode());assertFalse(e.getStatus().getDescription().contains("SELECT"));}
 @Test void authenticatedDeleteAndLookupReturnOriginalCommittedIdentityAndTime()throws Exception{
  var deployment=f.f.registry.deploy(f.n.request(f.n.V1,1,"all"));var q=request();var r=stub.deleteFlow(q);
  assertEquals(q.getAppId(),r.getAppId());assertEquals(q.getFlowId(),r.getFlowId());assertEquals(q.getOperationId(),r.getOperationId());assertEquals(1,r.getDeletedVersions());
  assertEquals(f.deletion.lookup(f.request()).orElseThrow().deletedAt(),Instant.ofEpochSecond(r.getDeletedAtSeconds(),r.getDeletedAtNanos()));
  assertEquals(0,r.getDeletedAtNanos()%1000);assertEquals(r,stub.lookupFlowDeletion(q).getConfirmed());assertEquals(r,stub.deleteFlow(q));f.c.absent(deployment);
 }
 @Test void bothNewMethodsRejectMissingIdentityBeforeBusinessDispatch(){
  code(Status.Code.UNAUTHENTICATED,()->anonymous.deleteFlow(request()));code(Status.Code.UNAUTHENTICATED,()->anonymous.lookupFlowDeletion(request()));
  assertEquals(0,f.f.jdbc.queryForObject("SELECT count(*) FROM wf_flow_deletion_guards",Long.class));
 }
 @Test void missingAndCrossScopeLookupAreExplicitNotObservedAndReadOnly(){
  assertTrue(stub.lookupFlowDeletion(request()).hasNotObserved());assertEquals(0,f.f.jdbc.queryForObject("SELECT count(*) FROM wf_flow_deletion_guards",Long.class));
  stub.deleteFlow(request());assertTrue(stub.lookupFlowDeletion(request().toBuilder().setAppId(f.n.OTHER_APP).build()).hasNotObserved());
  assertTrue(stub.lookupFlowDeletion(request().toBuilder().setOperationId("70000000-0000-4000-8000-000000000002").build()).hasNotObserved());
  assertEquals(1,f.f.jdbc.queryForObject("SELECT count(*) FROM wf_flow_deletion_guards",Long.class));
 }
 @Test void invalidUUIDAndOversizedUnknownFieldsAreInvalidArgument(){
  for(var q:java.util.List.of(request().toBuilder().setAppId("bad").build(),request().toBuilder().setFlowId("00000000-0000-0000-0000-000000000000").build(),request().toBuilder().setOperationId("bad").build(),request().toBuilder().setUnknownFields(com.google.protobuf.UnknownFieldSet.newBuilder().addField(100,com.google.protobuf.UnknownFieldSet.Field.newBuilder().addLengthDelimited(com.google.protobuf.ByteString.copyFrom(new byte[1024])).build()).build()).build())){
   code(Status.Code.INVALID_ARGUMENT,()->stub.deleteFlow(q));code(Status.Code.INVALID_ARGUMENT,()->stub.lookupFlowDeletion(q));
  }
  assertEquals(0,f.f.jdbc.queryForObject("SELECT count(*) FROM wf_flow_deletion_guards",Long.class));
 }
 @Test void activeInstanceAndChangedOperationHaveDistinctSafeStatus()throws Exception{
  var deployment=f.f.registry.deploy(f.n.request(f.n.V1,1,"all"));String p=f.n.start(deployment);
  code(Status.Code.FAILED_PRECONDITION,()->stub.deleteFlow(request()));assertTrue(f.deletion.lookup(f.request()).isEmpty());f.c.present(deployment);
  f.c.finish(p);stub.deleteFlow(request());code(Status.Code.ALREADY_EXISTS,()->stub.deleteFlow(request().toBuilder().setOperationId("70000000-0000-4000-8000-000000000002").build()));
 }
 static class Observer<T> implements StreamObserver<T>{T value;Throwable error;boolean complete;public void onNext(T v){value=v;}public void onError(Throwable e){error=e;}public void onCompleted(){complete=true;}}
 @Test void outerTransactionCannotSendProvisionalDeletionSuccess(){
  var a=new Observer<FlowDeletionReceipt>();var b=new Observer<FlowDeletionLookupResponse>();
  new TransactionTemplate(f.f.tm).executeWithoutResult(tx->{adapter.deleteFlow(request(),a);adapter.lookupFlowDeletion(request(),b);});
  assertNull(a.value);assertNull(b.value);assertFalse(a.complete);assertFalse(b.complete);
  assertEquals(Status.Code.FAILED_PRECONDITION,Status.fromThrowable(a.error).getCode());assertEquals(Status.Code.FAILED_PRECONDITION,Status.fromThrowable(b.error).getCode());assertTrue(f.deletion.lookup(f.request()).isEmpty());
 }
 static class DeliveryLost extends RuntimeException{}
 @Test void responseLossAfterCommitRecoversSameOperation(){
  var out=new Observer<FlowDeletionReceipt>(){@Override public void onNext(FlowDeletionReceipt r){super.onNext(r);throw new DeliveryLost();}};
  assertThrows(DeliveryLost.class,()->adapter.deleteFlow(request(),out));assertNotNull(out.value);assertNull(out.error);assertFalse(out.complete);
  assertEquals(out.value,stub.lookupFlowDeletion(request()).getConfirmed());assertEquals(out.value,stub.deleteFlow(request()));
 }
}
