package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.util.*;
import javax.sql.DataSource;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;

class RootFlowDeletionRuntimeSchemaTest {
 final RootFlowDeletionSchemaTest f=new RootFlowDeletionSchemaTest();String role;DataSource runtime;
 @BeforeEach void setup()throws Exception{
  f.setup();role="v067_runtime_"+UUID.randomUUID().toString().replace("-","");
  f.admin.execute("CREATE ROLE "+role+" LOGIN PASSWORD 'v067_synthetic_only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT");
  f.admin.execute("REVOKE TEMPORARY ON DATABASE "+(f.url.contains("weaveos_v067_engine_test")?"weaveos_v067_engine_test":"b3_flowable_fixture")+" FROM PUBLIC");
  f.admin.execute("GRANT USAGE ON SCHEMA "+f.schema+" TO "+role);
  for(String table:f.jdbc.queryForList("SELECT tablename FROM pg_tables WHERE schemaname=? AND left(tablename,4) IN ('act_','flw_')",String.class,f.schema))f.admin.execute("GRANT SELECT,INSERT,UPDATE,DELETE ON "+f.schema+"."+table+" TO "+role);
  for(String seq:f.jdbc.queryForList("SELECT sequencename FROM pg_sequences WHERE schemaname=?",String.class,f.schema))f.admin.execute("GRANT USAGE,SELECT ON SEQUENCE "+f.schema+"."+seq+" TO "+role);
  for(String table:List.of("wf_deployments","wf_execution_commands","wf_execution_instances","wf_execution_tasks"))f.admin.execute("GRANT SELECT,INSERT ON "+f.schema+"."+table+" TO "+role);
  grant("wf_deployments","status,engine_deployment_id,process_definition_id");grant("wf_execution_commands","outcome,result_sequence,proof_id,result_hash,result_bytes");grant("wf_execution_instances","engine_process_id,state,sequence,fence_epoch,schema_version,record_version,activation_epoch,updated_at");grant("wf_execution_tasks","state,decision");
  runtime=new DriverManagerDataSource(f.url+"?currentSchema="+f.schema,role,"v067_synthetic_only");
 }
 void grant(String table,String columns){f.admin.execute("GRANT UPDATE("+columns+") ON "+f.schema+"."+table+" TO "+role);}
 void upgrade()throws Exception{f.migrate(false);f.present();f.admin.execute("GRANT SELECT,INSERT ON "+f.schema+".wf_flow_deletion_guards TO "+role);grant("wf_flow_deletion_guards","retired,operation_id,deleted_versions,deleted_at");}
 void verify(){RuntimeSchema.verify(runtime,f.schema,5000);}
 @AfterEach void cleanup(){f.cleanup();if(role!=null)f.admin.execute("DROP ROLE "+role);}
 @Test void oldSchemaWithoutPersistentIdentityIsNoLongerReady(){assertThrows(IllegalStateException.class,this::verify,"runtime must not accept pre-guard schema");}
 @Test void explicitlyUpgradedSchemaAndMinimalNewRoleAreReady()throws Exception{upgrade();assertDoesNotThrow(this::verify);}
 @Test void guardIdentityUpdatesAndDeletionAuthorityAreRejected()throws Exception{
  upgrade();assertDoesNotThrow(this::verify);grant("wf_flow_deletion_guards","app_id");assertThrows(IllegalStateException.class,this::verify);
  f.admin.execute("REVOKE UPDATE(app_id) ON "+f.schema+".wf_flow_deletion_guards FROM "+role);assertDoesNotThrow(this::verify);
  f.admin.execute("GRANT DELETE ON "+f.schema+".wf_flow_deletion_guards TO "+role);assertThrows(IllegalStateException.class,this::verify);
 }
 @Test void publicExecuteOnDeletionGuardFunctionIsRejected()throws Exception{
  upgrade();assertDoesNotThrow(this::verify);
  f.admin.execute("GRANT EXECUTE ON FUNCTION "+f.schema+".wf_flow_deletion_guard_immutable() TO PUBLIC");
  assertThrows(IllegalStateException.class,this::verify,"public trigger function execution must not pass readiness");
 }
 @Test void runtimeRoleActuallyTransitionsOnceAndCannotMutateIdentityOrHistory()throws Exception{
  upgrade();var j=new JdbcTemplate(runtime);j.update("INSERT INTO wf_flow_deletion_guards(app_id,flow_id) VALUES(?::uuid,?::uuid)",f.app,f.flow);
  j.update("UPDATE wf_flow_deletion_guards SET retired=true,operation_id=?::uuid,deleted_versions=0,deleted_at=clock_timestamp()",f.op);
  assertThrows(Exception.class,()->j.update("UPDATE wf_flow_deletion_guards SET app_id=?::uuid",f.app));assertThrows(Exception.class,()->j.update("DELETE FROM wf_flow_deletion_guards"));assertThrows(Exception.class,()->j.execute("TRUNCATE wf_flow_deletion_guards"));
  assertThrows(Exception.class,()->j.update("UPDATE wf_flow_deletion_guards SET deleted_versions=1"));assertEquals(1,j.queryForObject("SELECT count(*) FROM wf_flow_deletion_guards WHERE retired AND deleted_versions=0",Long.class));
 }
}
