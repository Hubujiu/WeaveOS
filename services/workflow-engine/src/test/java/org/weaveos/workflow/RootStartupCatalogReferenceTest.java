package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.sql.*;
import java.util.*;
import org.junit.jupiter.api.Test;

/** Independent reference capture from vendor-created schema and reviewed protocol fixture.
 * No runtime verifier implementation is called or consulted. */
class RootStartupCatalogReferenceTest {
 @Test void captureOfficialNativeAndProtocolCatalog() throws Exception {
  RootExecutionRegistryTest fixture=new RootExecutionRegistryTest();
  try {
   fixture.setup();List<String> rows=new ArrayList<>();
   String scope="(left(c.relname,4) IN ('act_','flw_') OR left(c.relname,3)='wf_')";
   List<String> queries=List.of(
    "SELECT 'T|'||c.relname||'|'||c.relkind::text||'|'||c.relpersistence::text||'|'||c.relrowsecurity||'|'||c.relforcerowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=? AND c.relkind IN ('r','p') AND "+scope,
    "SELECT 'C|'||c.relname||'|'||a.attname||'|'||a.attnum||'|'||format_type(a.atttypid,a.atttypmod)||'|'||a.attnotnull||'|'||coalesce(pg_get_expr(d.adbin,d.adrelid),'') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum WHERE n.nspname=? AND c.relkind IN ('r','p') AND a.attnum>0 AND NOT a.attisdropped AND "+scope,
    "SELECT 'K|'||c.relname||'|'||k.conname||'|'||k.contype::text||'|'||pg_get_constraintdef(k.oid)||'|'||k.condeferrable||'|'||k.condeferred||'|'||k.convalidated FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_constraint k ON k.conrelid=c.oid WHERE n.nspname=? AND "+scope,
    "SELECT 'I|'||c.relname||'|'||pg_get_indexdef(i.indexrelid)||'|'||i.indisvalid||'|'||i.indisready FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_index i ON i.indrelid=c.oid WHERE n.nspname=? AND "+scope,
    "SELECT 'F|'||p.proname||'|'||pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=? AND p.prokind='f' AND left(p.proname,3)='wf_'",
    "SELECT 'G|'||c.relname||'|'||coalesce(k.conname,'')||'|'||CASE WHEN t.tgisinternal THEN 'internal' ELSE t.tgname END||'|'||p.proname||'|'||t.tgtype||'|'||t.tgenabled::text||'|'||t.tgdeferrable||'|'||t.tginitdeferred||'|'||coalesce(pg_get_expr(t.tgqual,t.tgrelid),'') FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_proc p ON p.oid=t.tgfoid LEFT JOIN pg_constraint k ON k.oid=t.tgconstraint WHERE n.nspname=? AND "+scope,
    "SELECT 'S|'||sequencename||'|'||data_type||'|'||start_value||'|'||min_value||'|'||max_value||'|'||increment_by||'|'||cycle||'|'||cache_size FROM pg_sequences WHERE schemaname=? AND (left(sequencename,4) IN ('act_','flw_') OR left(sequencename,3)='wf_')"
   );
   try(Connection connection=fixture.ds.getConnection()) {
    connection.setReadOnly(true);connection.setTransactionIsolation(Connection.TRANSACTION_REPEATABLE_READ);connection.setAutoCommit(false);
    try(Statement statement=connection.createStatement()){statement.execute("SET LOCAL search_path TO pg_catalog");}
    for(String sql:queries)try(PreparedStatement statement=connection.prepareStatement(sql)){
     statement.setString(1,fixture.schema);
     try(ResultSet result=statement.executeQuery()){while(result.next())rows.add(result.getString(1).replace(fixture.schema+".","<schema>."));}
    }
    connection.rollback();
   }
   Collections.sort(rows);
   assertEquals(37,rows.stream().filter(row->row.startsWith("T|")).count());
   assertEquals(3,rows.stream().filter(row->row.startsWith("F|")).count());
   assertEquals(2,rows.stream().filter(row->row.startsWith("S|")).count());
   assertTrue(rows.size()>500 && rows.size()<4096);
   StringBuilder manifest=new StringBuilder("# Flowable8.0.0 / PostgreSQL18 native schema plus reviewed WeaveOS protocol structure\n# Each non-comment line is base64 UTF-8; sorted before encoding. Schema qualifier is <schema>.\n");
   for(String row:rows){assertTrue(row.getBytes(StandardCharsets.UTF_8).length<=8192);manifest.append(Base64.getEncoder().encodeToString(row.getBytes(StandardCharsets.UTF_8))).append('\n');}
   byte[] bytes=manifest.toString().getBytes(StandardCharsets.UTF_8);assertTrue(bytes.length<2*1024*1024);
   Path output=Path.of("ci-logs/runtime-catalog-reference.txt");Files.createDirectories(output.getParent());Files.write(output,bytes);
   System.out.println("V041_CATALOG_META:rows="+rows.size()+";bytes="+bytes.length+";sha256="+HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes)));
   // The initial independent reference has been captured and committed. Keep the
   // actual artifact and hash, without flooding each CI log with 185 KB of Base64.
   try(var frozen=getClass().getResourceAsStream("/runtime-catalog-reference.txt")){
    assertNotNull(frozen);assertArrayEquals(frozen.readAllBytes(),bytes);
   }
  } finally {fixture.cleanup();}
 }
}
