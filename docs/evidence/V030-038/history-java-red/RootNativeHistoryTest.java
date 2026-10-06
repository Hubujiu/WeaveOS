package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import static org.weaveos.workflow.RootExecutionRegistryTest.*;
import java.util.List;
import java.util.Set;
import org.flowable.common.engine.impl.history.HistoryLevel;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.workflow.ExecutionRegistry.*;

/** Native history is authoritative; the legacy table here is synthetic compatibility input. */
class RootNativeHistoryTest {
 final RootExecutionRegistryTest f = new RootExecutionRegistryTest();
 @BeforeEach void setup() throws Exception {
  f.setup();
  f.jdbc.execute("""
   CREATE TABLE IF NOT EXISTS wf_execution_visits (
    instance_id uuid NOT NULL REFERENCES wf_execution_instances(instance_id),
    node_id uuid NOT NULL, activation_epoch bigint NOT NULL,
    PRIMARY KEY(instance_id,node_id,activation_epoch), UNIQUE(instance_id,activation_epoch))
   """);
  f.deploy("all");
 }
 @AfterEach void cleanup() { f.cleanup(); }
 long history(String process,int node) {
  String key="n_"+id(node).replace("-","");
  long activities=f.engine.getHistoryService().createHistoricActivityInstanceQuery()
   .processInstanceId(process).activityId(key).activityType("userTask").count();
  long tasks=f.engine.getHistoryService().createHistoricTaskInstanceQuery()
   .processInstanceId(process).taskDefinitionKey(key).count();
  // MI containers may also have native activity records; exact cardinality is
  // defined by actual assignee tasks, while visitation is activity existence.
  assertEquals(tasks>0,activities>0);return tasks;
 }
 void clearShadow() { f.jdbc.update("DELETE FROM wf_execution_visits"); }
 Command back(Command c,Receipt prior,int actor,int target) {
  var q=action(c,"return",prior,actor(prior,actor),id(actor));q.strings[11]=id(target);return q;
 }
 @Test void startWritesNativeHistorySynchronouslyWithoutShadowVisits() {
  var c=new Command();
  new TransactionTemplate(f.tm).executeWithoutResult(tx->{
   var r=f.registry.execute(c.request());assertEquals("success",r.outcome());
   assertEquals(2,history(r.result().engineProcessId(),2));assertEquals(0,f.count("wf_execution_visits"));
  });
 }
 @Test void visitedReturnWorksWithoutLegacyRows() {
  var c=new Command();var first=f.registry.execute(c.request());var second=f.second(c,first);clearShadow();
  var q=back(c,second,10,2);var returned=f.registry.execute(q.request());f.verify(q,returned,"success");
  f.active(returned,2,Set.of(id(8),id(9)),3);assertEquals(4,history(returned.result().engineProcessId(),2));
  assertEquals(0,f.count("wf_execution_visits"));
 }
 @Test void forgedLegacyVisitCannotAuthorizeUnvisitedTarget() {
  var c=new Command();var first=f.registry.execute(c.request());
  assertEquals(0,history(first.result().engineProcessId(),3));
  f.jdbc.update("INSERT INTO wf_execution_visits(instance_id,node_id,activation_epoch) VALUES(?::uuid,?::uuid,99)",c.instanceId(),id(3));
  f.noEffect(back(c,first,8,3),"return_target_unvisited");f.active(first,2,Set.of(id(8),id(9)),1);
 }
 @Test void reopenedEngineUsesNativeHistoryAndPreservesOldReceipt() {
  var c=new Command();var first=f.registry.execute(c.request());var second=f.second(c,first);clearShadow();
  f.engine.close();f.engine=null;f.open();
  assertEquals(first,f.registry.execute(c.request()));
  var q=back(c,second,10,2);var returned=f.registry.execute(q.request());f.verify(q,returned,"success");
  f.active(returned,2,Set.of(id(8),id(9)),3);assertEquals(4,history(returned.result().engineProcessId(),2));
 }
 @Test void repeatedReturnsKeepDistinctActivationsAndAppendHistory() {
  var c=new Command();var first=f.registry.execute(c.request());var current=f.second(c,first);
  for(int round=0;round<3;round++) {
   clearShadow();var q=back(c,current,10,2);var returned=f.registry.execute(q.request());f.verify(q,returned,"success");
   f.active(returned,2,Set.of(id(8),id(9)),3+2L*round);
   assertEquals(4+2L*round,history(returned.result().engineProcessId(),2));
   assertEquals(first,f.registry.execute(c.request()));current=f.second(c,returned);
  }
  assertEquals(0,f.count("wf_execution_visits"));
 }
 @Test void failedReturnRollsBackNativeHistoryAndExactRuntimeTasks() {
  var c=new Command();var first=f.registry.execute(c.request());var second=f.second(c,first);clearShadow();
  String process=second.result().engineProcessId();long before=history(process,2);
  List<String> tasks=f.engine.getTaskService().createTaskQuery().processInstanceId(process).list().stream().map(t->t.getId()).sorted().toList();
  var q=back(c,second,10,2);
  assertThrows(Injected.class,()->f.registry.execute(q.request(),s->{if(s==Stage.AFTER_ENGINE)throw new Injected();}));
  assertEquals(before,history(process,2));
  assertEquals(tasks,f.engine.getTaskService().createTaskQuery().processInstanceId(process).list().stream().map(t->t.getId()).sorted().toList());
  assertTrue(f.registry.lookup(q.commandId(),hex(hash(q.bytes()))).isEmpty());
  var retried=f.registry.execute(q.request());f.verify(q,retried,"success");
 }
 @Test void missingOrAsyncHistoryConfigurationCannotEnableBridge() {
  var cfg=(SpringProcessEngineConfiguration)f.engine.getProcessEngineConfiguration();
  var old=cfg.getHistoryLevel();
  try {
   cfg.setHistoryLevel(HistoryLevel.NONE);
   assertThrows(IllegalArgumentException.class,()->new ExecutionRegistry(f.jdbc,f.tm,f.engine));
  } finally { cfg.setHistoryLevel(old); }
  boolean async=cfg.isAsyncHistoryEnabled();
  try {
   cfg.setAsyncHistoryEnabled(true);
   assertThrows(IllegalArgumentException.class,()->new ExecutionRegistry(f.jdbc,f.tm,f.engine));
  } finally { cfg.setAsyncHistoryEnabled(async); }
 }
 @Test void anotherInstanceHistoryCannotAuthorizeCurrentReturn() {
  var a=new Command();var b=new Command();b.strings[4]=id(199);
  var ar=f.registry.execute(a.request());var br=f.registry.execute(b.request());
  f.second(b,br);clearShadow();
  assertEquals(0,history(ar.result().engineProcessId(),3));assertEquals(3,history(br.result().engineProcessId(),3));
  f.noEffect(back(a,ar,8,3),"return_target_unvisited");
  f.active(ar,2,Set.of(id(8),id(9)),1);
 }
 static final class Injected extends RuntimeException {}
}
