package org.weaveos.workflow;

import com.zaxxer.hikari.HikariDataSource;

/** Declaration-only test-first placeholder; no listener or service wiring. */
public final class RuntimeDataSource {
    private RuntimeDataSource() {}
    public static HikariDataSource open(RuntimeConfiguration configuration) { return null; }
}
