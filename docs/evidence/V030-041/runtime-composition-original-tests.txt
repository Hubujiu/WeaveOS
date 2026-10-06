package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import com.google.protobuf.ByteString;
import io.grpc.*;
import io.grpc.health.v1.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.stub.MetadataUtils;
import java.net.*;
import java.sql.*;
import java.util.*;
import java.util.concurrent.*;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.weaveos.workflow.v1.*;

/** Root-owned real runtime composition, separate from the later OS-process/BFF acceptance. */
class RootWorkflowRuntimeTest {
 static final String TOKEN="V041_SYNTHETIC_TEST_ONLY_1234567890";
 static final String FAILURE="workflow runtime initialization failed";
 final RootRuntimeDataSourceTest database=new RootRuntimeDataSourceTest();
 WorkflowRuntime runtime;ManagedChannel channel;Map<String,String> env;int port;
 @BeforeEach void setup()throws Exception{
  database.setup();env=database.environment();
  try(var socket=new ServerSocket(0,0,InetAddress.getByName("127.0.0.1"))){port=socket.getLocalPort();}
  env.put("WEAVEOS_ENGINE_PORT",Integer.toString(port));env.put("WEAVEOS_ENGINE_BIND_HOST","127.0.0.1");
  env.put("WEAVEOS_ENGINE_CONNECTION_TIMEOUT_MS","1000");env.put("WEAVEOS_ENGINE_STATEMENT_TIMEOUT_MS","10000");
  env.put("WEAVEOS_ENGINE_LOCK_TIMEOUT_MS","8000");env.put("WEAVEOS_ENGINE_SHUTDOWN_TIMEOUT_MS","1000");
 }
 @AfterEach void cleanup()throws Exception{
  try{disconnect();if(runtime!=null)runtime.close();}finally{database.cleanup();}
 }
 void start(){
  runtime=WorkflowRuntime.start(RuntimeConfiguration.read(env));assertNotNull(runtime,"a running composed engine is required");assertEquals(port,runtime.port());
  channel=NettyChannelBuilder.forAddress("127.0.0.1",port).usePlaintext().disableRetry().build();
 }
 void disconnect()throws Exception{if(channel!=null){channel.shutdownNow();channel.awaitTermination(5,TimeUnit.SECONDS);channel=null;}}
 Channel authorized(){Metadata headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+TOKEN);return ClientInterceptors.intercept(channel,MetadataUtils.newAttachHeadersInterceptor(headers));}
 HealthGrpc.HealthBlockingStub health(){return HealthGrpc.newBlockingStub(authorized()).withDeadlineAfter(3,TimeUnit.SECONDS);}
 DeploymentServiceGrpc.DeploymentServiceBlockingStub deployments(){return DeploymentServiceGrpc.newBlockingStub(authorized()).withDeadlineAfter(15,TimeUnit.SECONDS);}
 ExecutionServiceGrpc.ExecutionServiceBlockingStub executions(){return ExecutionServiceGrpc.newBlockingStub(authorized()).withDeadlineAfter(15,TimeUnit.SECONDS);}
 void code(Status.Code expected,org.junit.jupiter.api.function.Executable action){assertEquals(expected,assertThrows(StatusRuntimeException.class,action).getStatus().getCode());}
 void deploy()throws Exception{
  byte[] xml;try(var input=getClass().getResourceAsStream("/return-compatibility/return-all.bpmn20.xml")){assertNotNull(input);xml=input.readAllBytes();}
  var receipt=deployments().deploy(DeployRequest.newBuilder().setAppId(RootExecutionRegistryTest.APP).setFlowId(RootExecutionRegistryTest.FLOW).setVersionId(RootExecutionRegistryTest.VERSION).setVersion(1).setBpmnXml(ByteString.copyFrom(xml)).build());
  assertFalse(receipt.getProcessDefinitionId().isEmpty());
 }
 void noConnections()throws Exception{
  long deadline=System.nanoTime()+5_000_000_000L;int count;
  do{count=database.fixture.admin.queryForObject("SELECT count(*) FROM pg_stat_activity WHERE usename=?",Integer.class,database.fixture.role);if(count==0)break;Thread.sleep(10);}while(System.nanoTime()<deadline);
  assertEquals(0,count);
 }
 void portAvailable()throws Exception{try(var socket=new ServerSocket()){socket.setReuseAddress(true);socket.bind(new InetSocketAddress("127.0.0.1",port));}}
 @Test void authenticatedHealthReflectsRegisteredServices(){
  start();for(String name:List.of("","weaveos.workflow.v1.DeploymentService","weaveos.workflow.v1.ExecutionService"))
   assertEquals(HealthCheckResponse.ServingStatus.SERVING,health().check(HealthCheckRequest.newBuilder().setService(name).build()).getStatus());
 }
 @Test void everyRegisteredServiceRejectsMissingIdentity(){
  start();code(Status.Code.UNAUTHENTICATED,()->HealthGrpc.newBlockingStub(channel).withDeadlineAfter(3,TimeUnit.SECONDS).check(HealthCheckRequest.getDefaultInstance()));
  code(Status.Code.UNAUTHENTICATED,()->DeploymentServiceGrpc.newBlockingStub(channel).withDeadlineAfter(3,TimeUnit.SECONDS).lookup(LookupRequest.getDefaultInstance()));
  code(Status.Code.UNAUTHENTICATED,()->ExecutionServiceGrpc.newBlockingStub(channel).withDeadlineAfter(3,TimeUnit.SECONDS).lookup(ExecutionLookupRequest.getDefaultInstance()));
  assertEquals(0,database.fixture.jdbc.queryForObject("SELECT count(*) FROM wf_deployments",Integer.class));assertEquals(0,database.fixture.jdbc.queryForObject("SELECT count(*) FROM wf_execution_commands",Integer.class));
 }
 @Test void unknownHealthServiceIsNotFalselyServing(){start();code(Status.Code.NOT_FOUND,()->health().check(HealthCheckRequest.newBuilder().setService("unregistered").build()));}
 @Test void controlledPublishStartAndApprovalUseTheRestrictedRole()throws Exception{
  start();deploy();var initial=new RootExecutionRegistryTest.Command();var first=executions().execute(RootExecutionGrpcIT.wire(initial));assertEquals("success",first.getOutcome());assertEquals(1,first.getSequence());
  var task=database.fixture.jdbc.queryForMap("SELECT task_id,activation_epoch FROM wf_execution_tasks WHERE instance_id=?::uuid AND assignee_id=?::uuid",initial.instanceId(),RootExecutionRegistryTest.id(8));
  var agree=initial.copy();agree.strings[0]=UUID.randomUUID().toString();agree.strings[8]=task.get("task_id").toString();agree.strings[9]=RootExecutionRegistryTest.id(8);agree.strings[10]="agree";
  agree.numbers[3]=2;agree.numbers[4]=((Number)task.get("activation_epoch")).longValue();agree.numbers[5]=1;agree.payload=RootExecutionRegistryTest.actionPayload();
  var second=executions().execute(RootExecutionGrpcIT.wire(agree));assertEquals("success",second.getOutcome());assertEquals(2,second.getSequence());
  assertEquals(2,database.fixture.jdbc.queryForObject("SELECT count(*) FROM wf_execution_commands WHERE outcome='success'",Integer.class));
  assertEquals(1,database.fixture.jdbc.queryForObject("SELECT count(*) FROM act_hi_taskinst WHERE end_time_ IS NOT NULL",Integer.class));
 }
 @Test void closeAndRestartReplayTheSameDurableReceipt()throws Exception{
  start();deploy();var command=new RootExecutionRegistryTest.Command();var receipt=executions().execute(RootExecutionGrpcIT.wire(command));
  disconnect();runtime.close();runtime=null;noConnections();start();
  assertEquals(receipt,executions().lookup(RootExecutionGrpcIT.query(command)).getConfirmed());assertEquals(receipt,executions().execute(RootExecutionGrpcIT.wire(command)));
  assertEquals(1,database.fixture.jdbc.queryForObject("SELECT count(*) FROM wf_execution_commands",Integer.class));assertEquals(1,database.fixture.jdbc.queryForObject("SELECT count(*) FROM act_hi_procinst",Integer.class));
 }
 @Test void missingSchemaFailsBeforeListeningOrAutoDdl()throws Exception{
  database.fixture.jdbc.execute("DROP TABLE act_hi_actinst CASCADE");var error=assertThrows(IllegalStateException.class,()->WorkflowRuntime.start(RuntimeConfiguration.read(env)));assertEquals(FAILURE,error.getMessage());assertNull(error.getCause());
  assertNull(database.fixture.jdbc.queryForObject("SELECT to_regclass(?::text)",String.class,database.fixture.schema+".act_hi_actinst"));noConnections();portAvailable();
 }
 @Test void privilegedAccountCannotStartTheRuntime()throws Exception{
  env.put("WEAVEOS_ENGINE_DB_USER","b3_fixture");env.put("WEAVEOS_ENGINE_DB_PASSWORD","b3_fixture_only");
  var error=assertThrows(IllegalStateException.class,()->WorkflowRuntime.start(RuntimeConfiguration.read(env)));assertEquals(FAILURE,error.getMessage());assertNull(error.getCause());portAvailable();
 }
 @Test void occupiedPortStartupFailureReleasesDatabaseResources()throws Exception{
  try(var occupied=new ServerSocket(port,0,InetAddress.getByName("127.0.0.1"))){var error=assertThrows(IllegalStateException.class,()->WorkflowRuntime.start(RuntimeConfiguration.read(env)));assertEquals(FAILURE,error.getMessage());assertNull(error.getCause());noConnections();}
 }
 @Test void closeIsIdempotentStopsServingAndReleasesResources()throws Exception{
  start();assertDoesNotThrow(runtime::close);assertDoesNotThrow(runtime::close);noConnections();portAvailable();
  assertThrows(StatusRuntimeException.class,()->health().withDeadlineAfter(1,TimeUnit.SECONDS).check(HealthCheckRequest.getDefaultInstance()));
  assertTimeoutPreemptively(java.time.Duration.ofSeconds(1),runtime::awaitTermination);
 }
 @Test void healthIsNotServingWhenDatabaseCannotBeReached()throws Exception{
  start();database.fixture.admin.execute("ALTER ROLE "+database.fixture.role+" NOLOGIN");
  database.fixture.admin.queryForList("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename=?",database.fixture.role);
  assertEquals(HealthCheckResponse.ServingStatus.NOT_SERVING,health().check(HealthCheckRequest.getDefaultInstance()).getStatus());
 }
 @Test void configuredCallCapacityRejectsOverloadWithoutUnboundedWorkers()throws Exception{
  env.put("WEAVEOS_ENGINE_RPC_THREADS","1");env.put("WEAVEOS_ENGINE_QUEUE_CAPACITY","1");start();
  var request=LookupRequest.newBuilder().setAppId(RootExecutionRegistryTest.APP).setFlowId(RootExecutionRegistryTest.FLOW).setVersionId(RootExecutionRegistryTest.VERSION).setVersion(1).setBpmnSha256("0".repeat(64)).build();
  var senders=Executors.newFixedThreadPool(8);var futures=new ArrayList<Future<Status.Code>>();
  try(var owner=new DriverManagerDataSource(database.fixture.url+"?currentSchema="+database.fixture.schema,"b3_fixture","b3_fixture_only").getConnection()){
   owner.setAutoCommit(false);try(var lock=owner.createStatement()){
    lock.execute("LOCK TABLE wf_deployments IN ACCESS EXCLUSIVE MODE");
    for(int i=0;i<8;i++)futures.add(senders.submit(()->{try{deployments().lookup(request);return Status.Code.OK;}catch(StatusRuntimeException failure){return failure.getStatus().getCode();}}));
    boolean rejected=false;long until=System.nanoTime()+5_000_000_000L;
    do{for(var future:futures)if(future.isDone()&&future.get()==Status.Code.RESOURCE_EXHAUSTED)rejected=true;if(rejected)break;Thread.sleep(10);}while(System.nanoTime()<until);
    assertTrue(rejected,"bounded call capacity must explicitly reject excess work");
    int blocked=database.fixture.admin.queryForObject("SELECT count(*) FROM pg_stat_activity WHERE usename=? AND wait_event_type='Lock'",Integer.class,database.fixture.role);assertTrue(blocked<=1,"one configured RPC worker must not run many blocked queries");
   }finally{owner.rollback();}
   for(var future:futures)assertTrue(Set.of(Status.Code.OK,Status.Code.RESOURCE_EXHAUSTED).contains(future.get(10,TimeUnit.SECONDS)));
  }finally{senders.shutdownNow();senders.awaitTermination(5,TimeUnit.SECONDS);}
 }
 @Test void nullConfigurationFailsClosed(){var error=assertThrows(IllegalStateException.class,()->WorkflowRuntime.start(null));assertEquals(FAILURE,error.getMessage());assertNull(error.getCause());}
}
