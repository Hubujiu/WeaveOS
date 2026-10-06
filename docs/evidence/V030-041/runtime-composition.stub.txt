package org.weaveos.workflow;

/** Declaration-only composition placeholder for Root-owned real transport tests. */
public final class WorkflowRuntime implements AutoCloseable {
    private WorkflowRuntime() {}
    public static WorkflowRuntime start(RuntimeConfiguration configuration) { return null; }
    public int port() { return 0; }
    public void awaitTermination() throws InterruptedException {}
    @Override public void close() {}
}
