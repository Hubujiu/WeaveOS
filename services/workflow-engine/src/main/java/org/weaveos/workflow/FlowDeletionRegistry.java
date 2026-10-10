package org.weaveos.workflow;

import java.time.Instant;
import java.util.HashSet;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Consumer;
import javax.sql.DataSource;
import org.flowable.engine.ProcessEngine;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.TransactionAwareDataSourceProxy;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;

/** Trusted internal port. Receipt remains provisional until an enclosing REQUIRED transaction commits. */
public final class FlowDeletionRegistry {
 public record Request(String appId, String flowId, String operationId) {}
 public record Receipt(String appId, String flowId, String operationId, long deletedVersions, Instant deletedAt) {}
 public enum Stage { AFTER_CHECK, AFTER_DELETION, AFTER_RECEIPT }
 public static final class Conflict extends RuntimeException { public Conflict(){super("flow deletion identity conflict");} }
 public static final class NotDrained extends RuntimeException { public NotDrained(){super("flow is not drained");} }
 private final JdbcTemplate jdbc;
 private final ProcessEngine engine;
 private final TransactionTemplate transaction;
 private static final RowMapper<Receipt> RECEIPT=(row,index)->new Receipt(row.getString("app_id"),row.getString("flow_id"),
  row.getString("operation_id"),row.getLong("deleted_versions"),row.getTimestamp("deleted_at").toInstant());
 public FlowDeletionRegistry(JdbcTemplate jdbc, PlatformTransactionManager manager, ProcessEngine engine) {
  this.jdbc=Objects.requireNonNull(jdbc);this.engine=Objects.requireNonNull(engine);
  if(!(engine.getProcessEngineConfiguration() instanceof SpringProcessEngineConfiguration c)
   ||c.getTransactionManager()!=manager||!(manager instanceof DataSourceTransactionManager m)
   ||underlying(jdbc.getDataSource())!=underlying(m.getDataSource())
   ||underlying(jdbc.getDataSource())!=underlying(c.getDataSource()))
   throw new IllegalArgumentException("registry and engine require the same Spring JDBC transaction boundary");
  transaction=new TransactionTemplate(manager);transaction.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRED);
 }
 private static DataSource underlying(DataSource ds){while(ds instanceof TransactionAwareDataSourceProxy p)ds=p.getTargetDataSource();return Objects.requireNonNull(ds);}
 private static void validate(Request r){Objects.requireNonNull(r);ControlledBpmn.requireUuid(r.appId());ControlledBpmn.requireUuid(r.flowId());ControlledBpmn.requireUuid(r.operationId());}
 public Receipt delete(Request request) { return delete(request, stage -> {}); }
 public Receipt delete(Request r, Consumer<Stage> probe) {
  validate(r);Objects.requireNonNull(probe);
  try{return transaction.execute(status->{
   if(FlowIdentityGate.lock(jdbc,r.appId(),r.flowId()))return lookup(r).orElseThrow(Conflict::new);
   if(Boolean.TRUE.equals(jdbc.queryForObject("SELECT EXISTS(SELECT 1 FROM wf_flow_deletion_guards WHERE operation_id=?::uuid)",Boolean.class,r.operationId())))throw new Conflict();
   var rows=jdbc.queryForList("SELECT * FROM wf_deployments WHERE app_id=?::uuid AND flow_id=?::uuid ORDER BY version,version_id",r.appId(),r.flowId());
   var repository=engine.getRepositoryService();var definitions=new HashSet<String>();
   for(var row:rows){
    if(!"confirmed".equals(row.get("status")))throw new IllegalStateException("unconfirmed controlled deployment");
    String deployment=(String)row.get("engine_deployment_id"),definition=(String)row.get("process_definition_id");
    var actual=repository.createProcessDefinitionQuery().deploymentId(deployment).list();
    if(actual.size()!=1||!actual.get(0).getId().equals(definition)||!definitions.add(definition))
     throw new IllegalStateException("controlled deletion binding mismatch");
   }
   if(!definitions.isEmpty()&&engine.getRuntimeService().createProcessInstanceQuery().processDefinitionIds(definitions).count()!=0)throw new NotDrained();
   if(Boolean.TRUE.equals(jdbc.queryForObject("SELECT EXISTS(SELECT 1 FROM wf_execution_instances WHERE app_id=?::uuid AND flow_id=?::uuid AND state IN ('starting','active'))",Boolean.class,r.appId(),r.flowId())))throw new NotDrained();
   probe.accept(Stage.AFTER_CHECK);
   for(var row:rows){repository.deleteDeployment((String)row.get("engine_deployment_id"));probe.accept(Stage.AFTER_DELETION);}
   int changed=jdbc.update("UPDATE wf_flow_deletion_guards SET retired=true,operation_id=?::uuid,deleted_versions=?,deleted_at=clock_timestamp() WHERE app_id=?::uuid AND flow_id=?::uuid AND NOT retired",r.operationId(),(long)rows.size(),r.appId(),r.flowId());
   if(changed!=1)throw new IllegalStateException("deletion receipt not recorded");
   var result=lookup(r).orElseThrow(()->new IllegalStateException("deletion receipt missing"));
   probe.accept(Stage.AFTER_RECEIPT);return result;
  });}catch(DuplicateKeyException conflict){throw new Conflict();}
 }
 /** Missing is a read result, never proof that an in-flight deletion failed. */
 public Optional<Receipt> lookup(Request r) {
  validate(r);return jdbc.query("SELECT * FROM wf_flow_deletion_guards WHERE app_id=?::uuid AND flow_id=?::uuid AND operation_id=?::uuid AND retired",RECEIPT,r.appId(),r.flowId(),r.operationId()).stream().findFirst();
 }
}
