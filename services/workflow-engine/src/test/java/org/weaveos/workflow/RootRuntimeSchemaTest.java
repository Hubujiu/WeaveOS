package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.lang.reflect.*;
import java.nio.file.*;
import java.sql.*;
import java.util.*;
import javax.sql.DataSource;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.AbstractDataSource;

/** Root-owned startup contract against isolated explicit SQL and independent native reference. */
class RootRuntimeSchemaTest {
 static final String FAILURE="workflow runtime schema is not ready";
 String schema,role,url; JdbcTemplate admin,jdbc; DataSource runtime;
 @BeforeEach void setup() throws Exception {
  url=System.getenv("B3_TEST_JDBC_URL");
  assertEquals("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture",url);
  admin=new JdbcTemplate(new DriverManagerDataSource(url,"b3_fixture","b3_fixture_only"));
  String suffix=UUID.randomUUID().toString().replace("-","");schema="v041_guard_"+suffix;role="v041_role_"+suffix;
  admin.execute("CREATE SCHEMA "+schema);
  jdbc=new JdbcTemplate(new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only"));
  String up=Files.readString(Path.of("schema/migrations/00001_flowable8.sql")).split("-- \\+goose Down",-1)[0];
  jdbc.execute(up);
  jdbc.execute(Files.readString(Path.of("schema/migrations/00002_flow_deletion_guards.sql")).split("-- \\+goose Down",-1)[0]);
  admin.execute("CREATE ROLE "+role+" LOGIN PASSWORD 'v041_synthetic_only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT");
  admin.execute("REVOKE TEMPORARY ON DATABASE b3_flowable_fixture FROM PUBLIC");
  admin.execute("GRANT USAGE ON SCHEMA "+schema+" TO "+role);
  for(String table:jdbc.queryForList("SELECT tablename FROM pg_tables WHERE schemaname=? AND (left(tablename,4) IN ('act_','flw_'))",String.class,schema))
   admin.execute("GRANT SELECT,INSERT,UPDATE,DELETE ON "+schema+"."+table+" TO "+role);
  for(String sequence:jdbc.queryForList("SELECT sequencename FROM pg_sequences WHERE schemaname=? AND left(sequencename,4) IN ('act_','flw_')",String.class,schema))
   admin.execute("GRANT USAGE,SELECT ON SEQUENCE "+schema+"."+sequence+" TO "+role);
  for(String table:List.of("wf_deployments","wf_execution_commands","wf_execution_instances","wf_execution_tasks","wf_flow_deletion_guards"))
   admin.execute("GRANT SELECT,INSERT ON "+schema+"."+table+" TO "+role);
  grantUpdate("wf_deployments","status,engine_deployment_id,process_definition_id");
  grantUpdate("wf_execution_commands","outcome,result_sequence,proof_id,result_hash,result_bytes");
  grantUpdate("wf_execution_instances","engine_process_id,state,sequence,fence_epoch,schema_version,record_version,activation_epoch,updated_at");
  grantUpdate("wf_execution_tasks","state,decision");
  grantUpdate("wf_flow_deletion_guards","retired,operation_id,deleted_versions,deleted_at");
  runtime=new DriverManagerDataSource(url+"?currentSchema="+schema,role,"v041_synthetic_only");
 }
 void grantUpdate(String table,String columns){admin.execute("GRANT UPDATE("+columns+") ON "+schema+"."+table+" TO "+role);}
 @AfterEach void cleanup(){
  if(admin!=null&&schema!=null)admin.execute("DROP SCHEMA IF EXISTS "+schema+" CASCADE");
  if(admin!=null&&role!=null){admin.execute("DROP OWNED BY "+role);admin.execute("DROP ROLE "+role);}
 }
 void verify(){RuntimeSchema.verify(runtime,schema,5000);}
 void rejects(){var e=assertThrows(IllegalStateException.class,this::verify);assertEquals(FAILURE,e.getMessage());assertNull(e.getCause());}
 @Test void migratedSchemaAndRestrictedRoleAreAccepted(){assertDoesNotThrow(this::verify);}
 @Test void currentIdAllocatorAndHistoricalRowsAreNotFixedSnapshots(){
  jdbc.update("UPDATE act_ge_property SET value_='25001' WHERE name_='next.dbid'");
  jdbc.update("INSERT INTO act_hi_procinst(id_,proc_inst_id_,proc_def_id_,start_time_) VALUES ('retained','retained','definition',TIMESTAMP '2026-10-06 00:00:00')");
  assertDoesNotThrow(this::verify);assertEquals("25001",jdbc.queryForObject("SELECT value_ FROM act_ge_property WHERE name_='next.dbid'",String.class));
  assertEquals(1,jdbc.queryForObject("SELECT count(*) FROM act_hi_procinst",Integer.class));
 }
 @Test void missingNativeTableIsRejected(){jdbc.execute("DROP TABLE act_hi_actinst CASCADE");rejects();}
 @Test void missingProtocolTableIsRejected(){jdbc.execute("DROP TABLE wf_execution_tasks CASCADE");rejects();}
 @Test void missingColumnIsRejected(){jdbc.execute("ALTER TABLE act_hi_procinst DROP COLUMN business_key_");rejects();}
 @Test void changedColumnTypeIsRejected(){jdbc.execute("ALTER TABLE wf_deployments ALTER COLUMN bpmn_sha256 TYPE varchar(128)");rejects();}
 @Test void removedNotNullIsRejected(){jdbc.execute("ALTER TABLE wf_deployments ALTER COLUMN app_id DROP NOT NULL");rejects();}
 @Test void missingIndexIsRejected(){jdbc.execute("DROP INDEX ix_wf_execution_tasks_active");rejects();}
 @Test void removedConstraintIsRejected(){jdbc.execute("ALTER TABLE wf_deployments DROP CONSTRAINT wf_deployments_version_check");rejects();}
 @Test void disabledDeferredTriggerIsRejected(){jdbc.execute("ALTER TABLE wf_execution_commands DISABLE TRIGGER wf_execution_result_required");rejects();}
 @Test void disabledForeignKeyTriggersAreRejected(){jdbc.execute("ALTER TABLE wf_execution_tasks DISABLE TRIGGER ALL");rejects();}
 @Test void changedTriggerFunctionIsRejected(){jdbc.execute("CREATE OR REPLACE FUNCTION wf_execution_result_required() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END; $$");rejects();}
 @Test void enabledRowSecurityIsRejected(){jdbc.execute("ALTER TABLE wf_execution_tasks ENABLE ROW LEVEL SECURITY");rejects();}
 @Test void wrongNativeVersionIsRejected(){jdbc.update("UPDATE act_ge_property SET value_='7.0.0.0' WHERE name_='schema.version'");rejects();}
 @Test void wrongCommonVersionIsRejected(){jdbc.update("UPDATE act_ge_property SET value_='7.0.0.0' WHERE name_='common.schema.version'");rejects();}
 @Test void missingVersionIsRejected(){jdbc.update("DELETE FROM act_ge_property WHERE name_='common.schema.version'");rejects();}
 @Test void superuserIsRejected(){runtime=new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only");rejects();}
 @Test void tableOwnerIsRejected(){admin.execute("ALTER TABLE "+schema+".act_hi_procinst OWNER TO "+role);rejects();}
 @Test void ownerMembershipIsRejectedEvenWithoutInheritance(){admin.execute("GRANT b3_fixture TO "+role);rejects();}
 @Test void schemaCreatePrivilegeIsRejected(){admin.execute("GRANT CREATE ON SCHEMA "+schema+" TO "+role);rejects();}
 @Test void databaseCreatePrivilegeIsRejected(){admin.execute("GRANT CREATE ON DATABASE b3_flowable_fixture TO "+role);rejects();}
 @Test void temporaryTablePrivilegeIsRejected(){admin.execute("GRANT TEMPORARY ON DATABASE b3_flowable_fixture TO "+role);rejects();}
 @Test void truncatePrivilegeIsRejected(){admin.execute("GRANT TRUNCATE ON "+schema+".act_hi_procinst TO "+role);rejects();}
 @Test void protocolDeletePrivilegeIsRejected(){admin.execute("GRANT DELETE ON "+schema+".wf_deployments TO "+role);rejects();}
 @Test void immutableProtocolUpdatePrivilegeIsRejected(){grantUpdate("wf_deployments","bpmn_sha256");rejects();}
 @Test void missingRequiredReadPrivilegeIsRejected(){admin.execute("REVOKE SELECT ON "+schema+".act_ru_task FROM "+role);rejects();}
 @Test void missingRequiredUpdatePrivilegeIsRejected(){admin.execute("REVOKE UPDATE(decision) ON "+schema+".wf_execution_tasks FROM "+role);rejects();}
 @Test void missingSequencePrivilegeIsRejected(){admin.execute("REVOKE ALL ON SEQUENCE "+schema+".act_evt_log_log_nr__seq FROM "+role);rejects();}
 @Test void tableRegrantPrivilegeIsRejected(){admin.execute("GRANT SELECT ON "+schema+".act_hi_procinst TO "+role+" WITH GRANT OPTION");rejects();}
 @Test void columnRegrantPrivilegeIsRejected(){admin.execute("GRANT UPDATE(decision) ON "+schema+".wf_execution_tasks TO "+role+" WITH GRANT OPTION");rejects();}
 @Test void sequenceRegrantPrivilegeIsRejected(){admin.execute("GRANT USAGE ON SEQUENCE "+schema+".act_evt_log_log_nr__seq TO "+role+" WITH GRANT OPTION");rejects();}
 @Test void invalidArgumentsFailClosedWithoutConnecting(){
  var monitor=new Monitor(runtime);
  for(String invalid:List.of("", "pg_catalog", "workflow; DROP SCHEMA public", "x".repeat(64)))
   assertEquals(FAILURE,assertThrows(IllegalStateException.class,()->RuntimeSchema.verify(monitor,invalid,5000)).getMessage());
  for(int invalid:new int[]{0,-1,30001})assertEquals(FAILURE,assertThrows(IllegalStateException.class,()->RuntimeSchema.verify(monitor,schema,invalid)).getMessage());
  assertEquals(FAILURE,assertThrows(IllegalStateException.class,()->RuntimeSchema.verify(null,schema,5000)).getMessage());
  assertEquals(0,monitor.connections);
 }
 @Test void connectionFailureDoesNotExposeSecretOrSqlCause(){
  DataSource broken=new AbstractDataSource(){public Connection getConnection()throws SQLException{throw new SQLException("password=private-test-secret jdbc:internal SQL body");}public Connection getConnection(String u,String p)throws SQLException{return getConnection();}};
  var e=assertThrows(IllegalStateException.class,()->RuntimeSchema.verify(broken,schema,5000));assertEquals(FAILURE,e.getMessage());assertNull(e.getCause());assertEquals(0,e.getSuppressed().length);
 }
 @Test void verificationIsBoundedReadOnlyAndClosesItsConnection(){
  var monitored=new Monitor(runtime);RuntimeSchema.verify(monitored,schema,5000);
  assertEquals(1,monitored.connections);assertTrue(monitored.closed);assertTrue(monitored.ended);assertTrue(monitored.queries.size()>0&&monitored.queries.size()<=64);
  assertTrue(monitored.queries.stream().anyMatch(q->q.contains("act_ge_property")),"must validate actual native version values");
 }
 static class Monitor extends AbstractDataSource {
  final DataSource delegate;int connections;boolean closed,ended;List<String> queries=new ArrayList<>();
  Monitor(DataSource delegate){this.delegate=delegate;}
  public Connection getConnection()throws SQLException{
   Connection c=delegate.getConnection();connections++;
   return (Connection)Proxy.newProxyInstance(Connection.class.getClassLoader(),new Class[]{Connection.class},(p,m,a)->{
    if(m.getName().equals("close"))closed=true;
    if(m.getName().equals("rollback"))ended=true;
    assertNotEquals("commit",m.getName(),"startup validation must roll back its read-only transaction");
    Object result=invoke(c,m,a);
    if(result instanceof Statement st){String sql=a!=null&&a.length>0&&a[0] instanceof String?(String)a[0]:null;Class<?> type=result instanceof PreparedStatement?PreparedStatement.class:Statement.class;
     return Proxy.newProxyInstance(type.getClassLoader(),new Class[]{type},(sp,sm,sa)->{
      if(sm.getName().startsWith("execute")){
       assertTrue(c.isReadOnly());assertFalse(c.getAutoCommit());assertEquals(Connection.TRANSACTION_REPEATABLE_READ,c.getTransactionIsolation());
       assertTrue(st.getQueryTimeout()>0&&st.getQueryTimeout()<=5,"every executed statement must be bounded");
       String query=(sql==null?(String)sa[0]:sql).toLowerCase(Locale.ROOT).strip();queries.add(query);
       assertTrue(query.startsWith("select ")||query.startsWith("set local "),"only read-only queries/local session settings allowed: "+query);
       assertFalse(query.contains("goose_db_version"),"runtime cannot inspect migration ledger");
       assertFalse(query.matches("(?s).*\\b(from|join)\\s+[^ ,;]*(act_hi_|act_ru_|wf_execution_|wf_deployments).*"),"must not scan business rows");
      }
      return invoke(st,sm,sa);
     });
    }return result;
   });
  }
  public Connection getConnection(String u,String p)throws SQLException{return getConnection();}
  static Object invoke(Object target,Method method,Object[] args)throws Throwable{try{return method.invoke(target,args);}catch(InvocationTargetException failure){throw failure.getCause();}}
 }
}
