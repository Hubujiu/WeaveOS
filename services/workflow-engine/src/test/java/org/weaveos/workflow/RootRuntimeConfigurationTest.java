package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.util.*;
import org.junit.jupiter.api.Test;

/** Pure configuration tests, no sockets, real credentials or database access. */
class RootRuntimeConfigurationTest {
 static final String PREFIX="WEAVEOS_ENGINE_";
 static final String TOKEN="V041_SYNTHETIC_CONFIG_TOKEN_1234567890";
 static final String PASSWORD="synthetic-config-only-password";
 Map<String,String> valid(){return new HashMap<>(Map.of(PREFIX+"JDBC_URL","jdbc:postgresql://engine-db:5432/flowable",PREFIX+"DB_USER","workflow_runtime",PREFIX+"DB_PASSWORD",PASSWORD,PREFIX+"SERVICE_TOKEN",TOKEN));}
 RuntimeConfiguration with(String key,String value){var map=valid();map.put(PREFIX+key,value);return RuntimeConfiguration.read(map);}
 void invalid(Map<String,String> values){var error=assertThrows(IllegalArgumentException.class,()->RuntimeConfiguration.read(values));assertEquals("invalid workflow runtime configuration",error.getMessage());}
 @Test void completeConfigurationUsesBoundedDefaults(){
  var c=RuntimeConfiguration.read(valid());assertEquals("jdbc:postgresql://engine-db:5432/flowable",c.jdbcUrl());assertEquals("workflow_runtime",c.dbUser());assertEquals(PASSWORD,c.dbPassword());assertEquals("workflow",c.schema());assertEquals("127.0.0.1",c.bindHost());assertEquals(50051,c.port());assertEquals(8,c.poolMax());assertEquals(4,c.rpcThreads());assertEquals(32,c.queueCapacity());assertEquals(3000,c.connectionTimeoutMs());assertEquals(10000,c.statementTimeoutMs());assertEquals(1000,c.lockTimeoutMs());assertEquals(10000,c.shutdownTimeoutMs());assertNotNull(c.serviceIdentity());
 }
 @Test void requiredFieldsFailClosed(){
  invalid(null);invalid(Map.of());
  for(String key:List.of("JDBC_URL","DB_USER","DB_PASSWORD","SERVICE_TOKEN"))for(String value:List.of(""," ")){var map=valid();map.put(PREFIX+key,value);invalid(map);}
  for(String key:List.of("JDBC_URL","DB_USER","DB_PASSWORD","SERVICE_TOKEN")){var map=valid();map.remove(PREFIX+key);invalid(map);}
  for(String token:List.of("short",TOKEN+"\n",TOKEN+"=","a".repeat(257))){var map=valid();map.put(PREFIX+"SERVICE_TOKEN",token);invalid(map);}
 }
 @Test void unsafeJdbcTargetsAreRejected(){
  for(String url:List.of("jdbc:mysql://engine-db:5432/flowable","jdbc:postgresql:flowable","jdbc:postgresql://engine-db/flowable","jdbc:postgresql://engine-db:0/flowable","jdbc:postgresql://engine-db:65536/flowable","jdbc:postgresql://engine-db:5432/","jdbc:postgresql://user:password@engine-db:5432/flowable","jdbc:postgresql://engine-db:5432/flowable?password=leak","jdbc:postgresql://engine-db:5432/flowable#fragment","jdbc:postgresql://engine-db:5432/a/b","jdbc:postgresql://8.8.8.8:5432/flowable","jdbc:postgresql://0.0.0.0:5432/flowable","jdbc:postgresql://224.0.0.1:5432/flowable","jdbc:postgresql://[::]:5432/flowable")){var map=valid();map.put(PREFIX+"JDBC_URL",url);invalid(map);}
 }
 @Test void privateAndLoopbackJdbcTargetsAreAccepted(){
  for(String host:List.of("engine-db.internal","localhost","127.0.0.1","10.0.0.2","172.16.0.2","192.168.1.4","[::1]","[fd00::2]")){String url="jdbc:postgresql://"+host+":5432/flowable";assertEquals(url,with("JDBC_URL",url).jdbcUrl());}
 }
 @Test void schemaUsesSafeExplicitIdentifier(){
  for(String value:List.of("app_flow","workflow2"))assertEquals(value,with("SCHEMA",value).schema());
  for(String value:List.of("","public.workflow","pg_catalog","pg_temp","UPPER","bad-schema","workflow;DROP SCHEMA public","a".repeat(64))){var map=valid();map.put(PREFIX+"SCHEMA",value);invalid(map);}
 }
 @Test void bindHostAndPortAreValidated(){
  for(String value:List.of("127.0.0.1","10.0.0.2","0.0.0.0","::1","::","fd00::2"))assertEquals(value,with("BIND_HOST",value).bindHost());
  for(String value:List.of("8.8.8.8","224.0.0.1","host.example","127.0.0.1:50051","")){var map=valid();map.put(PREFIX+"BIND_HOST",value);invalid(map);}
  assertEquals(1,with("PORT","1").port());assertEquals(65535,with("PORT","65535").port());
  for(String value:List.of("0","65536","-1","abc","1.5","")){var map=valid();map.put(PREFIX+"PORT",value);invalid(map);}
 }
 @Test void resourceLimitsRejectInvalidOrUnboundedValues(){
  Map<String,String> excessive=Map.of("POOL_MAX","17","RPC_THREADS","17","QUEUE_CAPACITY","129","CONNECTION_TIMEOUT_MS","30001","STATEMENT_TIMEOUT_MS","30001","LOCK_TIMEOUT_MS","10001","SHUTDOWN_TIMEOUT_MS","30001");
  for(var entry:excessive.entrySet())for(String value:List.of("0","-1","",entry.getValue(),"1s","999999999999999999999")){var map=valid();map.put(PREFIX+entry.getKey(),value);invalid(map);}
  var map=valid();map.put(PREFIX+"CONNECTION_TIMEOUT_MS","249");invalid(map);map=valid();map.put(PREFIX+"SHUTDOWN_TIMEOUT_MS","99");invalid(map);
 }
 @Test void resourceBoundariesAreAccepted(){
  var map=valid();map.put(PREFIX+"POOL_MAX","16");map.put(PREFIX+"RPC_THREADS","16");map.put(PREFIX+"QUEUE_CAPACITY","128");map.put(PREFIX+"CONNECTION_TIMEOUT_MS","30000");map.put(PREFIX+"STATEMENT_TIMEOUT_MS","30000");map.put(PREFIX+"LOCK_TIMEOUT_MS","30000");map.put(PREFIX+"SHUTDOWN_TIMEOUT_MS","30000");var c=RuntimeConfiguration.read(map);assertEquals(16,c.poolMax());assertEquals(16,c.rpcThreads());assertEquals(128,c.queueCapacity());assertEquals(30000,c.connectionTimeoutMs());assertEquals(30000,c.statementTimeoutMs());assertEquals(30000,c.lockTimeoutMs());assertEquals(30000,c.shutdownTimeoutMs());
  map=valid();map.put(PREFIX+"POOL_MAX","1");map.put(PREFIX+"RPC_THREADS","1");map.put(PREFIX+"QUEUE_CAPACITY","1");map.put(PREFIX+"CONNECTION_TIMEOUT_MS","250");map.put(PREFIX+"STATEMENT_TIMEOUT_MS","1");map.put(PREFIX+"LOCK_TIMEOUT_MS","1");map.put(PREFIX+"SHUTDOWN_TIMEOUT_MS","100");c=RuntimeConfiguration.read(map);assertEquals(1,c.poolMax());assertEquals(1,c.rpcThreads());assertEquals(1,c.queueCapacity());assertEquals(250,c.connectionTimeoutMs());assertEquals(1,c.statementTimeoutMs());assertEquals(1,c.lockTimeoutMs());assertEquals(100,c.shutdownTimeoutMs());
 }
 @Test void configurationAndErrorsNeverEchoSecrets(){
  var c=RuntimeConfiguration.read(valid());assertEquals("[REDACTED workflow runtime configuration]",c.toString());assertFalse(c.serviceIdentity()==null,"service identity required");assertFalse(c.serviceIdentity().toString().contains(TOKEN));
  var map=valid();map.put(PREFIX+"JDBC_URL","jdbc:postgresql://operator:"+PASSWORD+"@engine-db:5432/flowable");invalid(map);
 }
 @Test void unknownEngineOptionsAreNotSilentlyIgnored(){
  var map=valid();map.put(PREFIX+"TLS_CERT","obsolete-option");invalid(map);map=valid();map.put(PREFIX+"POOL_MXA","8");invalid(map);
  map=valid();map.put("UNRELATED_ENVIRONMENT","allowed");assertEquals(8,RuntimeConfiguration.read(map).poolMax());
 }
 @Test void parsedConfigurationDoesNotRetainMutableEnvironmentMap(){var map=valid();var c=RuntimeConfiguration.read(map);map.put(PREFIX+"JDBC_URL","changed");map.put(PREFIX+"DB_PASSWORD","changed");assertEquals("jdbc:postgresql://engine-db:5432/flowable",c.jdbcUrl());assertEquals(PASSWORD,c.dbPassword());}
}
