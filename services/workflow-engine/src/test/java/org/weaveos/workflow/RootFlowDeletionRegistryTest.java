package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.util.UUID;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;

/** R1 product oracle: V067 PRD/ADR read before this test; not native characterization. */
class RootFlowDeletionRegistryTest {
 final RootNativeDeletionContractTest c = new RootNativeDeletionContractTest();
 final RootNativeVersioningTest n = c.n;
 final RootDeploymentRegistryTest f = c.f;
 FlowDeletionRegistry deletion;
 static final String OP="70000000-0000-4000-8000-000000000001";
 FlowDeletionRegistry.Request request(){return new FlowDeletionRegistry.Request(n.APP,n.FLOW,OP);}
 @BeforeEach void setup() throws Exception {
  if("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture".equals(System.getenv("B3_TEST_JDBC_URL"))) f.setup();
  else {
   f.url=System.getenv("WEAVEOS_V067_TEST_JDBC_URL");
   assertEquals("jdbc:postgresql://127.0.0.1:55445/weaveos_v067_engine_test",f.url);
   f.admin=new JdbcTemplate(new DriverManagerDataSource(f.url,"b3_fixture","b3_fixture_only"));
   f.schema="v067_"+UUID.randomUUID().toString().replace("-","");f.admin.execute("CREATE SCHEMA "+f.schema);f.open();
   resource("/deployment-registry-fixture.sql");
  }
  resource("/execution-registry-fixture.sql");
  deletion=new FlowDeletionRegistry(f.jdbc,f.tm,f.engine);
 }
 void resource(String name)throws Exception{try(var in=getClass().getResourceAsStream(name)){assertNotNull(in);f.jdbc.execute(new String(in.readAllBytes(),StandardCharsets.UTF_8));}}
 @AfterEach void cleanup(){f.cleanup();}
 @Test void deletesAllTargetDefinitionsAndResourcesButKeepsHistoryAndOriginalReceipts() throws Exception {
  var a=f.registry.deploy(n.request(n.V1,1,"all"));var b=f.registry.deploy(n.request(n.V2,2,"all"));
  var other=f.registry.deploy(new DeploymentRegistry.Request(n.OTHER_APP,n.FLOW,"30000000-0000-4000-8000-000000000003",1,n.xml("all",n.OTHER_KEY)));
  String p=n.start(a);c.finish(p);String before=c.history();
  long activities=f.engine.getHistoryService().createHistoricActivityInstanceQuery().processInstanceId(p).count();assertTrue(activities>0);
  var result=deletion.delete(request());
  assertNotNull(result,"controlled deletion must return its committed receipt");
  assertEquals(n.APP,result.appId());assertEquals(n.FLOW,result.flowId());assertEquals(OP,result.operationId());assertEquals(2,result.deletedVersions());assertNotNull(result.deletedAt());
  c.absent(a);c.absent(b);c.present(other);assertEquals(before,c.history());
  assertEquals(1,f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(p).finished().count());
  assertEquals(activities,f.engine.getHistoryService().createHistoricActivityInstanceQuery().processInstanceId(p).count());
 }
 @Test void exactReplayAndReadOnlyLookupSurviveEngineReopenWithoutRecreatingDefinitions() throws Exception {
  var q=n.request(n.V1,1,"all");var original=f.registry.deploy(q);var result=deletion.delete(request());assertNotNull(result);
  assertEquals(result,deletion.delete(request()));assertEquals(result,deletion.lookup(request()).orElseThrow());
  f.engine.close();f.engine=null;f.open();deletion=new FlowDeletionRegistry(f.jdbc,f.tm,f.engine);
  assertEquals(result,deletion.lookup(request()).orElseThrow());assertEquals(result,deletion.delete(request()));
  assertEquals(original,f.registry.deploy(q));c.absent(original);assertEquals(1,f.count());
 }
 @Test void changedOperationOrCrossScopeReuseCannotDeleteAnythingElse() throws Exception {
  var a=f.registry.deploy(n.request(n.V1,1,"all"));assertNotNull(deletion.delete(request()));
  assertThrows(FlowDeletionRegistry.Conflict.class,()->deletion.delete(new FlowDeletionRegistry.Request(n.APP,n.FLOW,"70000000-0000-4000-8000-000000000002")));
  assertThrows(FlowDeletionRegistry.Conflict.class,()->deletion.delete(new FlowDeletionRegistry.Request(n.OTHER_APP,n.FLOW,OP)));
  assertTrue(deletion.lookup(new FlowDeletionRegistry.Request(n.OTHER_APP,n.FLOW,OP)).isEmpty());c.absent(a);
 }
 @Test void liveNativeInstanceIsRefusedAndCanStillComplete() throws Exception {
  var a=f.registry.deploy(n.request(n.V1,1,"all"));String p=n.start(a),before=c.history();
  assertThrows(FlowDeletionRegistry.NotDrained.class,()->deletion.delete(request()));
  assertTrue(deletion.lookup(request()).isEmpty());c.present(a);assertEquals(before,c.history());
  c.finish(p);assertNotNull(deletion.delete(request()));c.absent(a);
 }
 @ParameterizedTest @EnumSource(FlowDeletionRegistry.Stage.class)
 void everyFailurePointRollsBackResourcesAndDeletionIdentity(FlowDeletionRegistry.Stage point)throws Exception {
  var a=f.registry.deploy(n.request(n.V1,1,"all"));var b=f.registry.deploy(n.request(n.V2,2,"all"));String before=c.history();
  assertThrows(Injected.class,()->deletion.delete(request(),stage->{if(stage==point)throw new Injected();}));
  c.present(a);c.present(b);assertEquals(before,c.history());assertTrue(deletion.lookup(request()).isEmpty());
  assertEquals(2,deletion.delete(request()).deletedVersions());
 }
 @Test void outerRequiredRollbackDoesNotLeaveSuccessfulDeletion() throws Exception {
  var a=f.registry.deploy(n.request(n.V1,1,"all"));
  new TransactionTemplate(f.tm).executeWithoutResult(tx->{assertNotNull(deletion.delete(request()));c.absent(a);tx.setRollbackOnly();});
  c.present(a);assertTrue(deletion.lookup(request()).isEmpty());assertNotNull(deletion.delete(request()));
 }
 @Test void deletedIdentityBlocksNewVersionButAllowsOriginalPublicationReplay() throws Exception {
  var q=n.request(n.V1,1,"all");var original=f.registry.deploy(q);assertNotNull(deletion.delete(request()));
  assertThrows(DeploymentRegistry.DeploymentConflict.class,()->f.registry.deploy(n.request(n.V2,2,"all")));
  assertEquals(original,f.registry.deploy(q));assertEquals(1,f.count());c.absent(original);assertTrue(f.registry.lookup(n.V2).isEmpty());
 }
 @Test void neverPublishedIdentityCanBeRetiredWithoutForgedDeployment() throws Exception {
  assertTrue(deletion.lookup(request()).isEmpty());assertEquals(0,f.count());var result=deletion.delete(request());assertNotNull(result);assertEquals(0,result.deletedVersions());
  assertEquals(0,f.count());assertEquals(0,f.deployments());assertEquals(result,deletion.delete(request()));
  assertThrows(DeploymentRegistry.DeploymentConflict.class,()->f.registry.deploy(n.request(n.V1,1,"all")));assertEquals(0,f.count());
 }
 RootExecutionRegistryTest.Command startCommand(){
  var cmd=new RootExecutionRegistryTest.Command();cmd.strings[1]=n.APP;cmd.strings[5]=n.FLOW;cmd.strings[6]=n.V1;return cmd;
 }
 @Test void newStartAfterDeletionReturnsOriginalProtocolNoEffectWithoutLoadingMissingBpmn()throws Exception {
  f.registry.deploy(n.request(n.V1,1,"all"));assertNotNull(deletion.delete(request()));
  var execution=new ExecutionRegistry(f.jdbc,f.tm,f.engine);var cmd=startCommand();
  var result=assertDoesNotThrow(()->execution.execute(cmd.request()),"retired deployment must be recognized before BPMN resource lookup");
  assertEquals("no_effect",result.outcome());assertEquals("deployment_missing",result.result().reason());
  assertEquals(result,execution.execute(cmd.request()));assertEquals(0,f.engine.getRuntimeService().createProcessInstanceQuery().count());
  assertEquals(0,f.jdbc.queryForObject("SELECT count(*) FROM wf_execution_instances",Long.class));
 }
 @Test void originalExecutionReceiptsReplayAfterWithdrawalAndDeletionUnchanged()throws Exception {
  f.registry.deploy(n.request(n.V1,1,"all"));var execution=new ExecutionRegistry(f.jdbc,f.tm,f.engine);var cmd=startCommand();
  var started=execution.execute(cmd.request());assertEquals("success",started.outcome());
  var withdraw=RootExecutionRegistryTest.action(cmd,"withdraw",started,null,RootExecutionRegistryTest.INITIATOR);
  var ended=execution.execute(withdraw.request());assertEquals("withdrawn",ended.result().state());
  String before=f.jdbc.queryForObject("SELECT jsonb_agg(to_jsonb(c) ORDER BY command_id)::text FROM wf_execution_commands c",String.class);
  assertNotNull(deletion.delete(request()));assertEquals(started,execution.execute(cmd.request()));assertEquals(ended,execution.execute(withdraw.request()));
  assertEquals(before,f.jdbc.queryForObject("SELECT jsonb_agg(to_jsonb(c) ORDER BY command_id)::text FROM wf_execution_commands c",String.class));
  assertEquals(0,f.deployments());
 }
 static final class Injected extends RuntimeException {}
}
