package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.io.ByteArrayOutputStream;
import java.io.DataOutputStream;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.*;
import java.util.concurrent.*;
import javax.sql.DataSource;
import org.flowable.engine.ProcessEngine;
import org.flowable.engine.ProcessEngineConfiguration;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.junit.jupiter.api.*;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.workflow.ExecutionRegistry.*;

/** Root-owned behavior contract. All database records and actors are synthetic. */
class RootExecutionRegistryTest {
 static String id(int n){return String.format("%08x-0000-4000-8000-%012x",n,n);}
 static final String APP=id(101),FLOW=id(102),TABLE=id(103),VIEW=id(104),RECORD=id(105),VERSION=id(100),INITIATOR=id(106);
 static final long MAX=9007199254740991L;
 JdbcTemplate admin,jdbc; DataSource ds; DataSourceTransactionManager tm;
 ProcessEngine engine; DeploymentRegistry deployments; ExecutionRegistry registry;
 String schema,url;
 @BeforeEach void setup() throws Exception {
  url=System.getenv("B3_TEST_JDBC_URL");
  assertEquals("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture",url);
  admin=new JdbcTemplate(new DriverManagerDataSource(url,"b3_fixture","b3_fixture_only"));
  schema="v030_exec_"+UUID.randomUUID().toString().replace("-","");
  admin.execute("CREATE SCHEMA "+schema);open();
  for(String resource:List.of("/deployment-registry-fixture.sql","/execution-registry-fixture.sql")){
   try(var in=getClass().getResourceAsStream(resource)){
    assertNotNull(in);jdbc.execute(new String(in.readAllBytes(),StandardCharsets.UTF_8));
   }
  }
 }
 void open(){
  ds=new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only");
  tm=new DataSourceTransactionManager(ds);jdbc=new JdbcTemplate(ds);
  var c=new SpringProcessEngineConfiguration();c.setDataSource(ds);c.setTransactionManager(tm);
  c.setDatabaseSchema(schema);c.setDatabaseSchemaUpdate(ProcessEngineConfiguration.DB_SCHEMA_UPDATE_TRUE);
  c.setAsyncExecutorActivate(false);c.setDisableIdmEngine(true);c.setDisableEventRegistry(true);
  engine=c.buildProcessEngine();deployments=new DeploymentRegistry(jdbc,tm,engine);
  registry=new ExecutionRegistry(jdbc,tm,engine);
 }
 @AfterEach void cleanup(){
  if(engine!=null)engine.close();
  if(admin!=null&&schema!=null)admin.execute("DROP SCHEMA "+schema+" CASCADE");
 }
 void deploy(String mode) throws Exception {
  try(var in=getClass().getResourceAsStream("/return-compatibility/return-"+mode+".bpmn20.xml")){
   assertNotNull(in);deployments.deploy(new DeploymentRegistry.Request(APP,FLOW,VERSION,1,new String(in.readAllBytes(),StandardCharsets.UTF_8)));
  }
 }
 static byte[] hash(byte[] b){
  try{return MessageDigest.getInstance("SHA-256").digest(b);}catch(Exception e){throw new AssertionError(e);}
 }
 static String hex(byte[] b){return HexFormat.of().formatHex(b);}
 static void text(DataOutputStream out,String s)throws Exception{
  byte[] b=s.getBytes(StandardCharsets.UTF_8);out.writeInt(b.length);out.write(b);
 }
 static byte[] payload(boolean start,boolean withdraw,Map<String,List<String>> roster,Map<String,Boolean> routes){
  try{
   var buffer=new ByteArrayOutputStream();var out=new DataOutputStream(buffer);
   out.write(new byte[]{87,86,70,80,65,89,0,1});out.write(hash("root-audit-evidence".getBytes(StandardCharsets.UTF_8)));
   out.writeBoolean(start);
   if(start){
    out.writeBoolean(withdraw);out.writeInt(roster.size());
    for(var entry:new TreeMap<>(roster).entrySet()){
     text(out,entry.getKey());out.writeInt(entry.getValue().size());
     for(String actor:entry.getValue())text(out,actor);
    }
   }
   out.writeInt(routes.size());
   for(var entry:new TreeMap<>(routes).entrySet()){text(out,entry.getKey());out.writeBoolean(entry.getValue());}
   return buffer.toByteArray();
  }catch(Exception e){throw new AssertionError(e);}
 }
 static byte[] startPayload(boolean allow){
  return payload(true,allow,Map.of(id(2),List.of(id(8),id(9)),id(3),List.of(id(10),id(11),id(12))),Map.of());
 }
 static byte[] actionPayload(){return payload(false,false,Map.of(),Map.of());}
 static class Command {
  String[] strings={UUID.randomUUID().toString(),APP,TABLE,VIEW,RECORD,FLOW,VERSION,UUID.randomUUID().toString(),"",INITIATOR,"start",""};
  long[] numbers={1,1,1,1,0,0};
  byte[] payload=startPayload(true);
  byte[] bytes(){
   try{
    var buffer=new ByteArrayOutputStream();var out=new DataOutputStream(buffer);
    out.write(new byte[]{87,86,70,67,77,68,0,2});
    for(String s:strings)text(out,s);
    for(long n:numbers)out.writeLong(n);
    out.write(hash(payload));return buffer.toByteArray();
   }catch(Exception e){throw new AssertionError(e);}
  }
  Request request(){return new Request(bytes(),payload);}
  String commandId(){return strings[0];}
  String instanceId(){return strings[7];}
  Command copy(){
   var c=new Command();c.strings=strings.clone();c.numbers=numbers.clone();c.payload=payload.clone();return c;
  }
 }
 static Command action(Command initial,String kind,Receipt prior,Task task,String actor){
  var c=initial.copy();c.strings[0]=UUID.randomUUID().toString();c.strings[10]=kind;
  c.strings[8]=task==null?"":task.id();c.strings[9]=actor;c.strings[11]="";
  c.numbers[3]=prior.sequence()+1;c.numbers[4]=task==null?0:task.activationEpoch();c.numbers[5]=prior.sequence();
  c.numbers[1]=prior.result().schemaVersion();c.numbers[2]=prior.result().recordVersion();
  c.payload=actionPayload();return c;
 }
 static Task actor(Receipt r,int n){
  return r.result().tasks().stream().filter(t->t.assigneeId().equals(id(n))).findFirst().orElseThrow();
 }
 static byte[] resultBytes(Result r){
  try{
   var buffer=new ByteArrayOutputStream();var out=new DataOutputStream(buffer);
   out.write(new byte[]{87,86,70,82,83,76,0,1});
   text(out,r.instanceId());text(out,r.engineProcessId());text(out,r.state());text(out,r.reason());
   out.writeLong(r.schemaVersion());out.writeLong(r.recordVersion());out.writeInt(r.tasks().size());
   for(var t:r.tasks().stream().sorted(Comparator.comparing(Task::id)).toList()){
    text(out,t.id());text(out,t.nodeId());text(out,t.assigneeId());text(out,t.engineTaskId());out.writeLong(t.activationEpoch());
   }
   return buffer.toByteArray();
  }catch(Exception e){throw new AssertionError(e);}
 }
 void verify(Command c,Receipt r,String outcome){
  assertEquals(c.commandId(),r.commandId());assertEquals(hex(hash(c.bytes())),r.commandHash());
  assertEquals(outcome,r.outcome());assertEquals(c.numbers[5]+(outcome.equals("success")?1:0),r.sequence());
  assertEquals(c.instanceId(),r.result().instanceId());
  assertEquals(c.numbers[1],r.result().schemaVersion());assertEquals(c.numbers[2],r.result().recordVersion());
  assertEquals(UUID.fromString(r.proofId()).toString(),r.proofId());assertNotEquals(new UUID(0,0).toString(),r.proofId());
  assertEquals(hex(hash(resultBytes(r.result()))),r.resultHash());
  assertArrayEquals(resultBytes(r.result()),jdbc.queryForObject("SELECT result_bytes FROM wf_execution_commands WHERE command_id=?::uuid",byte[].class,c.commandId()));
  assertEquals(r,registry.lookup(c.commandId(),r.commandHash()).orElseThrow());
  assertEquals(r.result().tasks().stream().map(Task::id).sorted().toList(),r.result().tasks().stream().map(Task::id).toList());
 }
 void noEffect(Command c,String reason){
  var r=registry.execute(c.request());verify(c,r,"no_effect");
  assertEquals("unchanged",r.result().state());assertEquals(reason,r.result().reason());
  assertEquals("",r.result().engineProcessId());assertEquals(List.of(),r.result().tasks());
 }
 void active(Receipt r,int node,Set<String> actors,long epoch){
  assertEquals("active",r.result().state());assertEquals("",r.result().reason());
  assertEquals(actors,r.result().tasks().stream().map(Task::assigneeId).collect(java.util.stream.Collectors.toSet()));
  assertEquals(actors.size(),r.result().tasks().size());
  for(var t:r.result().tasks()){
   assertEquals(id(node),t.nodeId());assertEquals(epoch,t.activationEpoch());
   assertEquals(UUID.fromString(t.id()).toString(),t.id());
   var real=engine.getTaskService().createTaskQuery().taskId(t.engineTaskId()).singleResult();
   assertNotNull(real);assertEquals(t.assigneeId(),real.getAssignee());
   assertEquals(r.result().engineProcessId(),real.getProcessInstanceId());
  }
 }
 long count(String table){return jdbc.queryForObject("SELECT count(*) FROM "+table,Long.class);}
 void emptyExecution(){
  assertEquals(0,count("wf_execution_commands"));assertEquals(0,count("wf_execution_instances"));
  assertEquals(0,count("wf_execution_tasks"));assertEquals(0,count("wf_execution_visits"));
  assertEquals(0,engine.getRuntimeService().createProcessInstanceQuery().count());
  assertEquals(0,engine.getHistoryService().createHistoricProcessInstanceQuery().count());
 }
 Receipt agree(Command initial,Receipt prior,int actor){
  var c=action(initial,"agree",prior,actor(prior,actor),id(actor));var r=registry.execute(c.request());verify(c,r,"success");return r;
 }
 Receipt second(Command c,Receipt first){return agree(c,agree(c,first,8),9);}

 @Test void startCreatesDurableExactBindingsAndFullTaskMapping()throws Exception{
  deploy("all");var c=new Command();var r=registry.execute(c.request());verify(c,r,"success");
  active(r,2,Set.of(id(8),id(9)),1);assertEquals(1,count("wf_execution_instances"));
  assertEquals(2,count("wf_execution_tasks"));assertEquals(1,count("wf_execution_visits"));
  assertEquals(VERSION,jdbc.queryForObject("SELECT version_id::text FROM wf_execution_instances",String.class));
  assertEquals(INITIATOR,jdbc.queryForObject("SELECT initiator_id::text FROM wf_execution_instances",String.class));
  assertArrayEquals(c.payload,jdbc.queryForObject("SELECT start_payload_bytes FROM wf_execution_instances",byte[].class));
 }
 @ParameterizedTest @ValueSource(strings={"all","any"})
 void approvalModesTrackPartialCompletionAndInvalidateCancelledPeers(String mode)throws Exception{
  deploy(mode);var c=new Command();var first=registry.execute(c.request());
  var one=agree(c,first,8);active(one,2,Set.of(id(9)),1);
  var next=agree(c,one,9);active(next,3,Set.of(id(10),id(11),id(12)),2);
  var r=agree(c,next,10);
  if(mode.equals("all")){active(r,3,Set.of(id(11),id(12)),2);r=agree(c,r,11);r=agree(c,r,12);}
  assertEquals("completed",r.result().state());assertTrue(r.result().tasks().isEmpty());
  assertEquals(0,engine.getRuntimeService().createProcessInstanceQuery().count());
  long invalidated=jdbc.queryForObject("SELECT count(*) FROM wf_execution_tasks WHERE state='invalidated'",Long.class);
  assertEquals(mode.equals("any")?2:0,invalidated);
 }
 @Test void rejectionEndsOnlyItsOwnInstance()throws Exception{
  deploy("all");var a=new Command();var b=new Command();b.strings[4]=id(199);
  var ar=registry.execute(a.request());var br=registry.execute(b.request());
  var reject=action(a,"reject",ar,actor(ar,8),id(8));var result=registry.execute(reject.request());verify(reject,result,"success");
  assertEquals("rejected",result.result().state());assertTrue(result.result().tasks().isEmpty());
  active(br,2,Set.of(id(8),id(9)),1);assertEquals(1,engine.getRuntimeService().createProcessInstanceQuery().count());
  assertEquals(1,engine.getHistoryService().createHistoricActivityInstanceQuery()
   .processInstanceId(ar.result().engineProcessId()).activityId("reject_end").finished().count());
 }
 @Test void withdrawalRequiresOriginalInitiatorAndEnabledConfiguration()throws Exception{
  deploy("all");var c=new Command();var r=registry.execute(c.request());
  noEffect(action(c,"withdraw",r,null,id(8)),"actor_mismatch");
  var withdraw=action(c,"withdraw",r,null,INITIATOR);var end=registry.execute(withdraw.request());verify(withdraw,end,"success");
  assertEquals("withdrawn",end.result().state());assertEquals(0,engine.getRuntimeService().createProcessInstanceQuery().count());
  assertNotNull(engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(r.result().engineProcessId()).singleResult().getEndTime());
  var d=new Command();d.payload=startPayload(false);var dr=registry.execute(d.request());
  noEffect(action(d,"withdraw",dr,null,INITIATOR),"withdrawal_forbidden");active(dr,2,Set.of(id(8),id(9)),1);
 }
 @Test void eachSuccessRecordsLatestVersionsWithoutChangingThePinnedDefinition()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  var edit=action(c,"agree",first,actor(first,8),id(8));edit.numbers[1]=2;edit.numbers[2]=7;
  var second=registry.execute(edit.request());verify(edit,second,"success");
  assertEquals(1,first.result().recordVersion());assertEquals(7,second.result().recordVersion());
  var third=agree(c,second,9);assertEquals(7,third.result().recordVersion());assertEquals(2,third.result().schemaVersion());
  assertEquals(VERSION,jdbc.queryForObject("SELECT version_id::text FROM wf_execution_instances",String.class));
  assertEquals(first,registry.lookup(c.commandId(),first.commandHash()).orElseThrow());
 }
 @Test void returnReactivatesVisitedNodeWithNewIdentitiesAndInvalidatesOldTasks()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());var next=second(c,first);
  var back=action(c,"return",next,actor(next,10),id(10));back.strings[11]=id(2);
  var returned=registry.execute(back.request());verify(back,returned,"success");active(returned,2,Set.of(id(8),id(9)),3);
  Set<String> old=new HashSet<>();first.result().tasks().forEach(t->old.add(t.id()));next.result().tasks().forEach(t->old.add(t.id()));
  assertTrue(returned.result().tasks().stream().noneMatch(t->old.contains(t.id())));
  noEffect(action(c,"agree",returned,actor(next,11),id(11)),"task_inactive");
  assertEquals(3,jdbc.queryForObject("SELECT count(*) FROM wf_execution_tasks WHERE state='invalidated'",Long.class));
  var again=second(c,returned);active(again,3,Set.of(id(10),id(11),id(12)),4);
 }
 @Test void historicalApproverMayReturnOnlyToTheirOwnVisitedApprovalNode()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());var next=second(c,first);
  var wrong=action(c,"return",next,actor(first,8),id(8));wrong.strings[11]=id(3);noEffect(wrong,"return_target_forbidden");
  var own=action(c,"return",next,actor(first,8),id(8));own.strings[11]=id(2);
  var r=registry.execute(own.request());verify(own,r,"success");active(r,2,Set.of(id(8),id(9)),3);
 }
 @Test void unvisitedTargetIsRejectedWithoutMovingRuntime()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  var back=action(c,"return",first,actor(first,8),id(8));back.strings[11]=id(3);
  noEffect(back,"return_target_unvisited");active(first,2,Set.of(id(8),id(9)),1);
 }
 @Test void duplicatesReplayOriginalReceiptAfterProgressAndEngineRestart()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());second(c,first);
  engine.close();engine=null;open();
  assertEquals(first,registry.execute(c.request()));assertEquals(first,registry.lookup(c.commandId(),first.commandHash()).orElseThrow());
  assertEquals(1,engine.getRuntimeService().createProcessInstanceQuery().count());
  assertEquals(3,count("wf_execution_commands"));
 }
 @Test void sameCommandIdWithDifferentValidPayloadIsAConflict()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  var changed=c.copy();changed.payload=startPayload(false);
  assertThrows(CommandConflict.class,()->registry.execute(changed.request()));
  assertEquals(first,registry.execute(c.request()));assertEquals(1,count("wf_execution_commands"));
  assertThrows(CommandConflict.class,()->registry.lookup(c.commandId(),"f".repeat(64)));
 }
 @Test void missingLookupNeverCreatesAReceiptOrCancellationProof(){
  var c=new Command();assertTrue(registry.lookup(c.commandId(),hex(hash(c.bytes()))).isEmpty());emptyExecution();
 }
 @Test void cancellationBeforeExecutionPermanentlyBlocksThatExactCommand()throws Exception{
  deploy("all");var c=new Command();var cancelled=registry.establishNoEffect(c.request());verify(c,cancelled,"no_effect");
  assertEquals("cancelled",cancelled.result().reason());assertEquals("unchanged",cancelled.result().state());
  assertEquals(cancelled,registry.execute(c.request()));assertEquals(cancelled,registry.establishNoEffect(c.request()));
  assertEquals(0,count("wf_execution_instances"));assertEquals(0,engine.getHistoryService().createHistoricProcessInstanceQuery().count());
 }
 @Test void cancellationAfterExecutionCanOnlyReplayTheSuccessfulReceipt()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  assertEquals(first,registry.establishNoEffect(c.request()));assertEquals(first,registry.execute(c.request()));
 }
 @Test void concurrentDuplicatesCommitOneEngineInstance()throws Exception{
  deploy("all");var c=new Command();var results=race(()->registry.execute(c.request()),()->registry.execute(c.request()));
  assertEquals(results.get(0),results.get(1));assertEquals(1,count("wf_execution_commands"));assertEquals(1,count("wf_execution_instances"));
  assertEquals(1,engine.getRuntimeService().createProcessInstanceQuery().count());
 }
 @Test void concurrentExecutionAndCancellationHaveOneDurableOutcome()throws Exception{
  deploy("all");var c=new Command();var results=race(()->registry.execute(c.request()),()->registry.establishNoEffect(c.request()));
  assertEquals(results.get(0),results.get(1));var r=results.get(0);verify(c,r,r.outcome());
  long expected=r.outcome().equals("success")?1:0;
  assertEquals(expected,count("wf_execution_instances"));assertEquals(expected,engine.getHistoryService().createHistoricProcessInstanceQuery().count());
 }
 @Test void concurrentDifferentCommandsOnTheSameSequenceApplyOnlyOne()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  var a=action(c,"agree",first,actor(first,8),id(8));var b=action(c,"agree",first,actor(first,9),id(9));
  var r=race(()->registry.execute(a.request()),()->registry.execute(b.request()));
  assertEquals(1,r.stream().filter(x->x.outcome().equals("success")).count());
  assertEquals(1,r.stream().filter(x->x.outcome().equals("no_effect")&&x.result().reason().equals("stale_sequence")).count());
  assertEquals(1,engine.getTaskService().createTaskQuery().count());
 }
 static <T> List<T> race(Callable<T> a,Callable<T> b)throws Exception{
  var pool=Executors.newFixedThreadPool(2);var ready=new CountDownLatch(2);var start=new CountDownLatch(1);
  try{
   var fa=pool.submit(()->{ready.countDown();start.await();return a.call();});
   var fb=pool.submit(()->{ready.countDown();start.await();return b.call();});
   assertTrue(ready.await(10,TimeUnit.SECONDS));start.countDown();return List.of(fa.get(30,TimeUnit.SECONDS),fb.get(30,TimeUnit.SECONDS));
  }finally{start.countDown();pool.shutdownNow();assertTrue(pool.awaitTermination(10,TimeUnit.SECONDS));}
 }
 @ParameterizedTest @EnumSource(Stage.class)
 void everyPrecommitStartFailureRollsBackEngineLedgerAndTasks(Stage point)throws Exception{
  deploy("all");var c=new Command();
  assertThrows(InjectedFailure.class,()->registry.execute(c.request(),s->{if(s==point)throw new InjectedFailure();}));
  emptyExecution();verify(c,registry.execute(c.request()),"success");
 }
 @ParameterizedTest @EnumSource(Stage.class)
 void everyPrecommitApprovalFailureRestoresOriginalTasksAndLedger(Stage point)throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  var a=action(c,"agree",first,actor(first,8),id(8));
  assertThrows(InjectedFailure.class,()->registry.execute(a.request(),s->{if(s==point)throw new InjectedFailure();}));
  assertEquals(1,count("wf_execution_commands"));active(first,2,Set.of(id(8),id(9)),1);
  assertTrue(registry.lookup(a.commandId(),hex(hash(a.bytes()))).isEmpty());
  verify(a,registry.execute(a.request()),"success");
 }
 @Test void outerRequiredRollbackNeverLeavesCommittedEngineOrReceipt()throws Exception{
  deploy("all");var c=new Command();
  new TransactionTemplate(tm).executeWithoutResult(tx->{registry.execute(c.request());tx.setRollbackOnly();});
  emptyExecution();
 }
 @Test void allScopeBindingsAreCheckedBeforeAnyAction()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  for(int field:new int[]{1,2,3,4,5,6}){
   var bad=action(c,"agree",first,actor(first,8),id(8));bad.strings[field]=id(999);
   noEffect(bad,"scope_mismatch");
  }
  var bad=action(c,"agree",first,actor(first,8),id(8));bad.numbers[0]=2;noEffect(bad,"scope_mismatch");
  active(first,2,Set.of(id(8),id(9)),1);
 }
 @Test void taskActorEpochAndCurrentVersionsAreChecked()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  var actor=action(c,"agree",first,actor(first,8),id(9));noEffect(actor,"actor_mismatch");
  var epoch=action(c,"agree",first,actor(first,8),id(8));epoch.numbers[4]++;noEffect(epoch,"task_epoch_mismatch");
  var fence=action(c,"agree",first,actor(first,8),id(8));fence.numbers[3]=1;noEffect(fence,"stale_fence");
  var seq=action(c,"agree",first,actor(first,8),id(8));seq.numbers[5]=0;noEffect(seq,"stale_sequence");
  var update=action(c,"agree",first,actor(first,8),id(8));update.numbers[1]=2;update.numbers[2]=4;
  var current=registry.execute(update.request());
  var staleRecord=action(c,"agree",current,actor(current,9),id(9));staleRecord.numbers[2]=3;noEffect(staleRecord,"stale_record_version");
  var staleSchema=action(c,"agree",current,actor(current,9),id(9));staleSchema.numbers[1]=1;noEffect(staleSchema,"stale_schema_version");
  active(current,2,Set.of(id(9)),1);
 }
 @Test void terminalInstanceRejectsNewCommandsButReplaysOldOnes()throws Exception{
  deploy("any");var c=new Command();var first=registry.execute(c.request());var next=second(c,first);var end=agree(c,next,10);
  noEffect(action(c,"agree",end,actor(next,11),id(11)),"terminal_instance");
  assertEquals(first,registry.execute(c.request()));
 }
 @Test void startIdentityAndDeploymentBindingsAreNotImplicitlyReused()throws Exception{
  deploy("all");var c=new Command();var first=registry.execute(c.request());
  var duplicate=c.copy();duplicate.strings[0]=UUID.randomUUID().toString();noEffect(duplicate,"instance_exists");
  var missing=new Command();missing.strings[6]=id(901);noEffect(missing,"deployment_missing");
  var mismatch=new Command();mismatch.strings[1]=id(902);noEffect(mismatch,"deployment_mismatch");
  active(first,2,Set.of(id(8),id(9)),1);
 }
 @Test void malformedPayloadOrHashMismatchCannotWriteAnything()throws Exception{
  deploy("all");var valid=new Command();byte[] bytes=valid.bytes();byte[] changed=valid.payload.clone();changed[10]^=1;
  assertThrows(InvalidCommand.class,()->registry.execute(new Request(bytes,changed)));
  var cases=new ArrayList<byte[]>();
  cases.add(new byte[0]);cases.add(new byte[262145]);cases.add(Arrays.copyOf(valid.payload,valid.payload.length-1));
  var trailing=Arrays.copyOf(valid.payload,valid.payload.length+1);cases.add(trailing);
  var wrongPrefix=valid.payload.clone();wrongPrefix[0]^=1;cases.add(wrongPrefix);
  var zeroEvidence=valid.payload.clone();Arrays.fill(zeroEvidence,8,40,(byte)0);cases.add(zeroEvidence);
  var badFlag=valid.payload.clone();badFlag[40]=2;cases.add(badFlag);
  var badWithdraw=valid.payload.clone();badWithdraw[41]=2;cases.add(badWithdraw);
  var hugeCount=valid.payload.clone();Arrays.fill(hugeCount,42,46,(byte)0xff);cases.add(hugeCount);
  var malformedUuid=valid.payload.clone();malformedUuid[50]=(byte)0xff;cases.add(malformedUuid);
  var duplicateNode=valid.payload.clone();System.arraycopy(duplicateNode,50,duplicateNode,174,36);cases.add(duplicateNode);
  cases.add(payload(false,false,Map.of(),Map.of()));
  cases.add(payload(true,true,Map.of(id(2),List.of(id(9),id(8)),id(3),List.of(id(10),id(11),id(12))),Map.of()));
  cases.add(payload(true,true,Map.of(id(2),List.of(id(8),id(8)),id(3),List.of(id(10),id(11),id(12))),Map.of()));
  cases.add(payload(true,true,Map.of(id(2),List.of(id(8))),Map.of()));
  cases.add(payload(true,true,Map.of(id(2),List.of(id(8),id(9)),id(3),List.of(id(10),id(11),id(12))),Map.of(id(999),true)));
  for(byte[] p:cases){var bad=new Command();bad.payload=p;assertThrows(InvalidCommand.class,()->registry.execute(bad.request()));emptyExecution();}
 }
 @Test void requestOwnsInputArraysAndReturnedTaskCollectionIsImmutable()throws Exception{
  deploy("all");var c=new Command();byte[] command=c.bytes(),body=c.payload.clone();var request=new Request(command,body);
  Arrays.fill(command,(byte)0);Arrays.fill(body,(byte)0);
  byte[] returnedCommand=request.commandBytes(),returnedPayload=request.payloadBytes();
  Arrays.fill(returnedCommand,(byte)0);Arrays.fill(returnedPayload,(byte)0);
  var r=registry.execute(request);verify(c,r,"success");
  assertThrows(UnsupportedOperationException.class,()->r.result().tasks().clear());
  assertEquals(r,registry.execute(c.request()));
 }

 @Test void conditionsUseTheLatestCommandRoutesRatherThanTheStartValues()throws Exception{
  String version=id(99);
  try(var in=getClass().getResourceAsStream("/compiler/generated-all.bpmn20.xml")){
   assertNotNull(in);deployments.deploy(new DeploymentRegistry.Request(APP,FLOW,version,1,new String(in.readAllBytes(),StandardCharsets.UTF_8)));
  }
  var c=new Command();c.strings[6]=version;
  c.payload=payload(true,true,Map.of(id(2),List.of(id(8),id(9))),Map.of(id(3),true));
  var first=registry.execute(c.request());
  var a=action(c,"agree",first,actor(first,8),id(8));
  a.payload=payload(false,false,Map.of(),Map.of(id(3),true));
  var partial=registry.execute(a.request());
  var b=action(c,"agree",partial,actor(partial,9),id(9));
  b.numbers[2]=2;b.payload=payload(false,false,Map.of(),Map.of(id(3),false));
  var end=registry.execute(b.request());verify(b,end,"success");assertEquals("completed",end.result().state());
  assertEquals(1,engine.getHistoryService().createHistoricActivityInstanceQuery()
   .processInstanceId(end.result().engineProcessId()).activityId("n_"+id(5).replace("-","")).finished().count());
  assertEquals(0,engine.getHistoryService().createHistoricActivityInstanceQuery()
   .processInstanceId(end.result().engineProcessId()).activityId("n_"+id(4).replace("-","")).finished().count());
 }
 @Test void noApprovalGraphCanCommitAnImmediatelyCompletedInstance()throws Exception{
  String xml=RootDeploymentRegistryTest.xml(VERSION);
  deployments.deploy(new DeploymentRegistry.Request(APP,FLOW,VERSION,1,xml));
  var c=new Command();c.payload=payload(true,true,Map.of(),Map.of());
  var end=registry.execute(c.request());verify(c,end,"success");
  assertEquals("completed",end.result().state());assertTrue(end.result().tasks().isEmpty());
  assertEquals(1,count("wf_execution_instances"));assertEquals(0,count("wf_execution_tasks"));
  assertEquals(0,engine.getRuntimeService().createProcessInstanceQuery().count());
  assertEquals(1,engine.getHistoryService().createHistoricProcessInstanceQuery().count());
 }
 @Test void registryRejectsWiringThatCouldCommitEngineAndLedgerSeparately(){
  var other=new DataSourceTransactionManager(ds);
  assertThrows(IllegalArgumentException.class,()->new ExecutionRegistry(jdbc,other,engine));
  var otherSource=new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only");
  assertThrows(IllegalArgumentException.class,()->new ExecutionRegistry(new JdbcTemplate(otherSource),tm,engine));
 }
 static final class InjectedFailure extends RuntimeException{}
}
