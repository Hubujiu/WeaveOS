package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.MetadataUtils;
import io.grpc.stub.StreamObserver;
import java.net.InetSocketAddress;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;
import org.weaveos.workflow.v1.*;

/** Synthetic loopback transport tests; no real credential, database or user data. */
class RootServiceTokenInterceptorTest {
 static final String TOKEN="V041_SYNTHETIC_TEST_ONLY_1234567890";
 static final Metadata.Key<String> AUTH=Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER);
 final AtomicInteger businessCalls=new AtomicInteger();
 Server server; ManagedChannel channel;
 DeploymentServiceGrpc.DeploymentServiceBlockingStub stub;
 void start() throws Exception {
  var service=new DeploymentServiceGrpc.DeploymentServiceImplBase() {
   @Override public void lookup(LookupRequest q,StreamObserver<LookupResponse> response) {
    businessCalls.incrementAndGet();response.onNext(LookupResponse.getDefaultInstance());response.onCompleted();
   }
  };
  server=NettyServerBuilder.forAddress(new InetSocketAddress("127.0.0.1",0))
   .intercept(new ServiceTokenInterceptor(TOKEN)).addService(service).build().start();
  channel=NettyChannelBuilder.forAddress("127.0.0.1",server.getPort()).usePlaintext().disableRetry().build();
  stub=DeploymentServiceGrpc.newBlockingStub(channel).withDeadlineAfter(5,TimeUnit.SECONDS);
 }
 @AfterEach void stop() throws Exception {
  if(channel!=null) {channel.shutdownNow();assertTrue(channel.awaitTermination(5,TimeUnit.SECONDS));}
  if(server!=null) {server.shutdownNow();assertTrue(server.awaitTermination(5,TimeUnit.SECONDS));}
 }
 void call(String...values) {
  Metadata md=new Metadata();for(String value:values) md.put(AUTH,value);
  stub.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(md)).lookup(LookupRequest.getDefaultInstance());
 }
 void reject(String...values) {
  var e=assertThrows(StatusRuntimeException.class,()->call(values));
  assertEquals(Status.Code.UNAUTHENTICATED,e.getStatus().getCode());
  assertEquals("unauthenticated service",e.getStatus().getDescription());
  assertFalse(e.toString().contains(TOKEN));
  assertEquals(0,businessCalls.get(),"unauthorized request reached business handler");
 }
 @Test void rejectsInvalidConfiguredTokensWithoutEchoingInput() {
  for(String token:List.of("","a".repeat(31),"a".repeat(257),TOKEN+" ",TOKEN+"\n",TOKEN+"é",TOKEN+"=",TOKEN+"/")) {
   var e=assertThrows(IllegalArgumentException.class,()->new ServiceTokenInterceptor(token));
   assertEquals("invalid service token configuration",e.getMessage());
  }
  assertThrows(IllegalArgumentException.class,()->new ServiceTokenInterceptor(null));
 }
 @Test void acceptsConfiguredBoundaryTokensAndRedactsIdentity() {
  for(String token:List.of("a".repeat(32),"Z".repeat(256),TOKEN)) {
   var identity=new ServiceTokenInterceptor(token);
   assertEquals("[REDACTED service identity]",identity.toString());
  }
 }
 @Test void validIdentityInvokesHandlerExactlyOnce() throws Exception {start();call("Bearer "+TOKEN);assertEquals(1,businessCalls.get());}
 @Test void missingIdentityIsRejectedBeforeHandler() throws Exception {start();reject();}
 @Test void incorrectIdentityIsRejectedBeforeHandler() throws Exception {start();reject("Bearer "+"x".repeat(40));}
 @Test void duplicateIdentityIsRejectedEvenWhenOneOrBothAreValid() throws Exception {
  start();reject("Bearer "+TOKEN,"Bearer "+TOKEN);reject("Bearer wrong","Bearer "+TOKEN);reject("Bearer "+TOKEN,"Bearer wrong");
 }
 @Test void ambiguousBearerFormattingIsRejected() throws Exception {
  start();for(String value:List.of(TOKEN,"bearer "+TOKEN,"Bearer  "+TOKEN,"Bearer "+TOKEN+" ","Basic "+TOKEN,"Bearer "+TOKEN+",Bearer "+TOKEN))reject(value);
 }
 @Test void badIdentityDoesNotPoisonLaterValidRequest() throws Exception {start();reject("Bearer wrong");call("Bearer "+TOKEN);assertEquals(1,businessCalls.get());}
}
