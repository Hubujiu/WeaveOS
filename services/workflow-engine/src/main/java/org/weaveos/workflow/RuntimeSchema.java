package org.weaveos.workflow;

import javax.sql.DataSource;

/** Declaration-only test-first placeholder. Not wired into a service. */
public final class RuntimeSchema {
    private RuntimeSchema() {}
    public static void verify(DataSource source, String schema, int timeoutMs) {}
}
