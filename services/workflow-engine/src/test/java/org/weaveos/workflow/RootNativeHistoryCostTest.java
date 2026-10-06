package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import static org.weaveos.workflow.RootExecutionRegistryTest.*;
import java.lang.management.ManagementFactory;
import java.util.Arrays;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.weaveos.workflow.ExecutionRegistry.*;

/** Measurements are observations, never a speed threshold or substitute for behavior tests. */
class RootNativeHistoryCostTest {
 final RootNativeHistoryTest fixture=new RootNativeHistoryTest();
 @BeforeEach void setup() throws Exception {fixture.setup();}
 @AfterEach void cleanup(){fixture.cleanup();}
 static long gcCount(){return ManagementFactory.getGarbageCollectorMXBeans().stream().mapToLong(x->Math.max(0,x.getCollectionCount())).sum();}
 long[] sample(Runnable operation){
  for(int i=0;i<20;i++)operation.run();
  long[] values=new long[100];
  for(int i=0;i<values.length;i++){long start=System.nanoTime();operation.run();values[i]=System.nanoTime()-start;}
  Arrays.sort(values);return values;
 }
 void measure(Command initial,Receipt current,int rounds){
  var f=fixture.f;String process=current.result().engineProcessId(),node="n_"+id(2).replace("-","");
  // Reconstruct only the former duplicate index in this isolated comparison schema.
  f.jdbc.update("INSERT INTO wf_execution_visits(instance_id,node_id,activation_epoch) SELECT DISTINCT instance_id,node_id,activation_epoch FROM wf_execution_tasks ON CONFLICT DO NOTHING");
  long activityRows=f.jdbc.queryForObject("SELECT count(*) FROM act_hi_actinst",Long.class);
  long shadowRows=f.count("wf_execution_visits");assertTrue(activityRows>0);assertTrue(shadowRows>0);
  long heapBefore=ManagementFactory.getMemoryMXBean().getHeapMemoryUsage().getUsed(),gcBefore=gcCount();
  long[] old=sample(()->assertTrue(f.jdbc.queryForObject("SELECT count(*) FROM wf_execution_visits WHERE instance_id=?::uuid AND node_id=?::uuid",Long.class,initial.instanceId(),id(2))>0));
  long[] nativeTimes=sample(()->assertEquals(1,f.engine.getHistoryService().createHistoricActivityInstanceQuery()
   .processInstanceId(process).activityId(node).activityType("userTask").listPage(0,1).size()));
  long nativeBytes=f.jdbc.queryForObject("SELECT pg_total_relation_size('act_hi_actinst'::regclass)",Long.class);
  long shadowBytes=f.jdbc.queryForObject("SELECT pg_total_relation_size('wf_execution_visits'::regclass)",Long.class);
  String nativePlan=f.jdbc.queryForObject("EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) SELECT RES.* FROM ACT_HI_ACTINST RES WHERE PROC_INST_ID_=? AND ACT_ID_=? AND ACT_TYPE_=? LIMIT 1",String.class,process,node,"userTask");
  String oldPlan=f.jdbc.queryForObject("EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) SELECT count(*) FROM wf_execution_visits WHERE instance_id=?::uuid AND node_id=?::uuid",String.class,initial.instanceId(),id(2));
  assertNotNull(nativePlan);assertNotNull(oldPlan);assertTrue(nativePlan.contains("Actual Rows"));assertTrue(oldPlan.contains("Actual Rows"));
  System.out.println("NATIVE_HISTORY_COST rounds="+rounds+" samples=100 nativeActivityRows="+activityRows+" oldVisitRows="+shadowRows
   +" oldJdbcMedianNs="+old[49]+" oldJdbcP95Ns="+old[94]+" nativeApiMedianNs="+nativeTimes[49]+" nativeApiP95Ns="+nativeTimes[94]
   +" nativeExistingRelationBytes="+nativeBytes+" duplicateRelationBytes="+shadowBytes
   +" heapUsedBefore="+heapBefore+" heapUsedAfter="+ManagementFactory.getMemoryMXBean().getHeapMemoryUsage().getUsed()+" gcDelta="+(gcCount()-gcBefore));
  System.out.println("NATIVE_HISTORY_PLAN rounds="+rounds+" "+nativePlan.replace('\n',' '));
  System.out.println("OLD_VISIT_PLAN rounds="+rounds+" "+oldPlan.replace('\n',' '));
 }
 @Test void observeNativeAndDuplicateCostsAtTwoActualHistorySizes(){
  var f=fixture.f;var c=new Command();var first=f.registry.execute(c.request());var current=f.second(c,first);measure(c,current,0);
  for(int i=0;i<100;i++){
   var back=fixture.back(c,current,10,2);var returned=f.registry.execute(back.request());assertEquals("success",returned.outcome());
   current=f.second(c,returned);
  }
  assertEquals(202,fixture.history(current.result().engineProcessId(),2));measure(c,current,100);
 }
}
