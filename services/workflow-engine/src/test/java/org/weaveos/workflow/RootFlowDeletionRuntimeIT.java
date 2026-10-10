package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import io.grpc.*;
import io.grpc.stub.MetadataUtils;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import java.net.ServerSocket;
import java.util.*;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.*;
import org.weaveos.workflow.v1.*;

/** Formal runtime with explicit schema and genuinely restricted login, never fixture adapter wiring. */
class RootFlowDeletionRuntimeIT {
 final RootFlowDeletionRuntimeSchemaTest f=new RootFlowDeletionRuntimeSchemaTest();
 WorkflowRuntime runtime; ManagedChannel channel; RuntimeConfiguration configuration;
 DeploymentServiceGrpc.DeploymentServiceBlockingStub stub,anonymous;
 int port;
 @BeforeEach void setup()throws Exception{
  f.setup();f.upgrade();try(var socket=new ServerSocket(0)){port=socket.getLocalPort();}
  var env=new HashMap<String,String>();env.put("WEAVEOS_ENGINE_JDBC_URL",f.f.url);env.put("WEAVEOS_ENGINE_DB_USER",f.role);env.put("WEAVEOS_ENGINE_DB_PASSWORD","v067_synthetic_only");env.put("WEAVEOS_ENGINE_SERVICE_TOKEN",RootFlowDeletionGrpcIT.TOKEN);env.put("WEAVEOS_ENGINE_SCHEMA",f.f.schema);env.put("WEAVEOS_ENGINE_BIND_HOST","127.0.0.1");env.put("WEAVEOS_ENGINE_PORT",Integer.toString(port));env.put("WEAVEOS_ENGINE_POOL_MAX","2");env.put("WEAVEOS_ENGINE_CONNECTION_TIMEOUT_MS","1000");env.put("WEAVEOS_ENGINE_STATEMENT_TIMEOUT_MS","10000");env.put("WEAVEOS_ENGINE_LOCK_TIMEOUT_MS","8000");env.put("WEAVEOS_ENGINE_SHUTDOWN_TIMEOUT_MS","1000");
  configuration=RuntimeConfiguration.read(env);start();
 }
 void start(){runtime=WorkflowRuntime.start(configuration);channel=NettyChannelBuilder.forAddress("127.0.0.1",runtime.port()).usePlaintext().disableRetry().build();anonymous=DeploymentServiceGrpc.newBlockingStub(channel).withDeadlineAfter(10,TimeUnit.SECONDS);var headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+RootFlowDeletionGrpcIT.TOKEN);stub=anonymous.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));}
 void stop()throws Exception{if(channel!=null){channel.shutdownNow().awaitTermination(10,TimeUnit.SECONDS);channel=null;}if(runtime!=null){runtime.close();runtime=null;}}
 @AfterEach void cleanup()throws Exception{stop();f.cleanup();}
 FlowDeletionRequest deletion(){return FlowDeletionRequest.newBuilder().setAppId(f.f.app).setFlowId(f.f.flow).setOperationId(f.f.op).build();}
 DeployRequest publication(){return DeployRequest.newBuilder().setAppId(f.f.app).setFlowId(f.f.flow).setVersionId(RootDeploymentGrpcIT.VERSION).setVersion(1).setBpmnXml(com.google.protobuf.ByteString.copyFromUtf8(RootDeploymentGrpcIT.xml(RootDeploymentGrpcIT.VERSION))).build();}
 void absent(){assertEquals(0,f.f.jdbc.queryForObject("SELECT count(*) FROM act_re_procdef",Long.class));assertEquals(0,f.f.jdbc.queryForObject("SELECT count(*) FROM act_ge_bytearray WHERE name_ LIKE '%.bpmn%'",Long.class));}
 @Test void formalRuntimeDeletesWithLeastPrivilegeThenRestartsAndReplaysWithoutResurrection()throws Exception{
  var original=stub.deploy(publication());assertTrue(stub.lookupFlowDeletion(deletion()).hasNotObserved());var receipt=stub.deleteFlow(deletion());assertEquals(1,receipt.getDeletedVersions());absent();assertEquals(original,stub.deploy(publication()));assertEquals(receipt,stub.lookupFlowDeletion(deletion()).getConfirmed());
  var command=new RootExecutionRegistryTest.Command();command.strings[1]=f.f.app;command.strings[5]=f.f.flow;command.strings[6]=RootDeploymentGrpcIT.VERSION;
  var headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+RootFlowDeletionGrpcIT.TOKEN);
  var execution=ExecutionServiceGrpc.newBlockingStub(channel).withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers)).withDeadlineAfter(10,TimeUnit.SECONDS);
  var q=ExecutionRequest.newBuilder().setCommandEnvelope(com.google.protobuf.ByteString.copyFrom(command.bytes())).setPayload(com.google.protobuf.ByteString.copyFrom(command.payload)).build();
  assertEquals("no_effect",execution.execute(q).getOutcome());
  stop();start();assertEquals(receipt,stub.deleteFlow(deletion()));assertEquals(receipt,stub.lookupFlowDeletion(deletion()).getConfirmed());assertEquals(original,stub.deploy(publication()));absent();
 }
 @Test void formalRuntimeRequiresIdentityForBothNewMethods(){
  assertEquals(Status.Code.UNAUTHENTICATED,assertThrows(StatusRuntimeException.class,()->anonymous.deleteFlow(deletion())).getStatus().getCode());assertEquals(Status.Code.UNAUTHENTICATED,assertThrows(StatusRuntimeException.class,()->anonymous.lookupFlowDeletion(deletion())).getStatus().getCode());assertEquals(0,f.f.jdbc.queryForObject("SELECT count(*) FROM wf_flow_deletion_guards",Long.class));
 }
}
