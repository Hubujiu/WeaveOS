package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import org.flowable.engine.ProcessEngineConfiguration;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Real SQL payload verification; Goose CLI and runtime grants are separate acceptance stages. */
class RootExplicitSchemaTest {
 String schema,url,up,down;
 JdbcTemplate admin,jdbc;
 DriverManagerDataSource ds;
 DataSourceTransactionManager manager;
 @BeforeEach void setup() throws Exception {
  url=System.getenv("B3_TEST_JDBC_URL");
  assertEquals("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture",url,"only isolated synthetic storage permitted");
  admin=new JdbcTemplate(new DriverManagerDataSource(url,"b3_fixture","b3_fixture_only"));
  schema="v041_schema_"+UUID.randomUUID().toString().replace("-","");
  admin.execute("CREATE SCHEMA "+schema);
  ds=new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only");
  jdbc=new JdbcTemplate(ds);manager=new DataSourceTransactionManager(ds);
  String file=Files.readString(Path.of("schema/migrations/00001_flowable8.sql"));
  String[] parts=file.split("-- \\+goose Down",-1);assertEquals(2,parts.length,"one explicit Down boundary");
  assertTrue(parts[0].contains("-- +goose Up"));up=parts[0]+"\n"+Files.readString(Path.of("schema/migrations/00002_flow_deletion_guards.sql")).split("-- \\+goose Down",-1)[0];down=parts[1];
 }
 @AfterEach void cleanup(){if(admin!=null&&schema!=null)admin.execute("DROP SCHEMA "+schema+" CASCADE");}
 String rootMessage(Throwable error){while(error.getCause()!=null)error=error.getCause();return error.getMessage();}
 void migrate(){new TransactionTemplate(manager).executeWithoutResult(tx->jdbc.execute(up));}
 List<String> tables(String namespace,String prefix){return admin.queryForList("SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=? AND c.relkind='r' AND c.relname LIKE ? ORDER BY c.relname",String.class,namespace,prefix+"%");}
 void requireTable(String table){assertTrue(tables(schema,table).contains(table),"migration missing "+table);}
 List<String> catalog(String namespace,String prefix){
  List<String> rows=new ArrayList<>();
  String[] queries={
   "SELECT 'column|'||c.relname||'|'||a.attname||'|'||format_type(a.atttypid,a.atttypmod)||'|'||a.attnotnull||'|'||coalesce(pg_get_expr(d.adbin,d.adrelid),'') AS item FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum WHERE n.nspname=? AND c.relkind='r' AND c.relname LIKE ? AND a.attnum>0 AND NOT a.attisdropped",
   "SELECT 'constraint|'||c.relname||'|'||k.conname||'|'||pg_get_constraintdef(k.oid)||'|'||k.condeferrable||'|'||k.condeferred AS item FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_constraint k ON k.conrelid=c.oid WHERE n.nspname=? AND c.relname LIKE ?",
   "SELECT 'index|'||c.relname||'|'||pg_get_indexdef(i.indexrelid) AS item FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_index i ON i.indrelid=c.oid WHERE n.nspname=? AND c.relname LIKE ?"
  };
  for(String sql:queries)for(String row:admin.queryForList(sql,String.class,namespace,prefix+"%"))rows.add(row.replace(namespace+".","<schema>."));
  Collections.sort(rows);return rows;
 }
 @Test void nativeCatalogMatchesOfficialEngineCreatedSchema() throws Exception {
  RootExecutionRegistryTest baseline=new RootExecutionRegistryTest();
  try {baseline.setup();migrate();assertEquals(32,tables(baseline.schema,"act_").size()+tables(baseline.schema,"flw_").size());for(String prefix:List.of("act_","flw_")){assertEquals(tables(baseline.schema,prefix),tables(schema,prefix));assertEquals(catalog(baseline.schema,prefix),catalog(schema,prefix));}}
  finally {baseline.cleanup();}
 }
 @Test void nativeVersionMetadataIsPinned(){migrate();requireTable("act_ge_property");assertEquals("8.0.0.0",jdbc.queryForObject("SELECT value_ FROM act_ge_property WHERE name_='schema.version'",String.class));assertEquals("8.0.0.0",jdbc.queryForObject("SELECT value_ FROM act_ge_property WHERE name_='common.schema.version'",String.class));assertEquals("create(8.0.0.0)",jdbc.queryForObject("SELECT value_ FROM act_ge_property WHERE name_='schema.history'",String.class));}
 @Test void protocolLedgerCatalogMatchesReviewedContract() throws Exception {
  RootExecutionRegistryTest baseline=new RootExecutionRegistryTest();
  try {baseline.setup();migrate();assertEquals(List.of("wf_deployments","wf_execution_commands","wf_execution_instances","wf_execution_tasks","wf_flow_deletion_guards"),tables(schema,"wf_"));assertEquals(catalog(baseline.schema,"wf_"),catalog(schema,"wf_"));}
  finally {baseline.cleanup();}
 }
 @Test void pendingCommandCannotCommit(){
  migrate();requireTable("wf_execution_commands");
  var failure=assertThrows(RuntimeException.class,()->new TransactionTemplate(manager).executeWithoutResult(tx->jdbc.update("INSERT INTO wf_execution_commands(command_id,instance_id,command_hash,command_bytes,payload_hash,payload_bytes) VALUES (?::uuid,?::uuid,?,decode('01','hex'),?,decode('01','hex'))",UUID.randomUUID().toString(),UUID.randomUUID().toString(),"0".repeat(64),"1".repeat(64))));
  assertTrue(rootMessage(failure).contains("execution command requires a durable result"));
  assertEquals(0L,jdbc.queryForObject("SELECT count(*) FROM wf_execution_commands",Long.class));
 }
 @Test void provisionalInstanceCannotCommit(){
  migrate();requireTable("wf_execution_instances");String id=UUID.randomUUID().toString();
  jdbc.update("INSERT INTO wf_deployments(version_id,app_id,flow_id,version,bpmn_sha256,status,engine_deployment_id,process_definition_id) VALUES (?::uuid,?::uuid,?::uuid,1,?,'confirmed','synthetic-deployment','synthetic-definition')",id,id,id,"0".repeat(64));
  var failure=assertThrows(RuntimeException.class,()->new TransactionTemplate(manager).executeWithoutResult(tx->jdbc.update("INSERT INTO wf_execution_instances(instance_id,app_id,table_id,view_id,record_id,flow_id,version_id,definition_version,initiator_id,start_payload_bytes,allow_withdraw,state,sequence,fence_epoch,schema_version,record_version) VALUES (?::uuid,?::uuid,?::uuid,?::uuid,?::uuid,?::uuid,?::uuid,1,?::uuid,decode('01','hex'),false,'starting',0,1,1,1)",id,id,id,id,id,id,id,id)));
  assertTrue(rootMessage(failure).contains("engine instance may not commit provisional state"));
  assertEquals(0L,jdbc.queryForObject("SELECT count(*) FROM wf_execution_instances",Long.class));
 }
 @Test void installationFailureRollsBackAllTables(){assertThrows(RuntimeException.class,()->new TransactionTemplate(manager).executeWithoutResult(tx->{jdbc.execute(up);jdbc.execute("SELECT 1/0");}));assertTrue(tables(schema,"").isEmpty(),"partial migration committed");}
 @Test void downRefusesToDeleteExistingHistory(){migrate();requireTable("act_hi_procinst");jdbc.update("INSERT INTO act_hi_procinst(id_,proc_inst_id_,proc_def_id_,start_time_) VALUES ('synthetic-history','synthetic-history','synthetic-definition',TIMESTAMP '2026-10-06 00:00:00')");assertThrows(RuntimeException.class,()->new TransactionTemplate(manager).executeWithoutResult(tx->jdbc.execute(down)));requireTable("act_hi_procinst");requireTable("wf_execution_commands");assertEquals(1L,jdbc.queryForObject("SELECT count(*) FROM act_hi_procinst WHERE id_='synthetic-history'",Long.class));}
 @Test void engineStartsWithoutAutoCreatingOrUpgrading(){
  migrate();requireTable("act_ge_property");List<String> before=catalog(schema,"act_");
  var configuration=new SpringProcessEngineConfiguration();configuration.setDataSource(ds);configuration.setTransactionManager(manager);configuration.setDatabaseSchema(schema);configuration.setDatabaseSchemaUpdate(ProcessEngineConfiguration.DB_SCHEMA_UPDATE_FALSE);configuration.setDisableIdmEngine(true);configuration.setDisableEventRegistry(true);configuration.setAsyncExecutorActivate(false);
  var engine=configuration.buildProcessEngine();try{assertNotNull(engine.getRuntimeService());assertEquals(before,catalog(schema,"act_"));}finally{engine.close();}
 }
}
