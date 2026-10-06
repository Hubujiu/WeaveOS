package org.weaveos.workflow;

/** Starts the configured runtime and owns its process-lifetime cleanup. */
public final class WorkflowEngineMain {
    private WorkflowEngineMain() {}

    public static void main(String[] args) {
        Thread shutdownHook = null;
        boolean failed = false;
        try {
            if (args.length != 0) throw new IllegalArgumentException();
            try (WorkflowRuntime runtime = WorkflowRuntime.start(
                    RuntimeConfiguration.read(System.getenv()))) {
                shutdownHook = new Thread(runtime::close, "workflow-runtime-shutdown");
                Runtime.getRuntime().addShutdownHook(shutdownHook);
                runtime.awaitTermination();
            }
        } catch (InterruptedException interruption) {
            Thread.currentThread().interrupt();
            failed = true;
        } catch (Exception failure) {
            failed = true;
        } finally {
            if (shutdownHook != null) {
                try {
                    Runtime.getRuntime().removeShutdownHook(shutdownHook);
                } catch (IllegalStateException ignored) {
                    // Shutdown already started; the registered hook owns cleanup too.
                }
            }
        }
        if (failed) {
            System.err.println("workflow runtime initialization failed");
            System.exit(1);
        }
    }
}
