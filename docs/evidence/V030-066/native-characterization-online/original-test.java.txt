package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.util.UUID;
import org.flowable.common.engine.api.FlowableException;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.workflow.DeploymentRegistry.*;

/** Characterization of native cleanup, not an implementation of product deletion. */
class RootNativeDeletionContractTest {
 final RootNativeVersioningTest n = new RootNativeVersioningTest();
 final RootDeploymentRegistryTest f = n.f;
 @BeforeEach void setup() throws Exception {
  if ("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture".equals(System.getenv("B3_TEST_JDBC_URL"))) {
   f.setup(); return;
  }
  f.url=System.getenv("WEAVEOS_V066_TEST_JDBC_URL");
  assertEquals("jdbc:postgresql://127.0.0.1:55445/weaveos_v066_engine_test",f.url,
   "Only this dedicated synthetic local database or the original isolated CI fixture is permitted");
  f.admin=new JdbcTemplate(new DriverManagerDataSource(f.url,"b3_fixture","b3_fixture_only"));
  f.schema="v066_"+UUID.randomUUID().toString().replace("-","");
  f.admin.execute("CREATE SCHEMA "+f.schema);f.open();
  try(var in=getClass().getResourceAsStream("/deployment-registry-fixture.sql")){
   assertNotNull(in);f.jdbc.execute(new String(in.readAllBytes(),StandardCharsets.UTF_8));
  }
 }
 @AfterEach void cleanup(){f.cleanup();}
 String history(){return f.jdbc.queryForObject("SELECT coalesce(jsonb_agg(to_jsonb(d) ORDER BY version_id),'[]'::jsonb)::text FROM wf_deployments d",String.class);}
 void finish(String process){n.secondNode(process);n.complete(process,3);n.complete(process,2);n.complete(process,1);assertEquals(0,f.engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(process).count());}
 void present(Receipt r){assertNotNull(f.engine.getRepositoryService().createProcessDefinitionQuery().processDefinitionId(r.processDefinitionId()).singleResult());assertNotNull(f.engine.getRepositoryService().getBpmnModel(r.processDefinitionId()));assertFalse(f.engine.getRepositoryService().getDeploymentResourceNames(r.engineDeploymentId()).isEmpty());}
 void absent(Receipt r){assertEquals(0,f.engine.getRepositoryService().createDeploymentQuery().deploymentId(r.engineDeploymentId()).count());assertEquals(0,f.engine.getRepositoryService().createProcessDefinitionQuery().processDefinitionId(r.processDefinitionId()).count());assertEquals(0,f.jdbc.queryForObject("SELECT count(*) FROM ACT_GE_BYTEARRAY WHERE DEPLOYMENT_ID_=?",Long.class,r.engineDeploymentId()));}
 @Test void nonCascadeRefusesRunningInstanceAndTaskCanStillContinue() throws Exception {
  var r=f.registry.deploy(n.request(n.V1,1,"all"));String process=n.start(r),before=history();
  assertThrows(FlowableException.class,()->f.engine.getRepositoryService().deleteDeployment(r.engineDeploymentId()));
  present(r);assertEquals(2,f.engine.getTaskService().createTaskQuery().processInstanceId(process).count());
  finish(process);assertEquals(before,history());
 }
 @Test void nonCascadeRefusesFinishedNativeHistoryInsteadOfSilentlyDroppingIt() throws Exception {
  var r=f.registry.deploy(n.request(n.V1,1,"all"));String process=n.start(r);finish(process);
  assertEquals(1,f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(process).finished().count());
  assertThrows(FlowableException.class,()->f.engine.getRepositoryService().deleteDeployment(r.engineDeploymentId()));
  present(r);assertEquals(1,f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(process).count());
 }
 @Test void drainedCascadeRemovesOnlyTargetResourcesAndPreservesIndependentReceipts() throws Exception {
  var target=f.registry.deploy(n.request(n.V1,1,"all"));String ended=n.start(target);finish(ended);
  var sibling=f.registry.deploy(n.request(n.V2,2,"all"));String running=n.start(sibling);
  var other=f.registry.deploy(new Request(n.OTHER_APP,n.FLOW,"30000000-0000-4000-8000-000000000003",1,n.xml("all",n.OTHER_KEY)));
  String otherRunning=n.start(other),before=history();
  f.engine.getRepositoryService().deleteDeployment(target.engineDeploymentId(),true);
  absent(target);assertEquals(0,f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(ended).count());
  present(sibling);present(other);assertEquals(2,f.engine.getRuntimeService().createProcessInstanceQuery().count());
  assertEquals(2,f.engine.getTaskService().createTaskQuery().processInstanceId(running).count());
  assertEquals(2,f.engine.getTaskService().createTaskQuery().processInstanceId(otherRunning).count());
  assertEquals(before,history());assertEquals(target,f.registry.lookup(n.V1).orElseThrow());
 }
 @Test void nativeCascadeRollbackRestoresDefinitionResourcesAndHistory() throws Exception {
  var r=f.registry.deploy(n.request(n.V1,1,"all"));String process=n.start(r);finish(process);String before=history();
  assertThrows(Injected.class,()->new TransactionTemplate(f.tm).executeWithoutResult(tx->{
   f.engine.getRepositoryService().deleteDeployment(r.engineDeploymentId(),true);absent(r);throw new Injected();
  }));
  present(r);assertEquals(1,f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(process).finished().count());
  assertEquals(before,history());
 }
 @Test void deletingAllNativeVersionsKeepsOriginalReplayAcrossEngineReopen() throws Exception {
  var q1=n.request(n.V1,1,"all");var q2=n.request(n.V2,2,"all");
  var a=f.registry.deploy(q1);var b=f.registry.deploy(q2);String p1=n.start(a),p2=n.start(b);finish(p1);finish(p2);String before=history();
  f.engine.getRepositoryService().deleteDeployment(a.engineDeploymentId(),true);f.engine.getRepositoryService().deleteDeployment(b.engineDeploymentId(),true);
  f.engine.close();f.engine=null;f.open();
  assertEquals(a,f.registry.deploy(q1));assertEquals(b,f.registry.deploy(q2));assertEquals(before,history());absent(a);absent(b);assertEquals(0,f.deployments());
 }
 @Test void nativeCleanupAloneDoesNotReserveDeletedFlowIdentity() throws Exception {
  var old=f.registry.deploy(n.request(n.V1,1,"all"));f.engine.getRepositoryService().deleteDeployment(old.engineDeploymentId(),true);
  var fresh=f.registry.deploy(n.request(n.V2,2,"all"));present(fresh);absent(old);
  assertEquals(old,f.registry.lookup(n.V1).orElseThrow());
  // This documents a gap for the future controlled deletion contract, not approval
  // to permit resurrection after product deletion.
  assertEquals(1,f.deployments());assertEquals(2,f.count());
 }
 static final class Injected extends RuntimeException {}
}
