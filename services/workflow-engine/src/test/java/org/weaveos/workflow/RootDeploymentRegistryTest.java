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

class RootDeploymentRegistryTest {
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
  schema="v022_"+UUID.randomUUID().toString().replace("-","");
  admin.execute("CREATE SCHEMA "+schema);open();
  try(var in=getClass().getResourceAsStream("/deployment-registry-fixture.sql")){
   assertNotNull(in);jdbc.execute(new String(in.readAllBytes(),StandardCharsets.UTF_8));
  }
 }
 void open(){
  ds=new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only");
  tm=new DataSourceTransactionManager(ds);jdbc=new JdbcTemplate(ds);
  var c=new SpringProcessEngineConfiguration();c.setDataSource(ds);c.setTransactionManager(tm);
  c.setDatabaseSchema(schema);c.setDatabaseSchemaUpdate(ProcessEngineConfiguration.DB_SCHEMA_UPDATE_TRUE);
  c.setAsyncExecutorActivate(false);c.setDisableIdmEngine(true);c.setDisableEventRegistry(true);
  engine=c.buildProcessEngine();registry=new DeploymentRegistry(jdbc,tm,engine);
 }
 @AfterEach void cleanup(){if(engine!=null)engine.close();if(admin!=null&&schema!=null)admin.execute("DROP SCHEMA "+schema+" CASCADE");}
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
 @Test void confirmedReceiptBindsExactContextHashAndActualEngineDefinition()throws Exception{
  var q=request();var r=registry.deploy(q);verify(r,q);assertEquals(1,count());assertEquals(1,deployments());assertEquals(r,registry.lookup(VERSION).orElseThrow());
 }
 @Test void duplicateReplaysOriginalReceiptWithoutRedeploying()throws Exception{
  var q=request();var first=registry.deploy(q);assertEquals(first,registry.deploy(q));assertEquals(1,count());assertEquals(1,deployments());verify(first,q);
 }
 @Test void sameVersionIdentityRejectsDifferentXmlBytes(){
  var q=request();var r=registry.deploy(q);
  assertThrows(DeploymentConflict.class,()->registry.deploy(new Request(APP,FLOW,VERSION,1,q.bpmnXml()+"\n")));
  assertEquals(r,registry.lookup(VERSION).orElseThrow());assertEquals(1,deployments());
 }
 @Test void sameVersionIdentityCannotChangeApplicationOrFlowOrNumber(){
  registry.deploy(request());
  for(var q:List.of(new Request("10000000-0000-4000-8000-000000000002",FLOW,VERSION,1,xml(VERSION)),
    new Request(APP,"20000000-0000-4000-8000-000000000002",VERSION,1,xml(VERSION)),
    new Request(APP,FLOW,VERSION,2,xml(VERSION)))){
   assertThrows(DeploymentConflict.class,()->registry.deploy(q));
  }assertEquals(1,count());assertEquals(1,deployments());
 }
 @Test void oneLogicalVersionCannotBindTwoVersionIdentities(){
  registry.deploy(request());var id="30000000-0000-4000-8000-000000000002";
  assertThrows(DeploymentConflict.class,()->registry.deploy(new Request(APP,FLOW,id,1,xml(id))));
  assertEquals(1,count());assertEquals(1,deployments());assertTrue(registry.lookup(id).isEmpty());
 }
 @ParameterizedTest @EnumSource(Stage.class)
 void everyPrecommitFailureRollsBackRegistryAndFlowable(Stage point){
  assertThrows(InjectedFailure.class,()->registry.deploy(request(),s->{if(s==point)throw new InjectedFailure();}));assertEmpty();
 }
 @Test void joinsOuterRequiredTransactionWithoutLeavingAnEngineDeployment(){
  new TransactionTemplate(tm).executeWithoutResult(tx->{registry.deploy(request());assertEquals(1,count());assertEquals(1,deployments());tx.setRollbackOnly();});
  assertEmpty();
 }
 @Test void responseLostAfterCommitThenEngineReopenedReplaysDurableReceipt()throws Exception{
  var first=registry.deploy(request());engine.close();engine=null;open();
  assertEquals(first,registry.lookup(VERSION).orElseThrow());assertEquals(first,registry.deploy(request()));verify(first,request());assertEquals(1,count());assertEquals(1,deployments());
 }
 @Test void absentLookupIsEmptyAndHasNoWriteSideEffect(){
  assertTrue(registry.lookup(VERSION).isEmpty());assertEmpty();
 }
 @Test void invalidIdentitiesAndVersionBoundsNeverTouchEitherStore(){
  var cases=List.of(new Request("",FLOW,VERSION,1,xml(VERSION)),
   new Request(APP,"00000000-0000-0000-0000-000000000000",VERSION,1,xml(VERSION)),
   new Request(APP,FLOW,"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA",1,xml(VERSION)),
   new Request(APP,FLOW,VERSION,0,xml(VERSION)),new Request(APP,FLOW,VERSION,9007199254740992L,xml(VERSION)));
  for(var q:cases){assertThrows(InvalidDeployment.class,()->registry.deploy(q));assertEmpty();}
 }
 @Test void unsafeOrUnboundXmlIsRejectedBeforeAnyDeployment(){
  String valid=xml(VERSION);
  var cases=List.of("",valid.replace("p_"+VERSION.replace("-",""),"p_unbound"),
   "<!DOCTYPE definitions [<!ENTITY xxe SYSTEM \"file:///weaveos-test-nonexistent-sentinel\">]>"+valid,
   valid.replace("<startEvent","<scriptTask").replace("</startEvent>","</scriptTask>"),
   valid.replace("isExecutable=\"true\"","isExecutable=\"true\" flowable:async=\"true\""),
   valid.replace("</sequenceFlow>","<conditionExpression xsi:type=\"bpmn:tFormalExpression\">\u0024{runtime.exec('forbidden')}</conditionExpression></sequenceFlow>"),
   valid.replace("http://www.omg.org/spec/BPMN/20100524/MODEL","urn:invalid"),
   valid+" ".repeat(1024*1024));
  for(var value:cases){assertThrows(InvalidDeployment.class,()->registry.deploy(new Request(APP,FLOW,VERSION,1,value)));assertEmpty();}
 }
 @Test void concurrentSameRequestCommitsOneDeployment()throws Exception{
  var executor=Executors.newFixedThreadPool(2);var ready=new CountDownLatch(2);var start=new CountDownLatch(1);
  try{var a=executor.submit(()->{ready.countDown();start.await();return registry.deploy(request());});
   var b=executor.submit(()->{ready.countDown();start.await();return registry.deploy(request());});
   assertTrue(ready.await(10,TimeUnit.SECONDS));start.countDown();assertEquals(a.get(20,TimeUnit.SECONDS),b.get(20,TimeUnit.SECONDS));
   assertEquals(1,count());assertEquals(1,deployments());
  }finally{start.countDown();executor.shutdownNow();assertTrue(executor.awaitTermination(10,TimeUnit.SECONDS));}
 }
 @Test void differentApplicationMayReuseFlowIdAndVersionNumber()throws Exception{
  var first=registry.deploy(request());var id="30000000-0000-4000-8000-000000000002";
  var q=new Request("10000000-0000-4000-8000-000000000002",FLOW,id,1,xml(id));var second=registry.deploy(q);verify(second,q);
  assertNotEquals(first.engineDeploymentId(),second.engineDeploymentId());assertEquals(2,count());assertEquals(2,deployments());
 }
 static final class InjectedFailure extends RuntimeException{}
}
