package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.file.*;
import java.sql.SQLException;
import java.util.UUID;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Restored from the frozen V067 data contract; all evidence is freshly executed after recovery. */
class RootFlowDeletionSchemaTest {
 JdbcTemplate admin,jdbc;String schema,url;TransactionTemplate tx;
 final String app="10000000-0000-4000-8000-000000000001",flow="20000000-0000-4000-8000-000000000001",op="70000000-0000-4000-8000-000000000001";
 final Path migration=Path.of("schema/migrations/00002_flow_deletion_guards.sql");
 @BeforeEach void setup()throws Exception{
  url=System.getenv("B3_TEST_JDBC_URL");if(!"jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture".equals(url)){url=System.getenv("WEAVEOS_V067_TEST_JDBC_URL");assertEquals("jdbc:postgresql://127.0.0.1:55445/weaveos_v067_engine_test",url);}
  admin=new JdbcTemplate(new DriverManagerDataSource(url,"b3_fixture","b3_fixture_only"));schema="v067_schema_"+UUID.randomUUID().toString().replace("-","");admin.execute("CREATE SCHEMA "+schema);
  var ds=new DriverManagerDataSource(url+"?currentSchema="+schema,"b3_fixture","b3_fixture_only");jdbc=new JdbcTemplate(ds);tx=new TransactionTemplate(new DataSourceTransactionManager(ds));
  jdbc.execute(Files.readString(Path.of("schema/migrations/00001_flowable8.sql")).split("-- \\+goose Down",-1)[0]);
 }
 @AfterEach void cleanup(){if(admin!=null&&schema!=null)admin.execute("DROP SCHEMA "+schema+" CASCADE");}
 void migrate(boolean down)throws Exception{
  String sql=Files.exists(migration)?Files.readString(migration):"";
  if(!sql.isEmpty()){String[] parts=sql.split("-- \\+goose Down",-1);assertEquals(2,parts.length);tx.executeWithoutResult(s->jdbc.execute(parts[down?1:0]));}
 }
 void present(){assertNotNull(jdbc.queryForObject("SELECT to_regclass('wf_flow_deletion_guards')::text",String.class),"explicit upgrade must install persistent flow identity guards");}
 void publish(String scope){jdbc.update("INSERT INTO wf_deployments(version_id,app_id,flow_id,version,bpmn_sha256,status,engine_deployment_id,process_definition_id) VALUES('30000000-0000-4000-8000-000000000001',?::uuid,?::uuid,1,repeat('a',64),'confirmed','synthetic-deployment','synthetic-definition')",scope,flow);}
 String history(){return jdbc.queryForObject("SELECT coalesce(jsonb_agg(to_jsonb(d)),'[]'::jsonb)::text FROM wf_deployments d",String.class);}
 void retire(){jdbc.update("UPDATE wf_flow_deletion_guards SET retired=true,operation_id=?::uuid,deleted_versions=1,deleted_at=clock_timestamp() WHERE app_id=?::uuid AND flow_id=?::uuid",op,app,flow);}
 @Test void explicitUpgradeBackfillsOldIdentityWithoutRewritingOriginalReceipt()throws Exception{
  publish(app);String before=history();migrate(false);present();assertEquals(before,history());
  assertEquals(1,jdbc.queryForObject("SELECT count(*) FROM wf_flow_deletion_guards WHERE app_id=?::uuid AND flow_id=?::uuid AND NOT retired AND operation_id IS NULL AND deleted_at IS NULL AND deleted_versions IS NULL",Long.class,app,flow));
 }
 @Test void invalidLegacyIdentityRefusesWholeUpgradeWithoutDiscardingHistory(){
  publish("00000000-0000-0000-0000-000000000000");String before=history();assertThrows(Exception.class,()->migrate(false),"zero legacy identity must fail migration");
  assertEquals(before,history());assertNull(jdbc.queryForObject("SELECT to_regclass('wf_flow_deletion_guards')::text",String.class));
 }
 @Test void retiredIdentityAndCreationFactsCannotBeRewrittenOrForged()throws Exception{
  publish(app);migrate(false);present();
  assertThrows(Exception.class,()->jdbc.update("UPDATE wf_flow_deletion_guards SET flow_id='20000000-0000-4000-8000-000000000002'"));
  assertThrows(Exception.class,()->jdbc.update("UPDATE wf_flow_deletion_guards SET created_at=created_at+interval '1 second'"));
  assertThrows(Exception.class,()->jdbc.update("INSERT INTO wf_flow_deletion_guards(app_id,flow_id,retired,operation_id,deleted_versions,deleted_at) VALUES(?::uuid,'20000000-0000-4000-8000-000000000002',true,?::uuid,0,clock_timestamp())",app,op));
  retire();String before=jdbc.queryForObject("SELECT to_jsonb(g)::text FROM wf_flow_deletion_guards g",String.class);
  assertThrows(Exception.class,()->jdbc.update("UPDATE wf_flow_deletion_guards SET deleted_versions=2"));
  assertThrows(Exception.class,()->jdbc.update("UPDATE wf_flow_deletion_guards SET retired=false,operation_id=null,deleted_versions=null,deleted_at=null"));
  assertEquals(before,jdbc.queryForObject("SELECT to_jsonb(g)::text FROM wf_flow_deletion_guards g",String.class));
 }
 @Test void downgradeRefusesRetiredHistoryButAllowsReconstructibleLiveRows()throws Exception{
  publish(app);String before=history();migrate(false);present();migrate(true);assertNull(jdbc.queryForObject("SELECT to_regclass('wf_flow_deletion_guards')::text",String.class));assertEquals(before,history());
  migrate(false);present();retire();Exception e=assertThrows(Exception.class,()->migrate(true));Throwable cause=e;while(cause.getCause()!=null)cause=cause.getCause();assertInstanceOf(SQLException.class,cause);assertEquals("55000",((SQLException)cause).getSQLState());present();assertEquals(before,history());
 }
 @Test void newGateUsesInvokerPinnedSearchPathAndNoPublicExecute()throws Exception{
  migrate(false);present();var row=jdbc.queryForMap("SELECT p.prosecdef,p.proconfig,EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') AS public_execute FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=? AND p.proname='wf_flow_deletion_guard_immutable'",schema);
  assertEquals(false,row.get("prosecdef"));assertTrue(java.util.Arrays.asList((String[])((java.sql.Array)row.get("proconfig")).getArray()).contains("search_path=pg_catalog"));assertEquals(false,row.get("public_execute"));
 }
}
