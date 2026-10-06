package org.weaveos.workflow;

import com.zaxxer.hikari.HikariDataSource;
import java.sql.Connection;

/** Bounded, closeable PostgreSQL pool; no listener, migration or service wiring. */
public final class RuntimeDataSource {
    private static final String FAILURE = "workflow database initialization failed";
    private static final String POOL_NAME = "workflow-runtime";

    private RuntimeDataSource() {}

    public static HikariDataSource open(RuntimeConfiguration configuration) {
        HikariDataSource pool = null;
        try {
            if (configuration == null) throw new IllegalStateException(FAILURE);

            pool = new HikariDataSource();
            pool.setPoolName(POOL_NAME);
            pool.setJdbcUrl(configuration.jdbcUrl());
            pool.setUsername(configuration.dbUser());
            pool.setPassword(configuration.dbPassword());
            pool.setMaximumPoolSize(configuration.poolMax());
            pool.setMinimumIdle(0);
            pool.setConnectionTimeout(configuration.connectionTimeoutMs());
            pool.setValidationTimeout(Math.min(5000, configuration.connectionTimeoutMs()));
            pool.setAutoCommit(true);
            pool.setReadOnly(false);
            pool.setTransactionIsolation("TRANSACTION_READ_COMMITTED");

            pool.addDataSourceProperty("currentSchema", configuration.schema());
            pool.addDataSourceProperty("options",
                "-c statement_timeout=" + configuration.statementTimeoutMs()
                    + " -c lock_timeout=" + configuration.lockTimeoutMs());
            pool.addDataSourceProperty("tcpKeepAlive", "true");

            // pgJDBC uses whole seconds. Keep transport waits finite, allowing
            // the server's statement timeout to report cancellation first.
            int connectSeconds = seconds(configuration.connectionTimeoutMs());
            int socketSeconds = seconds(Math.max(configuration.connectionTimeoutMs(),
                configuration.statementTimeoutMs())) + 1;
            pool.addDataSourceProperty("connectTimeout", Integer.toString(connectSeconds));
            pool.addDataSourceProperty("loginTimeout", Integer.toString(connectSeconds));
            pool.addDataSourceProperty("socketTimeout", Integer.toString(socketSeconds));
            pool.addDataSourceProperty("cancelSignalTimeout", Integer.toString(connectSeconds));
            pool.addDataSourceProperty("logServerErrorDetail", "false");

            // Start without Hikari's separate synchronous seeding loop. The
            // explicit borrow is bounded by connectionTimeout and validates
            // credentials before this factory can return a usable pool.
            pool.setInitializationFailTimeout(-1);
            try (Connection connection = pool.getConnection()) {
                // Hikari validates and initializes each new physical connection.
            }
            return pool;
        } catch (Exception failure) {
            if (pool != null) {
                try {
                    pool.close();
                } catch (RuntimeException ignored) {
                    // Cleanup failures must not leak their cause or SQL details.
                }
            }
            throw new IllegalStateException(FAILURE);
        }
    }

    private static int seconds(int milliseconds) {
        return (milliseconds + 999) / 1000;
    }
}
