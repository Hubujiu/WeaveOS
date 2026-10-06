package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.flowable.common.engine.api.FlowableException;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.weaveos.workflow.DeploymentRegistry.*;

/** Root-owned contract tests: actual isolated PG/Flowable, no simulated lifecycle. */
class RootNativeVersioningTest {
 final RootDeploymentRegistryTest f = new RootDeploymentRegistryTest();
 static final String APP=RootDeploymentRegistryTest.APP, FLOW=RootDeploymentRegistryTest.FLOW;
 static final String V1=RootDeploymentRegistryTest.VERSION, V2="30000000-0000-4000-8000-000000000002";
 static final String OTHER_APP="10000000-0000-4000-8000-000000000002";
 static final String OTHER_FLOW="20000000-0000-4000-8000-000000000002";
 static final String KEY="p_10000000000040008000000000000001_20000000000040008000000000000001";
 static final String OTHER_KEY="p_10000000000040008000000000000002_20000000000040008000000000000001";
 @BeforeEach void setup() throws Exception { f.setup(); }
 @AfterEach void cleanup() { f.cleanup(); }
 String xml(String mode,String key) throws Exception {
  try(var in=getClass().getResourceAsStream("/return-compatibility/return-"+mode+".bpmn20.xml")) {
   assertNotNull(in);String original=new String(in.readAllBytes(),StandardCharsets.UTF_8);
   String result=original.replace("p_00000064000040008000000000000064",key);
   assertNotEquals(original,result);return result;
  }
 }
 Request request(String version,long revision,String mode) throws Exception {
  return new Request(APP,FLOW,version,revision,xml(mode,KEY));
 }
 String start(Receipt r) {
  return f.engine.getRuntimeService().startProcessInstanceById(r.processDefinitionId(),Map.of(
   "a_00000002000040008000000000000002",List.of(RootReturnCompatibilityTest.id(8),RootReturnCompatibilityTest.id(9)),
   "a_00000003000040008000000000000003",List.of(RootReturnCompatibilityTest.id(10),RootReturnCompatibilityTest.id(11),RootReturnCompatibilityTest.id(12)),
   "wf_rejected",false)).getId();
 }
 void complete(String process,int expected) {
  var tasks=f.engine.getTaskService().createTaskQuery().processInstanceId(process).list();
  assertEquals(expected,tasks.size());f.engine.getTaskService().complete(tasks.get(0).getId(),Map.of("wf_rejected",false));
 }
 void secondNode(String process) {complete(process,2);complete(process,1);}
 void definition(Receipt r,String key,int version) {
  var d=f.engine.getRepositoryService().createProcessDefinitionQuery().processDefinitionId(r.processDefinitionId()).singleResult();
  assertNotNull(d);assertEquals(key,d.getKey());assertEquals(version,d.getVersion());
 }
 @Test void scopedPublicationsUseNativeVersionsAndKeepRunningDefinition() throws Exception {
  var first=f.registry.deploy(request(V1,1,"all"));definition(first,KEY,1);
  String old=start(first);secondNode(old);
  var second=f.registry.deploy(request(V2,2,"any"));definition(second,KEY,2);
  assertNotEquals(first.processDefinitionId(),second.processDefinitionId());
  assertEquals(first.processDefinitionId(),f.engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(old).singleResult().getProcessDefinitionId());
  String fresh=start(second);secondNode(fresh);complete(fresh,3);
  assertEquals(0,f.engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(fresh).count());
  complete(old,3);complete(old,2);complete(old,1);
  assertEquals(first.processDefinitionId(),f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(old).singleResult().getProcessDefinitionId());
 }
 @Test void sameFlowInDifferentAppsHasIndependentNativeSequence() throws Exception {
  var a=f.registry.deploy(request(V1,1,"all"));
  var b=f.registry.deploy(new Request(OTHER_APP,FLOW,V2,1,xml("all",OTHER_KEY)));
  definition(a,KEY,1);definition(b,OTHER_KEY,1);assertEquals(2,f.deployments());
 }
 @Test void sameScopedPublicationReplaysAcrossEngineReopen() throws Exception {
  var q=request(V1,1,"all");var first=f.registry.deploy(q);
  f.engine.close();f.engine=null;f.open();
  assertEquals(first,f.registry.deploy(q));assertEquals(first,f.registry.lookup(V1).orElseThrow());
  assertEquals(1,f.count());assertEquals(1,f.deployments());definition(first,KEY,1);
 }
 @Test void scopedKeyMustMatchAppAndFlowAndRejectUnsafeXml() throws Exception {
  String valid=xml("all",KEY);
  for(var q:List.of(new Request(OTHER_APP,FLOW,V1,1,valid),new Request(APP,OTHER_FLOW,V1,1,valid),
   new Request(APP,FLOW,V1,1,valid.replace("isExecutable=\"true\"","isExecutable=\"true\" flowable:async=\"true\"")))) {
   assertThrows(InvalidDeployment.class,()->f.registry.deploy(q));f.assertEmpty();
  }
 }
 @Test void legacyToScopedKeepsOriginalDefinitionAndBusinessRevision() throws Exception {
  var legacy=new Request(APP,FLOW,V1,7,xml("all","p_"+V1.replace("-","")));
  var first=f.registry.deploy(legacy);String old=start(first);secondNode(old);
  var second=f.registry.deploy(request(V2,12,"any"));definition(first,"p_"+V1.replace("-",""),1);definition(second,KEY,1);
  assertEquals(7,first.version());assertEquals(12,second.version());assertEquals(first,f.registry.deploy(legacy));
  complete(old,3);complete(old,2);complete(old,1);
  assertEquals(first.processDefinitionId(),f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(old).singleResult().getProcessDefinitionId());
 }
 @Test void suspendingDefinitionLeavesInstancesRunnableAndHistoryReadable() throws Exception {
  var r=f.registry.deploy(request(V1,1,"all"));String process=start(r);
  f.engine.getRepositoryService().suspendProcessDefinitionById(r.processDefinitionId(),false,null);
  assertThrows(FlowableException.class,()->start(r));
  assertFalse(f.engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(process).singleResult().isSuspended());
  secondNode(process);complete(process,3);complete(process,2);complete(process,1);
  assertEquals(1,f.engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(process).finished().count());
  assertNotNull(f.engine.getRepositoryService().getBpmnModel(r.processDefinitionId()));
 }
 @Test void concurrentScopedReplayCommitsOneDefinition() throws Exception {
  var q=request(V1,1,"all");var executor=Executors.newFixedThreadPool(2);
  var ready=new CountDownLatch(2);var go=new CountDownLatch(1);
  try {
   var a=executor.submit(()->{ready.countDown();go.await();return f.registry.deploy(q);});
   var b=executor.submit(()->{ready.countDown();go.await();return f.registry.deploy(q);});
   assertTrue(ready.await(10,TimeUnit.SECONDS));go.countDown();
   var result=a.get(30,TimeUnit.SECONDS);assertEquals(result,b.get(30,TimeUnit.SECONDS));definition(result,KEY,1);
   assertEquals(1,f.count());assertEquals(1,f.deployments());
  } finally {go.countDown();executor.shutdownNow();assertTrue(executor.awaitTermination(10,TimeUnit.SECONDS));}
 }
 @Test void scopedFailureRollsBackLedgerAndNativeVersion() throws Exception {
  var q=request(V1,1,"all");
  assertThrows(Injected.class,()->f.registry.deploy(q,s->{if(s==Stage.AFTER_ENGINE)throw new Injected();}));
  f.assertEmpty();definition(f.registry.deploy(q),KEY,1);assertEquals(1,f.count());
 }
 static final class Injected extends RuntimeException {}
}
