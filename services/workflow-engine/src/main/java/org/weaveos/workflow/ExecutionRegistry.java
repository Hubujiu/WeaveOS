package org.weaveos.workflow;

import java.util.*;
import java.util.function.Consumer;
import javax.sql.DataSource;
import org.flowable.bpmn.model.UserTask;
import org.flowable.bpmn.model.ExclusiveGateway;
import org.flowable.engine.ProcessEngine;
import org.flowable.common.engine.impl.history.HistoryLevel;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.TransactionAwareDataSourceProxy;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.workflow.ExecutionCodec.Payload;

/** Internal execution port. An enclosing REQUIRED transaction must commit before a receipt is confirmed. */
public final class ExecutionRegistry {
    public record Request(byte[] commandBytes, byte[] payloadBytes) {
        public Request {
            if(commandBytes==null || payloadBytes==null || commandBytes.length>538 || payloadBytes.length>262144)throw new InvalidCommand();
            commandBytes=commandBytes.clone();payloadBytes=payloadBytes.clone();
        }
        @Override public byte[] commandBytes(){return commandBytes.clone();}
        @Override public byte[] payloadBytes(){return payloadBytes.clone();}
    }
    public record Task(String id, String nodeId, String assigneeId, String engineTaskId, long activationEpoch) {}
    public record Result(String instanceId, String engineProcessId, String state, String reason,
                         long schemaVersion, long recordVersion, List<Task> tasks) {
        public Result { tasks=tasks.stream().sorted(Comparator.comparing(Task::id)).toList(); }
    }
    public record Receipt(String commandId, String commandHash, String outcome, long sequence,
                          String proofId, String resultHash, Result result) {}
    public enum Stage { AFTER_LEDGER, AFTER_ENGINE, AFTER_RECEIPT }
    public static final class InvalidCommand extends IllegalArgumentException {
        public InvalidCommand() { super("invalid execution command"); }
    }
    public static final class CommandConflict extends RuntimeException {
        public CommandConflict() { super("execution command identity conflict"); }
    }
    private final JdbcTemplate jdbc;
    private final ProcessEngine engine;
    private final TransactionTemplate transaction;
    public ExecutionRegistry(JdbcTemplate jdbc, PlatformTransactionManager manager, ProcessEngine engine) {
        this.jdbc=Objects.requireNonNull(jdbc);this.engine=Objects.requireNonNull(engine);
        if(!(engine.getProcessEngineConfiguration() instanceof SpringProcessEngineConfiguration c)
            ||c.getTransactionManager()!=manager||!(manager instanceof DataSourceTransactionManager m)
            ||underlying(jdbc.getDataSource())!=underlying(m.getDataSource())
            ||underlying(jdbc.getDataSource())!=underlying(c.getDataSource()))
            throw new IllegalArgumentException("registry and engine require the same Spring JDBC transaction boundary");
        if(c.getHistoryLevel()==null || !c.getHistoryLevel().isAtLeast(HistoryLevel.ACTIVITY) || c.isAsyncHistoryEnabled())
            throw new IllegalArgumentException("execution requires synchronous native activity history");
        transaction=new TransactionTemplate(manager);transaction.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRED);
    }
    private static DataSource underlying(DataSource ds){while(ds instanceof TransactionAwareDataSourceProxy p)ds=p.getTargetDataSource();return Objects.requireNonNull(ds);}
    public Receipt execute(Request request){return execute(request,s->{});}
    public Receipt execute(Request request, Consumer<Stage> probe){return run(request,Objects.requireNonNull(probe),false);}
    public Receipt establishNoEffect(Request request){return run(request,s->{},true);}
    public Optional<Receipt> lookup(String commandId,String commandHash){
        ExecutionCodec.uuid(commandId);
        if(commandHash==null||!commandHash.matches("[0-9a-f]{64}"))throw new InvalidCommand();
        var rows=jdbc.queryForList("SELECT * FROM wf_execution_commands WHERE command_id=?::uuid",commandId);
        if(rows.isEmpty())return Optional.empty();
        var row=rows.get(0);if(!commandHash.equals(text(row,"command_hash")))throw new CommandConflict();
        return Optional.of(receipt(row));
    }
    private Receipt run(Request request,Consumer<Stage> probe,boolean cancelled){
        if(request==null)throw new InvalidCommand();
        CommandEnvelope c;
        try{c=CommandEnvelope.decode(request.commandBytes);}catch(IllegalArgumentException invalid){throw new InvalidCommand();}
        Payload payload=ExecutionCodec.payload(request.payloadBytes,c.action().equals("start"));
        if(!Arrays.equals(c.payloadHash(),ExecutionCodec.hash(request.payloadBytes)))throw new InvalidCommand();
        return transaction.execute(status->{
            int inserted=jdbc.update("""
                INSERT INTO wf_execution_commands(command_id,instance_id,command_hash,command_bytes,payload_hash,payload_bytes)
                VALUES(?::uuid,?::uuid,?,?,?,?) ON CONFLICT DO NOTHING
                """,c.commandId(),c.instanceId(),ExecutionCodec.hex(c.fingerprint()),request.commandBytes,ExecutionCodec.hex(c.payloadHash()),request.payloadBytes);
            if(inserted==0){
                var row=jdbc.queryForMap("SELECT * FROM wf_execution_commands WHERE command_id=?::uuid",c.commandId());
                if(!Arrays.equals(request.commandBytes,(byte[])row.get("command_bytes"))||!Arrays.equals(request.payloadBytes,(byte[])row.get("payload_bytes")))throw new CommandConflict();
                return receipt(row);
            }
            probe.accept(Stage.AFTER_LEDGER);
            // Recovery arbitration depends only on the exact command, never on live business context.
            if(cancelled)return finish(c,"no_effect",unchanged(c,"cancelled"),probe);
            if(c.expectedSequence()==ExecutionCodec.MAX)throw new InvalidCommand();
            var instances=jdbc.queryForList("SELECT * FROM wf_execution_instances WHERE instance_id=?::uuid FOR UPDATE",c.instanceId());
            Map<String,Object> instance=instances.isEmpty()?null:instances.get(0);
            if(c.action().equals("start")){
                if(instance!=null)return finish(c,"no_effect",unchanged(c,"instance_exists"),probe);
                var rows=jdbc.queryForList("SELECT * FROM wf_deployments WHERE version_id=?::uuid AND status='confirmed'",c.versionId());
                return start(c,payload,request.payloadBytes,instance,rows.isEmpty()?null:rows.get(0),probe);
            }
            String reason=check(c,instance);
            if(reason!=null)return finish(c,"no_effect",unchanged(c,reason),probe);
            var deployments=jdbc.queryForList("SELECT * FROM wf_deployments WHERE version_id=?::uuid AND status='confirmed'",text(instance,"version_id"));
            if(deployments.isEmpty())throw new IllegalStateException("pinned deployment missing");
            validateNodes(payload,text(deployments.get(0),"process_definition_id"));
            Map<String,Object> task=null;
            if(!c.action().equals("withdraw")){
                var rows=jdbc.queryForList("SELECT * FROM wf_execution_tasks WHERE task_id=?::uuid AND instance_id=?::uuid",c.taskId(),c.instanceId());
                if(rows.isEmpty())return finish(c,"no_effect",unchanged(c,"task_missing"),probe);
                task=rows.get(0);boolean historical=c.action().equals("return")&&text(task,"state").equals("completed");
                if(!text(task,"state").equals("active")&&!historical)return finish(c,"no_effect",unchanged(c,"task_inactive"),probe);
                if(number(task,"activation_epoch")!=c.taskEpoch())return finish(c,"no_effect",unchanged(c,"task_epoch_mismatch"),probe);
                if(!text(task,"assignee_id").equals(c.actorId()))return finish(c,"no_effect",unchanged(c,"actor_mismatch"),probe);
                if(c.action().equals("return")){
                    boolean visited=!engine.getHistoryService().createHistoricActivityInstanceQuery()
                        .processInstanceId(text(instance,"engine_process_id"))
                        .activityId("n_"+ExecutionCodec.compact(c.targetNodeId())).activityType("userTask")
                        .listPage(0,1).isEmpty();
                    if(!visited)return finish(c,"no_effect",unchanged(c,"return_target_unvisited"),probe);
                    if(historical&&!text(task,"node_id").equals(c.targetNodeId()))return finish(c,"no_effect",unchanged(c,"return_target_forbidden"),probe);
                }
            }else{
                if(!text(instance,"initiator_id").equals(c.actorId()))return finish(c,"no_effect",unchanged(c,"actor_mismatch"),probe);
                if(!Boolean.TRUE.equals(instance.get("allow_withdraw")))return finish(c,"no_effect",unchanged(c,"withdrawal_forbidden"),probe);
            }
            String process=text(instance,"engine_process_id"),endState=null;
            engine.getRuntimeService().setVariables(process,payload.variables(false));
            switch(c.action()){
                case "agree","reject"->{
                    jdbc.update("UPDATE wf_execution_tasks SET state='completed',decision=? WHERE task_id=?::uuid",c.action(),c.taskId());
                    engine.getTaskService().complete(text(task,"engine_task_id"),Map.of("wf_rejected",c.action().equals("reject")));
                    if(c.action().equals("reject"))endState="rejected";
                }
                case "withdraw"->{engine.getRuntimeService().deleteProcessInstance(process,"weaveos-withdraw");endState="withdrawn";}
                case "return"->{
                    var current=engine.getTaskService().createTaskQuery().processInstanceId(process).list();
                    var nodes=current.stream().map(t->t.getTaskDefinitionKey()).distinct().toList();
                    if(nodes.size()!=1)throw new IllegalStateException("controlled process must have one active approval node");
                    engine.getRuntimeService().createChangeActivityStateBuilder().processInstanceId(process)
                        .moveActivityIdTo(nodes.get(0),"n_"+ExecutionCodec.compact(c.targetNodeId())).changeState();
                }
                default->throw new InvalidCommand();
            }
            Result result=synchronize(c,process,number(instance,"activation_epoch"),endState);
            probe.accept(Stage.AFTER_ENGINE);
            return finish(c,"success",result,probe);
        });
    }
    private void validateNodes(Payload p,String definition){
        var process=engine.getRepositoryService().getBpmnModel(definition).getMainProcess();
        Set<String> approval=new HashSet<>(),routes=new HashSet<>();
        for(var e:process.getFlowElements()){
            if(e instanceof UserTask)approval.add(ExecutionCodec.nodeId(e.getId()));
            if(e instanceof ExclusiveGateway&&e.getId().startsWith("n_"))routes.add(ExecutionCodec.nodeId(e.getId()));
        }
        if(approval.size()>100||routes.size()>100||!p.routes().keySet().equals(routes)||(p.start()&&!p.rosters().keySet().equals(approval)))throw new InvalidCommand();
    }
    private String check(CommandEnvelope c,Map<String,Object> row){
        if(row==null)return "instance_missing";
        if(!text(row,"app_id").equals(c.appId())||!text(row,"table_id").equals(c.tableId())||!text(row,"view_id").equals(c.viewId())
            ||!text(row,"record_id").equals(c.recordId())||!text(row,"flow_id").equals(c.flowId())||!text(row,"version_id").equals(c.versionId())
            ||number(row,"definition_version")!=c.definitionVersion())return "scope_mismatch";
        if(!text(row,"state").equals("active"))return "terminal_instance";
        if(number(row,"sequence")!=c.expectedSequence())return "stale_sequence";
        if(number(row,"fence_epoch")>=c.fenceEpoch())return "stale_fence";
        if(number(row,"schema_version")>c.schemaVersion())return "stale_schema_version";
        if(number(row,"record_version")>c.recordVersion())return "stale_record_version";
        return null;
    }
    private Receipt start(CommandEnvelope c,Payload payload,byte[] rawPayload,Map<String,Object> instance,Map<String,Object> deployment,Consumer<Stage> probe){
        if(instance!=null)return finish(c,"no_effect",unchanged(c,"instance_exists"),probe);
        if(deployment==null)return finish(c,"no_effect",unchanged(c,"deployment_missing"),probe);
        if(!text(deployment,"app_id").equals(c.appId())||!text(deployment,"flow_id").equals(c.flowId())||number(deployment,"version")!=c.definitionVersion())return finish(c,"no_effect",unchanged(c,"deployment_mismatch"),probe);
        if(FlowIdentityGate.lock(jdbc,c.appId(),c.flowId()))return finish(c,"no_effect",unchanged(c,"deployment_missing"),probe);
        validateNodes(payload,text(deployment,"process_definition_id"));
        int inserted=jdbc.update("""
            INSERT INTO wf_execution_instances(instance_id,app_id,table_id,view_id,record_id,flow_id,version_id,definition_version,
                initiator_id,start_payload_bytes,allow_withdraw,state,sequence,fence_epoch,schema_version,record_version)
            VALUES(?::uuid,?::uuid,?::uuid,?::uuid,?::uuid,?::uuid,?::uuid,?,?::uuid,?,?,'starting',0,?,?,?) ON CONFLICT DO NOTHING
            """,c.instanceId(),c.appId(),c.tableId(),c.viewId(),c.recordId(),c.flowId(),c.versionId(),c.definitionVersion(),c.actorId(),rawPayload,payload.withdraw(),c.fenceEpoch(),c.schemaVersion(),c.recordVersion());
        if(inserted==0)return finish(c,"no_effect",unchanged(c,"instance_exists"),probe);
        var vars=payload.variables(true);vars.put("wf_rejected",false);
        String process=engine.getRuntimeService().startProcessInstanceById(text(deployment,"process_definition_id"),vars).getId();
        Result result=synchronize(c,process,0,null);probe.accept(Stage.AFTER_ENGINE);
        return finish(c,"success",result,probe);
    }
    private Result synchronize(CommandEnvelope c,String process,long epoch,String terminal){
        var runtime=engine.getTaskService().createTaskQuery().processInstanceId(process).list();
        if(runtime.size()>50)throw new IllegalStateException("too many active tasks");
        var old=jdbc.queryForList("SELECT * FROM wf_execution_tasks WHERE instance_id=?::uuid AND state='active'",c.instanceId());
        Map<String,Map<String,Object>> existing=new HashMap<>();old.forEach(t->existing.put(text(t,"engine_task_id"),t));
        Set<String> live=new HashSet<>();runtime.forEach(t->live.add(t.getId()));
        for(var row:old)if(!live.contains(text(row,"engine_task_id")))jdbc.update("UPDATE wf_execution_tasks SET state='invalidated' WHERE task_id=?::uuid",text(row,"task_id"));
        var fresh=runtime.stream().filter(t->!existing.containsKey(t.getId())).toList();
        if(!fresh.isEmpty()){
            if(epoch==ExecutionCodec.MAX)throw new InvalidCommand();epoch++;
            Set<String> nodes=new HashSet<>();fresh.forEach(t->nodes.add(ExecutionCodec.nodeId(t.getTaskDefinitionKey())));
            if(nodes.size()!=1)throw new IllegalStateException("controlled process must have one new approval activation");
            for(var t:fresh){ExecutionCodec.uuid(t.getAssignee());jdbc.update("""
                INSERT INTO wf_execution_tasks(task_id,instance_id,engine_task_id,node_id,activation_epoch,assignee_id,state)
                VALUES(?::uuid,?::uuid,?,?::uuid,?,?::uuid,'active')
                """,UUID.randomUUID().toString(),c.instanceId(),t.getId(),ExecutionCodec.nodeId(t.getTaskDefinitionKey()),epoch,t.getAssignee());}
        }
        String state;
        if(runtime.isEmpty()){
            if(engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(process).count()!=0)throw new IllegalStateException("unexpected live process without approval tasks");
            state=terminal==null?"completed":terminal;
        }else{
            if(terminal!=null)throw new IllegalStateException("terminal action left runtime tasks");state="active";
        }
        jdbc.update("""
            UPDATE wf_execution_instances SET engine_process_id=?,state=?,sequence=?,fence_epoch=?,schema_version=?,record_version=?,activation_epoch=?,updated_at=clock_timestamp()
            WHERE instance_id=?::uuid
            """,process,state,c.expectedSequence()+1,c.fenceEpoch(),c.schemaVersion(),c.recordVersion(),epoch,c.instanceId());
        List<Task> tasks=jdbc.query("SELECT * FROM wf_execution_tasks WHERE instance_id=?::uuid AND state='active' ORDER BY task_id",(r,i)->new Task(r.getString("task_id"),r.getString("node_id"),r.getString("assignee_id"),r.getString("engine_task_id"),r.getLong("activation_epoch")),c.instanceId());
        return new Result(c.instanceId(),process,state,"",c.schemaVersion(),c.recordVersion(),tasks);
    }
    private Result unchanged(CommandEnvelope c,String reason){return new Result(c.instanceId(),"","unchanged",reason,c.schemaVersion(),c.recordVersion(),List.of());}
    private Receipt finish(CommandEnvelope c,String outcome,Result result,Consumer<Stage> probe){
        byte[] bytes=ExecutionCodec.encode(result);long sequence=c.expectedSequence()+(outcome.equals("success")?1:0);
        Receipt receipt=new Receipt(c.commandId(),ExecutionCodec.hex(c.fingerprint()),outcome,sequence,UUID.randomUUID().toString(),ExecutionCodec.hex(ExecutionCodec.hash(bytes)),result);
        int updated=jdbc.update("""
            UPDATE wf_execution_commands SET outcome=?,result_sequence=?,proof_id=?::uuid,result_hash=?,result_bytes=?
            WHERE command_id=?::uuid AND outcome='pending'
            """,outcome,sequence,receipt.proofId(),receipt.resultHash(),bytes,c.commandId());
        if(updated!=1)throw new IllegalStateException("execution receipt was not recorded");
        probe.accept(Stage.AFTER_RECEIPT);return receipt;
    }
    private Receipt receipt(Map<String,Object> row){
        if(text(row,"outcome").equals("pending"))throw new IllegalStateException("unconfirmed durable command");
        byte[] bytes=(byte[])row.get("result_bytes");
        if(!ExecutionCodec.hex(ExecutionCodec.hash(bytes)).equals(text(row,"result_hash")))throw new IllegalStateException("durable result hash mismatch");
        return new Receipt(text(row,"command_id"),text(row,"command_hash"),text(row,"outcome"),number(row,"result_sequence"),text(row,"proof_id"),text(row,"result_hash"),ExecutionCodec.decodeResult(bytes));
    }
    private static String text(Map<String,Object> row,String key){return Objects.requireNonNull(row.get(key)).toString();}
    private static long number(Map<String,Object> row,String key){return ((Number)row.get(key)).longValue();}
}
