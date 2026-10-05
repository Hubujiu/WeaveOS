package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import com.google.protobuf.ByteString;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.*;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.*;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.workflow.v1.*;

/** Root-owned transport behavior, reusing the real isolated engine fixture. */
class RootExecutionGrpcIT {
 final RootExecutionRegistryTest f = new RootExecutionRegistryTest();
 io.grpc.Server server;
 io.grpc.ManagedChannel channel;
 ExecutionServiceGrpc.ExecutionServiceBlockingStub stub;
 @BeforeEach void setup() throws Exception {
  f.setup(); f.deploy("all"); start(new ExecutionGrpcService(f.registry));
 }
 void start(io.grpc.BindableService service) throws Exception {
  server=io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder.forPort(0)
   .maxInboundMessageSize(300000).addService(service).build().start();
  channel=io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder.forAddress("127.0.0.1",server.getPort())
   .usePlaintext().disableRetry().build();
  stub=ExecutionServiceGrpc.newBlockingStub(channel).withDeadlineAfter(15,TimeUnit.SECONDS);
 }
 void stop() throws Exception {
  if(channel!=null){channel.shutdownNow();channel.awaitTermination(10,TimeUnit.SECONDS);channel=null;}
  if(server!=null){server.shutdownNow();server.awaitTermination(10,TimeUnit.SECONDS);server=null;}
 }
 @AfterEach void cleanup() throws Exception {try{stop();}finally{f.cleanup();}}
 static ExecutionRequest wire(RootExecutionRegistryTest.Command c){
  return ExecutionRequest.newBuilder().setCommandEnvelope(ByteString.copyFrom(c.bytes()))
   .setPayload(ByteString.copyFrom(c.payload)).build();
 }
 static ExecutionLookupRequest query(RootExecutionRegistryTest.Command c){
  return ExecutionLookupRequest.newBuilder().setCommandId(c.strings[0])
   .setCommandHash(ByteString.copyFrom(RootExecutionRegistryTest.hash(c.bytes()))).build();
 }
 void status(Status.Code code,org.junit.jupiter.api.function.Executable action){
  var e=assertThrows(io.grpc.StatusRuntimeException.class,action);assertEquals(code,e.getStatus().getCode());
 }
 long commands(){return f.jdbc.queryForObject("SELECT count(*) FROM wf_execution_commands",Long.class);}
 long processes(){return f.engine.getRuntimeService().createProcessInstanceQuery().count();}
 void verify(ExecutionReceipt r,RootExecutionRegistryTest.Command c,String outcome) {
  assertEquals(c.strings[0],r.getCommandId());
  assertEquals(ByteString.copyFrom(RootExecutionRegistryTest.hash(c.bytes())),r.getCommandHash());
  assertEquals(outcome,r.getOutcome());assertEquals(c.numbers[5]+(outcome.equals("success")?1:0),r.getSequence());
  assertDoesNotThrow(()->UUID.fromString(r.getProofId()));assertNotEquals("00000000-0000-0000-0000-000000000000",r.getProofId());
  assertEquals(ByteString.copyFrom(RootExecutionRegistryTest.hash(r.getResultBytes().toByteArray())),r.getResultHash());
  assertFalse(r.getResultBytes().isEmpty());
  var durable=f.jdbc.queryForMap("SELECT * FROM wf_execution_commands WHERE command_id=?::uuid",c.strings[0]);
  assertArrayEquals((byte[])durable.get("result_bytes"),r.getResultBytes().toByteArray());
  assertEquals(durable.get("proof_id").toString(),r.getProofId());
 }
 @Test void executeReturnsCommittedBoundReceipt() {
  var c=new RootExecutionRegistryTest.Command();var r=stub.execute(wire(c));verify(r,c,"success");
  assertEquals(1,commands());assertEquals(1,processes());
  assertEquals(1L,f.admin.queryForObject("SELECT count(*) FROM "+f.schema+".wf_execution_commands WHERE outcome='success'",Long.class));
  var found=stub.lookup(query(c));assertEquals(ExecutionLookupResponse.ResultCase.CONFIRMED,found.getResultCase());assertEquals(r,found.getConfirmed());
 }
 @Test void duplicateExecutionReplaysExactReceipt() {
  var c=new RootExecutionRegistryTest.Command();var a=stub.execute(wire(c));assertEquals(a,stub.execute(wire(c)));
  assertEquals(1,commands());assertEquals(1,processes());
 }
 @Test void conflictingCommandCannotReuseIdentity() {
  var c=new RootExecutionRegistryTest.Command();stub.execute(wire(c));c.numbers[2]++;
  status(Status.Code.ALREADY_EXISTS,()->stub.execute(wire(c)));
  status(Status.Code.ALREADY_EXISTS,()->stub.establishNoEffect(wire(c)));assertEquals(1,commands());assertEquals(1,processes());
 }
 @Test void missingLookupIsExplicitAndDoesNotCancel() {
  var c=new RootExecutionRegistryTest.Command();var r=stub.lookup(query(c));
  assertEquals(ExecutionLookupResponse.ResultCase.NOT_OBSERVED,r.getResultCase());assertEquals(0,commands());
  verify(stub.execute(wire(c)),c,"success");assertEquals(1,processes());
 }
 @Test void lookupRejectsDifferentFingerprint() {
  var c=new RootExecutionRegistryTest.Command();stub.execute(wire(c));
  var q=query(c).toBuilder().setCommandHash(ByteString.copyFrom(new byte[32])).build();
  status(Status.Code.FAILED_PRECONDITION,()->stub.lookup(q));assertEquals(1,commands());
 }
 @Test void cancellationFirstDurablyPreventsExecution() {
  var c=new RootExecutionRegistryTest.Command();var r=stub.establishNoEffect(wire(c));verify(r,c,"no_effect");
  assertEquals(r,stub.execute(wire(c)));assertEquals(r,stub.establishNoEffect(wire(c)));assertEquals(r,stub.lookup(query(c)).getConfirmed());
  assertEquals(0,processes());assertEquals(1,commands());
 }
 @Test void executionFirstCannotBecomeCancelled() {
  var c=new RootExecutionRegistryTest.Command();var r=stub.execute(wire(c));
  assertEquals(r,stub.establishNoEffect(wire(c)));assertEquals("success",r.getOutcome());assertEquals(1,processes());
 }
 @Test void invalidRequestsNeverWriteLedger() {
  var c=new RootExecutionRegistryTest.Command();var q=wire(c);
  var invalid=List.of(ExecutionRequest.getDefaultInstance(),q.toBuilder().setCommandEnvelope(ByteString.copyFrom(new byte[539])).build(),
   q.toBuilder().setPayload(ByteString.copyFrom(new byte[262145])).build(),q.toBuilder().setPayload(ByteString.copyFrom(new byte[50])).build());
  for(var bad:invalid){status(Status.Code.INVALID_ARGUMENT,()->stub.execute(bad));status(Status.Code.INVALID_ARGUMENT,()->stub.establishNoEffect(bad));}
  for(var bad:List.of(ExecutionLookupRequest.getDefaultInstance(),query(c).toBuilder().setCommandHash(ByteString.copyFrom(new byte[31])).build(),
    query(c).toBuilder().setCommandId("bad").build()))status(Status.Code.INVALID_ARGUMENT,()->stub.lookup(bad));
  assertEquals(0,commands());assertEquals(0,processes());
 }
 @Test void databaseFailureRollsBackAndHidesPrivateDetails() {
  f.jdbc.execute("ALTER TABLE wf_execution_commands ADD CONSTRAINT private_rpc_constraint CHECK(outcome <> 'success')");
  var e=assertThrows(io.grpc.StatusRuntimeException.class,()->stub.execute(wire(new RootExecutionRegistryTest.Command())));
  assertEquals(Status.Code.UNAVAILABLE,e.getStatus().getCode());
  for(var secret:List.of("private_rpc_constraint","INSERT","b3_fixture_only"))assertFalse(e.getMessage().contains(secret));
  assertEquals(0,commands());assertEquals(0,processes());
 }
 @Test void everyMethodRejectsProvisionalOuterTransaction() {
  var c=new RootExecutionRegistryTest.Command();var service=new ExecutionGrpcService(f.registry);
  for(int method=0;method<3;method++){
   var values=new ArrayList<Object>();var errors=new ArrayList<Throwable>();var completed=new java.util.concurrent.atomic.AtomicBoolean();
   int which=method;
   new TransactionTemplate(f.tm).executeWithoutResult(tx->{
    if(which==1)service.lookup(query(c),observer(values,errors,completed));
    else if(which==0)service.execute(wire(c),observer(values,errors,completed));
    else service.establishNoEffect(wire(c),observer(values,errors,completed));
    tx.setRollbackOnly();
   });
   assertTrue(values.isEmpty());assertFalse(completed.get());assertEquals(1,errors.size());
   assertEquals(Status.Code.FAILED_PRECONDITION,Status.fromThrowable(errors.get(0)).getCode());
  }
  assertEquals(0,commands());assertEquals(0,processes());
 }
 static <T> StreamObserver<T> observer(List<Object> values,List<Throwable> errors,java.util.concurrent.atomic.AtomicBoolean completed){
  return new StreamObserver<>(){public void onNext(T r){values.add(r);}public void onError(Throwable t){errors.add(t);}public void onCompleted(){completed.set(true);}};
 }
 @Test void lostReplyThenServiceRestartRecoversCommittedReceipt() throws Exception {
  stop();var delegate=new ExecutionGrpcService(f.registry);
  start(new ExecutionServiceGrpc.ExecutionServiceImplBase(){
   @Override public void execute(ExecutionRequest q,StreamObserver<ExecutionReceipt> out){
    delegate.execute(q,new StreamObserver<>(){
     public void onNext(ExecutionReceipt r){}
     public void onError(Throwable t){out.onError(t);}
     public void onCompleted(){out.onError(Status.UNAVAILABLE.withDescription("synthetic reply loss").asRuntimeException());}
    });
   }
  });
  var c=new RootExecutionRegistryTest.Command();status(Status.Code.UNAVAILABLE,()->stub.execute(wire(c)));assertEquals(1,commands());
  stop();f.engine.close();f.engine=null;f.open();start(new ExecutionGrpcService(f.registry));
  var r=stub.lookup(query(c));assertEquals(ExecutionLookupResponse.ResultCase.CONFIRMED,r.getResultCase());verify(r.getConfirmed(),c,"success");
  assertEquals(r.getConfirmed(),stub.execute(wire(c)));assertEquals(1,commands());assertEquals(1,processes());
 }
}
