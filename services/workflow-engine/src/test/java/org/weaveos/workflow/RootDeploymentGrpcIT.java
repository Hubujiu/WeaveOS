package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import javax.sql.DataSource;
import org.flowable.engine.ProcessEngine;
import org.flowable.engine.ProcessEngineConfiguration;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.workflow.DeploymentRegistry.*;

class RootDeploymentGrpcIT {
 io.grpc.Server server; io.grpc.ManagedChannel channel;
 org.weaveos.workflow.v1.DeploymentServiceGrpc.DeploymentServiceBlockingStub stub;
 void start(io.grpc.BindableService service)throws Exception{
  server=io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder.forPort(0)
   .maxInboundMessageSize(1024*1024+16384).addService(service).build().start();
  channel=io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder.forAddress("127.0.0.1",server.getPort()).usePlaintext().disableRetry().build();
  stub=org.weaveos.workflow.v1.DeploymentServiceGrpc.newBlockingStub(channel).withDeadlineAfter(15,TimeUnit.SECONDS);
 }
 void stop(){
  if(channel!=null){channel.shutdownNow();try{channel.awaitTermination(10,TimeUnit.SECONDS);}catch(InterruptedException e){Thread.currentThread().interrupt();}}
  if(server!=null){server.shutdownNow();try{server.awaitTermination(10,TimeUnit.SECONDS);}catch(InterruptedException e){Thread.currentThread().interrupt();}}
 }
 org.weaveos.workflow.v1.DeployRequest wire(){return org.weaveos.workflow.v1.DeployRequest.newBuilder()
  .setAppId(APP).setFlowId(FLOW).setVersionId(VERSION).setVersion(1)
  .setBpmnXml(com.google.protobuf.ByteString.copyFromUtf8(xml(VERSION))).build();}
 static String hash(byte[] bytes)throws Exception{return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes));}
 org.weaveos.workflow.v1.LookupRequest lookup()throws Exception{return org.weaveos.workflow.v1.LookupRequest.newBuilder()
  .setAppId(APP).setFlowId(FLOW).setVersionId(VERSION).setVersion(1).setBpmnSha256(hash(wire().getBpmnXml().toByteArray())).build();}
 void code(io.grpc.Status.Code expected,org.junit.jupiter.api.function.Executable action){
  var e=assertThrows(io.grpc.StatusRuntimeException.class,action);assertEquals(expected,e.getStatus().getCode());
 }
 static final String APP="10000000-0000-4000-8000-000000000001";
 static final String FLOW="20000000-0000-4000-8000-000000000001";
 static final String VERSION="30000000-0000-4000-8000-000000000001";
 JdbcTemplate admin,jdbc; DataSource ds; DataSourceTransactionManager tm;
 ProcessEngine engine; DeploymentRegistry registry; String schema,url;
 @BeforeEach void setup() throws Exception {
  url=System.getenv("B3_TEST_JDBC_URL");
  assertEquals("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture",url,
   "Only the dedicated unexposed synthetic database may run these tests");
  admin=new JdbcTemplate(new DriverManagerDataSource(url,"b3_fixture","b3_fixture_only"));
  schema="v026_"+UUID.randomUUID().toString().replace("-","");
  admin.execute("CREATE SCHEMA "+schema);open();
  try(var in=getClass().getResourceAsStream("/deployment-registry-fixture.sql")){
   assertNotNull(in);jdbc.execute(new String(in.readAllBytes(),StandardCharsets.UTF_8));
  }
  start(new DeploymentGrpcService(registry));
 }
 void open(){
  ds=new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only");
  tm=new DataSourceTransactionManager(ds);jdbc=new JdbcTemplate(ds);
  var c=new SpringProcessEngineConfiguration();c.setDataSource(ds);c.setTransactionManager(tm);
  c.setDatabaseSchema(schema);c.setDatabaseSchemaUpdate(ProcessEngineConfiguration.DB_SCHEMA_UPDATE_TRUE);
  c.setAsyncExecutorActivate(false);c.setDisableIdmEngine(true);c.setDisableEventRegistry(true);
  engine=c.buildProcessEngine();registry=new DeploymentRegistry(jdbc,tm,engine);
 }
 @AfterEach void cleanup(){stop();if(engine!=null)engine.close();if(admin!=null&&schema!=null)admin.execute("DROP SCHEMA "+schema+" CASCADE");}
 static String xml(String id){
  return "<definitions xmlns=\"http://www.omg.org/spec/BPMN/20100524/MODEL\" xmlns:bpmn=\"http://www.omg.org/spec/BPMN/20100524/MODEL\" xmlns:flowable=\"http://flowable.org/bpmn\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" targetNamespace=\"urn:weaveos:workflow\"><process id=\"p_"+id.replace("-","")+"\" isExecutable=\"true\"><startEvent id=\"n_40000000000040008000000000000001\"></startEvent><endEvent id=\"n_40000000000040008000000000000002\"></endEvent><sequenceFlow id=\"e_0\" sourceRef=\"n_40000000000040008000000000000001\" targetRef=\"n_40000000000040008000000000000002\"></sequenceFlow></process></definitions>";
 }
 static Request request(){return new Request(APP,FLOW,VERSION,1,xml(VERSION));}
 long count(){return jdbc.queryForObject("SELECT count(*) FROM wf_deployments",Long.class);}
 long deployments(){return engine.getRepositoryService().createDeploymentQuery().count();}
 void assertEmpty(){assertEquals(0,count());assertEquals(0,deployments());}
 void verify(Receipt r,Request q)throws Exception{
  assertEquals(q.appId(),r.appId());assertEquals(q.flowId(),r.flowId());assertEquals(q.versionId(),r.versionId());assertEquals(q.version(),r.version());
  assertEquals(HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(q.bpmnXml().getBytes(StandardCharsets.UTF_8))),r.bpmnSha256());
  assertNotNull(r.engineDeploymentId());assertFalse(r.engineDeploymentId().isBlank());
  var definition=engine.getRepositoryService().createProcessDefinitionQuery().processDefinitionId(r.processDefinitionId()).singleResult();
  assertNotNull(definition);assertEquals(r.engineDeploymentId(),definition.getDeploymentId());
  assertEquals("p_"+q.versionId().replace("-",""),definition.getKey());
  assertEquals("confirmed",jdbc.queryForObject("SELECT status FROM wf_deployments WHERE version_id=?::uuid",String.class,q.versionId()));
 }

 @Test void deployReturnsCommittedFullReceipt()throws Exception{
  var q=wire();var r=stub.deploy(q);
  assertEquals(APP,r.getAppId());assertEquals(FLOW,r.getFlowId());assertEquals(VERSION,r.getVersionId());assertEquals(1,r.getVersion());
  assertEquals(hash(q.getBpmnXml().toByteArray()),r.getBpmnSha256());assertFalse(r.getEngineDeploymentId().isBlank());assertFalse(r.getProcessDefinitionId().isBlank());
  assertEquals(1,count());assertEquals(1,deployments());
  assertEquals("confirmed",admin.queryForObject("SELECT status FROM "+schema+".wf_deployments WHERE version_id=?::uuid",String.class,VERSION));
  assertEquals(r,stub.lookup(lookup()).getConfirmed());
 }
 @Test void exactReplayUsesOneDeployment(){
  var a=stub.deploy(wire());assertEquals(a,stub.deploy(wire()));assertEquals(1,count());assertEquals(1,deployments());
 }
 @Test void conflictingContentIsAlreadyExists(){
  stub.deploy(wire());code(io.grpc.Status.Code.ALREADY_EXISTS,()->stub.deploy(wire().toBuilder().setBpmnXml(com.google.protobuf.ByteString.copyFromUtf8(xml(VERSION)+"\n")).build()));
  assertEquals(1,count());assertEquals(1,deployments());
 }
 @Test void lookupNotObservedIsExplicitAndReadOnly()throws Exception{
  var r=stub.lookup(lookup());assertEquals(org.weaveos.workflow.v1.LookupResponse.ResultCase.NOT_OBSERVED,r.getResultCase());assertEmpty();
 }
 @Test void lookupCannotReturnAnotherContext()throws Exception{
  stub.deploy(wire());var q=lookup();
  for(var bad:List.of(q.toBuilder().setAppId(FLOW).build(),q.toBuilder().setFlowId(APP).build(),q.toBuilder().setVersion(2).build(),q.toBuilder().setBpmnSha256("0".repeat(64)).build()))
   code(io.grpc.Status.Code.FAILED_PRECONDITION,()->stub.lookup(bad));
  assertEquals(1,count());
 }
 @Test void invalidWireInputsFailBeforeStore()throws Exception{
  for(var q:List.of(wire().toBuilder().setAppId("").build(),wire().toBuilder().setVersion(0).build(),wire().toBuilder().setVersion(9007199254740992L).build(),
    wire().toBuilder().setVersion(-1L).build(),wire().toBuilder().setBpmnXml(com.google.protobuf.ByteString.copyFrom(new byte[]{(byte)0xc3,(byte)0x28})).build(),
    wire().toBuilder().setBpmnXml(com.google.protobuf.ByteString.copyFromUtf8(" ".repeat(1024*1024+1))).build())){
   code(io.grpc.Status.Code.INVALID_ARGUMENT,()->stub.deploy(q));assertEmpty();
  }
  code(io.grpc.Status.Code.INVALID_ARGUMENT,()->stub.lookup(lookup().toBuilder().setBpmnSha256("bad").build()));assertEmpty();
 }
 @Test void databaseFailureDoesNotLeakDetailsOrLeaveDeployment(){
  jdbc.execute("ALTER TABLE wf_deployments ADD CONSTRAINT synthetic_private_constraint CHECK(status <> 'confirmed')");
  var e=assertThrows(io.grpc.StatusRuntimeException.class,()->stub.deploy(wire()));
  assertEquals(io.grpc.Status.Code.UNAVAILABLE,e.getStatus().getCode());
  assertFalse(e.getMessage().contains("synthetic_private_constraint"));assertFalse(e.getMessage().contains("INSERT"));assertFalse(e.getMessage().contains("b3_fixture_only"));
  assertEmpty();
 }
 @Test void serviceRefusesProvisionalOuterTransaction(){
  var values=new java.util.ArrayList<org.weaveos.workflow.v1.DeploymentReceipt>();var errors=new java.util.ArrayList<Throwable>();var completed=new java.util.concurrent.atomic.AtomicBoolean();
  new TransactionTemplate(tm).executeWithoutResult(tx->{
   new DeploymentGrpcService(registry).deploy(wire(),new io.grpc.stub.StreamObserver<>(){
    public void onNext(org.weaveos.workflow.v1.DeploymentReceipt r){values.add(r);}
    public void onError(Throwable t){errors.add(t);}
    public void onCompleted(){completed.set(true);}
   });tx.setRollbackOnly();
  });
  assertTrue(values.isEmpty());assertFalse(completed.get());assertEquals(1,errors.size());assertEquals(io.grpc.Status.Code.FAILED_PRECONDITION,io.grpc.Status.fromThrowable(errors.get(0)).getCode());assertEmpty();
 }
 @Test void responseLossAndServiceRestartRecoverDurableReceipt()throws Exception{
  stop();var delegate=new DeploymentGrpcService(registry);
  start(new org.weaveos.workflow.v1.DeploymentServiceGrpc.DeploymentServiceImplBase(){
   @Override public void deploy(org.weaveos.workflow.v1.DeployRequest q,io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.DeploymentReceipt> out){
    delegate.deploy(q,new io.grpc.stub.StreamObserver<>(){
     public void onNext(org.weaveos.workflow.v1.DeploymentReceipt r){}
     public void onError(Throwable e){out.onError(e);}
     public void onCompleted(){out.onError(io.grpc.Status.UNAVAILABLE.withDescription("synthetic lost response").asRuntimeException());}
    });
   }
  });
  code(io.grpc.Status.Code.UNAVAILABLE,()->stub.deploy(wire()));assertEquals(1,count());assertEquals(1,deployments());
  stop();engine.close();engine=null;open();start(new DeploymentGrpcService(registry));
  var r=stub.lookup(lookup());assertEquals(org.weaveos.workflow.v1.LookupResponse.ResultCase.CONFIRMED,r.getResultCase());
  assertEquals(r.getConfirmed(),stub.deploy(wire()));assertEquals(1,count());assertEquals(1,deployments());
 }
}
