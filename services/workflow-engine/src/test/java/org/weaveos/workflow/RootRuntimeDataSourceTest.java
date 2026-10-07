package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import com.zaxxer.hikari.HikariDataSource;
import java.sql.*;
import java.util.*;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.datasource.DriverManagerDataSource;

/** Root-owned actual bounded-pool contract. Synthetic isolated storage only. */
class RootRuntimeDataSourceTest {
 final RootRuntimeSchemaTest fixture=new RootRuntimeSchemaTest();
 final List<HikariDataSource> pools=new ArrayList<>();
 @BeforeEach void setup()throws Exception{fixture.setup();}
 @AfterEach void cleanup(){try{for(var pool:pools)pool.close();}finally{fixture.cleanup();}}
 Map<String,String> environment(){
  return new HashMap<>(Map.of(
   "WEAVEOS_ENGINE_JDBC_URL",fixture.url,"WEAVEOS_ENGINE_DB_USER",fixture.role,
   "WEAVEOS_ENGINE_DB_PASSWORD","v041_synthetic_only","WEAVEOS_ENGINE_SERVICE_TOKEN","V041_SYNTHETIC_TEST_ONLY_1234567890",
   "WEAVEOS_ENGINE_SCHEMA",fixture.schema,"WEAVEOS_ENGINE_POOL_MAX","2",
   "WEAVEOS_ENGINE_CONNECTION_TIMEOUT_MS","250","WEAVEOS_ENGINE_STATEMENT_TIMEOUT_MS","1000",
   "WEAVEOS_ENGINE_LOCK_TIMEOUT_MS","150"));
 }
 HikariDataSource open(Map<String,String> env){
  var pool=RuntimeDataSource.open(RuntimeConfiguration.read(env));assertNotNull(pool,"a real bounded datasource is required");pools.add(pool);return pool;
 }
 static String scalar(Connection c,String sql)throws SQLException{try(var s=c.createStatement();var r=s.executeQuery(sql)){assertTrue(r.next());return r.getString(1);}}
 @Test void poolUsesValidatedSchemaAndSessionTimeouts()throws Exception{
  var pool=open(environment());assertEquals(2,pool.getMaximumPoolSize());assertEquals(0,pool.getMinimumIdle());assertEquals(250,pool.getConnectionTimeout());
  try(var c=pool.getConnection()){
   assertEquals(fixture.schema,scalar(c,"SELECT current_schema()"));assertEquals(fixture.role,scalar(c,"SELECT current_user"));
   assertEquals("1000",scalar(c,"SELECT setting FROM pg_settings WHERE name='statement_timeout'"));
   assertEquals("150",scalar(c,"SELECT setting FROM pg_settings WHERE name='lock_timeout'"));
   assertTrue(c.getAutoCommit());assertFalse(c.isReadOnly());
  }
 }
 @Test void returnedConnectionIsReusedWithoutUnboundedGrowth()throws Exception{
  var pool=open(environment());String first;
  try(var c=pool.getConnection()){first=scalar(c,"SELECT pg_backend_pid()");}
  for(int i=0;i<8;i++)try(var c=pool.getConnection()){assertEquals(first,scalar(c,"SELECT pg_backend_pid()"));}
  assertEquals(1,pool.getHikariPoolMXBean().getTotalConnections());
 }
 @Test void fullPoolTimesOutThenRecoversAfterReturn()throws Exception{
  var pool=open(environment());Connection one=pool.getConnection(),two=pool.getConnection();
  try{
   assertEquals(2,pool.getHikariPoolMXBean().getActiveConnections());
   long start=System.nanoTime();assertThrows(SQLTransientConnectionException.class,pool::getConnection);
   long ms=(System.nanoTime()-start)/1_000_000;assertTrue(ms>=150&&ms<5000,"bounded pool wait, observed "+ms+"ms");
   one.close();try(var recovered=pool.getConnection()){assertEquals("1",scalar(recovered,"SELECT 1"));}
   assertTrue(pool.getHikariPoolMXBean().getTotalConnections()<=2);
  }finally{one.close();two.close();}
 }
 @Test void actualStatementTimeoutCancelsSlowQuery()throws Exception{
  var env=environment();env.put("WEAVEOS_ENGINE_STATEMENT_TIMEOUT_MS","100");env.put("WEAVEOS_ENGINE_LOCK_TIMEOUT_MS","50");
  var pool=open(env);try(var c=pool.getConnection();var s=c.createStatement()){
   var error=assertThrows(SQLException.class,()->s.execute("SELECT pg_sleep(2)"));assertEquals("57014",error.getSQLState());
   assertEquals("1",scalar(c,"SELECT 1"));
  }
 }
 @Test void actualLockTimeoutDoesNotLeaveAChangedRow()throws Exception{
  var pool=open(environment());
  String before=fixture.jdbc.queryForObject("SELECT value_ FROM act_ge_property WHERE name_='next.dbid'",String.class);
  assertNotNull(before);
  try(var owner=new DriverManagerDataSource(fixture.url+"?currentSchema="+fixture.schema,"b3_fixture","b3_fixture_only").getConnection()){
   owner.setAutoCommit(false);try(var lock=owner.createStatement()){
    lock.execute("SELECT * FROM act_ge_property WHERE name_='next.dbid' FOR UPDATE");
    try(var c=pool.getConnection();var s=c.createStatement()){
     var error=assertThrows(SQLException.class,()->s.executeUpdate("UPDATE act_ge_property SET value_='999999' WHERE name_='next.dbid'"));assertEquals("55P03",error.getSQLState());
    }
   }finally{owner.rollback();}
  }
  assertEquals(before,fixture.jdbc.queryForObject("SELECT value_ FROM act_ge_property WHERE name_='next.dbid'",String.class));
 }
 @Test void schemaGuardDoesNotLeakReadOnlyTransactionStateIntoPool()throws Exception{
  var pool=open(environment());RuntimeSchema.verify(pool,fixture.schema,5000);
  try(var c=pool.getConnection()){
   assertFalse(c.isReadOnly());assertTrue(c.getAutoCommit());assertEquals(Connection.TRANSACTION_READ_COMMITTED,c.getTransactionIsolation());
   assertEquals(fixture.schema,scalar(c,"SELECT current_schema()"));
  }
 }
 @Test void closeRejectsBorrowAndReleasesServerConnections()throws Exception{
  var pool=open(environment());try(var c=pool.getConnection()){assertEquals("1",scalar(c,"SELECT 1"));}
  pool.close();assertDoesNotThrow(pool::close);assertTrue(pool.isClosed());assertThrows(SQLException.class,pool::getConnection);
  long deadline=System.nanoTime()+5_000_000_000L;int count;
  do{count=fixture.admin.queryForObject("SELECT count(*) FROM pg_stat_activity WHERE usename=?",Integer.class,fixture.role);if(count==0)break;Thread.sleep(10);}while(System.nanoTime()<deadline);
  assertEquals(0,count);
 }
 @Test void wrongCredentialsFailWithoutSecretCause(){
  var env=environment();env.put("WEAVEOS_ENGINE_DB_PASSWORD","private-synthetic-wrong-password");
  var error=assertThrows(IllegalStateException.class,()->RuntimeDataSource.open(RuntimeConfiguration.read(env)));
  assertEquals("workflow database initialization failed",error.getMessage());assertNull(error.getCause());assertEquals(0,error.getSuppressed().length);
 }
 @Test void nullConfigurationFailsClosed(){
  var error=assertThrows(IllegalStateException.class,()->RuntimeDataSource.open(null));assertEquals("workflow database initialization failed",error.getMessage());assertNull(error.getCause());
 }
}
