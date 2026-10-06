package org.weaveos.workflow;

import java.io.BufferedReader;
import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Collections;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Set;
import javax.sql.DataSource;

/** Read-only startup validation of the installed structure and runtime authority. */
public final class RuntimeSchema {
    private static final String FAILURE = "workflow runtime schema is not ready";
    private static final int MAX_ITEMS = 4096;
    private static final int MAX_ITEM_BYTES = 8192;
    private static final int MAX_TOTAL_BYTES = 2 * 1024 * 1024;
    private static final String REFERENCE_SHA256 =
        "660bf78ab94353b05b4c37e602aa29b18ff5a730b26a34754f13f0667fd74674";
    private static final String SCOPE =
        "(left(c.relname,4) IN ('act_','flw_') OR left(c.relname,3)='wf_')";

    // SET follows PostgreSQL's actual membership options, including NOINHERIT roles.
    // Also cover an already selected effective role and the original session identity.
    private static final String REACHABLE_ROLE = """
        (r.rolname=current_user OR r.rolname=session_user
         OR pg_has_role(current_user,r.oid,'USAGE')
         OR pg_has_role(current_user,r.oid,'SET')
         OR pg_has_role(session_user,r.oid,'SET'))
        """;

    private static final Map<String, Set<String>> MUTABLE_COLUMNS = Map.of(
        "wf_deployments", Set.of("status", "engine_deployment_id", "process_definition_id"),
        "wf_execution_commands", Set.of(
            "outcome", "result_sequence", "proof_id", "result_hash", "result_bytes"),
        "wf_execution_instances", Set.of(
            "engine_process_id", "state", "sequence", "fence_epoch", "schema_version",
            "record_version", "activation_epoch", "updated_at"),
        "wf_execution_tasks", Set.of("state", "decision"));

    // These projections match the independently captured vendor/protocol reference.
    // They contain no dynamic sequence values or application/history record reads.
    private static final List<String> CATALOG_QUERIES = List.of(
        """
        SELECT 'T|'||c.relname||'|'||c.relkind::text||'|'||c.relpersistence::text||'|'||
               c.relrowsecurity||'|'||c.relforcerowsecurity
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname=? AND c.relkind IN ('r','p') AND
        """ + SCOPE,
        """
        SELECT 'C|'||c.relname||'|'||a.attname||'|'||a.attnum||'|'||
               format_type(a.atttypid,a.atttypmod)||'|'||a.attnotnull||'|'||
               coalesce(pg_get_expr(d.adbin,d.adrelid),'')
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        JOIN pg_attribute a ON a.attrelid=c.oid
        LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
        WHERE n.nspname=? AND c.relkind IN ('r','p') AND a.attnum>0
              AND NOT a.attisdropped AND
        """ + SCOPE,
        """
        SELECT 'K|'||c.relname||'|'||k.conname||'|'||k.contype::text||'|'||
               pg_get_constraintdef(k.oid)||'|'||k.condeferrable||'|'||
               k.condeferred||'|'||k.convalidated
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        JOIN pg_constraint k ON k.conrelid=c.oid
        WHERE n.nspname=? AND
        """ + SCOPE,
        """
        SELECT 'I|'||c.relname||'|'||pg_get_indexdef(i.indexrelid)||'|'||
               i.indisvalid||'|'||i.indisready
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        JOIN pg_index i ON i.indrelid=c.oid
        WHERE n.nspname=? AND
        """ + SCOPE,
        """
        SELECT 'F|'||p.proname||'|'||pg_get_functiondef(p.oid)
        FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
        WHERE n.nspname=? AND p.prokind='f' AND left(p.proname,3)='wf_'
        """,
        """
        SELECT 'G|'||c.relname||'|'||coalesce(k.conname,'')||'|'||
               CASE WHEN t.tgisinternal THEN 'internal' ELSE t.tgname END||'|'||
               p.proname||'|'||t.tgtype||'|'||t.tgenabled::text||'|'||
               t.tgdeferrable||'|'||t.tginitdeferred||'|'||
               coalesce(pg_get_expr(t.tgqual,t.tgrelid),'')
        FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid
        JOIN pg_namespace n ON n.oid=c.relnamespace
        JOIN pg_proc p ON p.oid=t.tgfoid
        LEFT JOIN pg_constraint k ON k.oid=t.tgconstraint
        WHERE n.nspname=? AND
        """ + SCOPE,
        """
        SELECT 'S|'||sequencename||'|'||data_type||'|'||start_value||'|'||
               min_value||'|'||max_value||'|'||increment_by||'|'||cycle||'|'||cache_size
        FROM pg_sequences
        WHERE schemaname=? AND
              (left(sequencename,4) IN ('act_','flw_') OR left(sequencename,3)='wf_')
        """);

    private RuntimeSchema() {}

    public static void verify(DataSource source, String schema, int timeoutMs) {
        require(source != null && schema != null
            && schema.matches("[a-z_][a-z0-9_]{0,62}") && !schema.startsWith("pg_")
            && !schema.equals("information_schema") && timeoutMs >= 1 && timeoutMs <= 30000);
        try {
            List<String> reference = readReference();
            try (Connection connection = source.getConnection()) {
                try {
                    connection.setReadOnly(true);
                    connection.setTransactionIsolation(Connection.TRANSACTION_REPEATABLE_READ);
                    connection.setAutoCommit(false);
                    configureTransaction(connection, timeoutMs);
                    require(reference.equals(readCatalog(connection, schema, timeoutMs)));
                    verifyVersions(connection, schema, timeoutMs);
                    verifyRole(connection, schema, timeoutMs);
                    verifyTablePrivileges(connection, schema, timeoutMs);
                    verifyColumnPrivileges(connection, schema, timeoutMs);
                    verifySequencePrivileges(connection, schema, timeoutMs);
                    verifyNoRegrant(connection, schema, timeoutMs);
                } finally {
                    connection.rollback();
                }
            }
        } catch (Exception failure) {
            // Close/rollback/driver failures may contain credentials and SQL. None
            // of their messages, causes or suppressed exceptions leave this API.
            throw new IllegalStateException(FAILURE);
        }
    }

    private static void configureTransaction(Connection connection, int timeoutMs) throws SQLException {
        try (Statement statement = connection.createStatement()) {
            statement.setQueryTimeout(timeoutSeconds(timeoutMs));
            statement.execute("SET LOCAL statement_timeout = " + timeoutMs);
            statement.execute("SET LOCAL lock_timeout = " + timeoutMs);
            statement.execute("SET LOCAL search_path TO pg_catalog");
        }
    }

    private static List<String> readReference() throws Exception {
        byte[] bytes;
        try (InputStream input = RuntimeSchema.class.getResourceAsStream("/runtime-catalog-reference.txt")) {
            require(input != null);
            bytes = input.readNBytes(MAX_TOTAL_BYTES + 1);
        }
        require(bytes.length <= MAX_TOTAL_BYTES);
        require(REFERENCE_SHA256.equals(
            HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes))));
        List<String> rows = new ArrayList<>();
        int total = 0;
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(
                new ByteArrayInputStream(bytes), StandardCharsets.UTF_8))) {
            for (String line; (line = reader.readLine()) != null;) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                byte[] item = Base64.getDecoder().decode(line);
                require(item.length > 0 && item.length <= MAX_ITEM_BYTES);
                total += item.length;
                require(rows.size() < MAX_ITEMS && total <= MAX_TOTAL_BYTES);
                rows.add(new String(item, StandardCharsets.UTF_8));
            }
        }
        require(!rows.isEmpty());
        Collections.sort(rows);
        return List.copyOf(rows);
    }

    private static List<String> readCatalog(Connection connection, String schema, int timeoutMs)
            throws SQLException {
        List<String> rows = new ArrayList<>();
        int total = 0;
        for (String projection : CATALOG_QUERIES) {
            // Bound both the server result and individual transferred values. A
            // modified function body must not allocate an unbounded client item.
            String sql = "SELECT CASE WHEN octet_length(item) <= " + MAX_ITEM_BYTES
                + " THEN item ELSE NULL END FROM (" + projection + ") catalog(item) LIMIT "
                + (MAX_ITEMS + 1);
            try (PreparedStatement statement = prepare(connection, sql, timeoutMs, schema);
                 ResultSet result = statement.executeQuery()) {
                while (result.next()) {
                    String item = result.getString(1);
                    require(item != null);
                    item = item.replace(schema + ".", "<schema>.");
                    int length = item.getBytes(StandardCharsets.UTF_8).length;
                    total += length;
                    require(length <= MAX_ITEM_BYTES && rows.size() < MAX_ITEMS
                        && total <= MAX_TOTAL_BYTES);
                    rows.add(item);
                }
            }
        }
        Collections.sort(rows);
        return rows;
    }

    private static void verifyVersions(Connection connection, String schema, int timeoutMs)
            throws SQLException {
        // This is fixed native version metadata, not a snapshot of ID allocation
        // or business/history rows. The validated identifier is still quoted.
        String sql = "SELECT name_,value_ FROM \"" + schema + "\".act_ge_property"
            + " WHERE name_ IN ('schema.version','common.schema.version') LIMIT 3";
        int count = 0;
        try (PreparedStatement statement = prepare(connection, sql, timeoutMs);
             ResultSet result = statement.executeQuery()) {
            while (result.next()) {
                require(++count <= 2 && "8.0.0.0".equals(result.getString(2)));
            }
        }
        require(count == 2);
    }

    private static void verifyRole(Connection connection, String schema, int timeoutMs)
            throws SQLException {
        String authority = """
            SELECT NOT EXISTS (
              SELECT 1 FROM pg_roles r WHERE
            """ + REACHABLE_ROLE + """
              AND (r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolreplication
                   OR r.rolbypassrls OR r.oid IN (
                     SELECT d.datdba FROM pg_database d WHERE d.datname=current_database()
                     UNION ALL
                     SELECT n.nspowner FROM pg_namespace n
                       WHERE left(n.nspname,3)<>'pg_' AND n.nspname<>'information_schema'
                     UNION ALL
                     SELECT c.relowner FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
                       WHERE n.nspname=?
                     UNION ALL
                     SELECT p.proowner FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
                       WHERE n.nspname=?)))
            """;
        requireBoolean(connection, authority, timeoutMs, schema, schema);

        String database = """
            SELECT NOT EXISTS (
              SELECT 1 FROM pg_database d CROSS JOIN pg_roles r
              WHERE d.datname=current_database() AND
            """ + REACHABLE_ROLE + """
              AND (has_database_privilege(r.oid,d.oid,'CREATE')
                   OR has_database_privilege(r.oid,d.oid,'TEMPORARY')))
            """;
        requireBoolean(connection, database, timeoutMs);

        String namespaces = """
            SELECT has_schema_privilege(current_user,?,'USAGE') AND NOT EXISTS (
              SELECT 1 FROM pg_namespace n CROSS JOIN pg_roles r
              WHERE left(n.nspname,3)<>'pg_' AND n.nspname<>'information_schema' AND
            """ + REACHABLE_ROLE + """
              AND has_schema_privilege(r.oid,n.oid,'CREATE'))
            """;
        requireBoolean(connection, namespaces, timeoutMs, schema);
    }

    private static void verifyTablePrivileges(Connection connection, String schema, int timeoutMs)
            throws SQLException {
        String sql = """
            SELECT c.relname,has_table_privilege(c.oid,'SELECT'),
                   has_table_privilege(c.oid,'INSERT'),has_table_privilege(c.oid,'UPDATE'),
                   has_table_privilege(c.oid,'DELETE'),
                   EXISTS (SELECT 1 FROM pg_roles r WHERE
            """ + REACHABLE_ROLE + """
                     AND has_table_privilege(r.oid,c.oid,'TRUNCATE')),
                   EXISTS (SELECT 1 FROM pg_roles r WHERE
            """ + REACHABLE_ROLE + """
                     AND has_table_privilege(r.oid,c.oid,'DELETE'))
            FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            WHERE n.nspname=? AND c.relkind IN ('r','p') AND
            """ + SCOPE + " LIMIT " + (MAX_ITEMS + 1);
        int count = 0;
        try (PreparedStatement statement = prepare(connection, sql, timeoutMs, schema);
             ResultSet result = statement.executeQuery()) {
            while (result.next()) {
                require(++count <= MAX_ITEMS && result.getBoolean(2) && result.getBoolean(3)
                    && !result.getBoolean(6));
                String table = result.getString(1);
                if (MUTABLE_COLUMNS.containsKey(table)) {
                    require(!result.getBoolean(7));
                } else {
                    require((table.startsWith("act_") || table.startsWith("flw_"))
                        && result.getBoolean(4) && result.getBoolean(5));
                }
            }
        }
        require(count > 0);
    }

    private static void verifyColumnPrivileges(Connection connection, String schema, int timeoutMs)
            throws SQLException {
        String sql = """
            SELECT c.relname,a.attname,has_column_privilege(c.oid,a.attnum,'UPDATE'),
                   EXISTS (SELECT 1 FROM pg_roles r WHERE
            """ + REACHABLE_ROLE + """
                     AND has_column_privilege(r.oid,c.oid,a.attnum,'UPDATE'))
            FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            JOIN pg_attribute a ON a.attrelid=c.oid
            WHERE n.nspname=? AND c.relkind IN ('r','p') AND left(c.relname,3)='wf_'
                  AND a.attnum>0 AND NOT a.attisdropped
            """ + " LIMIT " + (MAX_ITEMS + 1);
        int count = 0;
        try (PreparedStatement statement = prepare(connection, sql, timeoutMs, schema);
             ResultSet result = statement.executeQuery()) {
            while (result.next()) {
                require(++count <= MAX_ITEMS);
                Set<String> mutable = MUTABLE_COLUMNS.get(result.getString(1));
                require(mutable != null);
                if (mutable.contains(result.getString(2))) {
                    require(result.getBoolean(3));
                } else {
                    require(!result.getBoolean(4));
                }
            }
        }
        require(count > 0);
    }

    private static void verifySequencePrivileges(Connection connection, String schema, int timeoutMs)
            throws SQLException {
        String sql = """
            SELECT has_sequence_privilege(c.oid,'USAGE'),has_sequence_privilege(c.oid,'SELECT'),
                   EXISTS (SELECT 1 FROM pg_roles r WHERE
            """ + REACHABLE_ROLE + """
                     AND has_sequence_privilege(r.oid,c.oid,'UPDATE'))
            FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            WHERE n.nspname=? AND c.relkind='S' AND
            """ + SCOPE + " LIMIT " + (MAX_ITEMS + 1);
        int count = 0;
        try (PreparedStatement statement = prepare(connection, sql, timeoutMs, schema);
             ResultSet result = statement.executeQuery()) {
            while (result.next()) {
                require(++count <= MAX_ITEMS && result.getBoolean(1) && result.getBoolean(2)
                    && !result.getBoolean(3));
            }
        }
    }

    private static void verifyNoRegrant(Connection connection, String schema, int timeoutMs)
            throws SQLException {
        // Comma-separated PostgreSQL privilege tests are intentionally ANY here:
        // every ability to regrant these table/column/sequence permissions is forbidden.
        String tables = """
            SELECT NOT EXISTS (
              SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
              CROSS JOIN pg_roles r WHERE n.nspname=? AND c.relkind IN ('r','p') AND
            """ + SCOPE + " AND " + REACHABLE_ROLE + """
              AND (has_table_privilege(r.oid,c.oid,
                   'SELECT WITH GRANT OPTION,INSERT WITH GRANT OPTION,UPDATE WITH GRANT OPTION,DELETE WITH GRANT OPTION,TRUNCATE WITH GRANT OPTION,REFERENCES WITH GRANT OPTION,TRIGGER WITH GRANT OPTION,MAINTAIN WITH GRANT OPTION')
                OR has_any_column_privilege(r.oid,c.oid,
                   'SELECT WITH GRANT OPTION,INSERT WITH GRANT OPTION,UPDATE WITH GRANT OPTION,REFERENCES WITH GRANT OPTION')))
            """;
        requireBoolean(connection, tables, timeoutMs, schema);
        String sequences = """
            SELECT NOT EXISTS (
              SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
              CROSS JOIN pg_roles r WHERE n.nspname=? AND c.relkind='S' AND
            """ + SCOPE + " AND " + REACHABLE_ROLE + """
              AND has_sequence_privilege(r.oid,c.oid,
                  'USAGE WITH GRANT OPTION,SELECT WITH GRANT OPTION,UPDATE WITH GRANT OPTION'))
            """;
        requireBoolean(connection, sequences, timeoutMs, schema);
    }

    private static void requireBoolean(Connection connection, String sql, int timeoutMs,
            String... parameters) throws SQLException {
        try (PreparedStatement statement = prepare(connection, sql, timeoutMs, parameters);
             ResultSet result = statement.executeQuery()) {
            require(result.next() && result.getBoolean(1) && !result.next());
        }
    }

    private static PreparedStatement prepare(Connection connection, String sql, int timeoutMs,
            String... parameters) throws SQLException {
        PreparedStatement statement = connection.prepareStatement(sql);
        try {
            statement.setQueryTimeout(timeoutSeconds(timeoutMs));
            statement.setMaxRows(MAX_ITEMS + 1);
            statement.setFetchSize(64);
            for (int i = 0; i < parameters.length; i++) statement.setString(i + 1, parameters[i]);
            return statement;
        } catch (SQLException | RuntimeException failure) {
            statement.close();
            throw failure;
        }
    }

    private static int timeoutSeconds(int timeoutMs) {
        return (timeoutMs + 999) / 1000;
    }

    private static void require(boolean ready) {
        if (!ready) throw new IllegalStateException(FAILURE);
    }
}
