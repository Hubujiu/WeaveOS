package org.weaveos.proof;

import java.util.function.Consumer;
import org.flowable.engine.ProcessEngine;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;

/** Test-only protocol experiment; no product API or authorization semantics. */
public final class LocalCommandExecutor {
    public record Command(String id, String taskId, boolean needsReview, String payload) {}
    public record Receipt(String commandId, String processInstanceId, String completedTaskId,
                          String nextTaskId, String nextTaskKey, boolean ended) {}
    public enum Stage { AFTER_LEDGER, AFTER_ENGINE, AFTER_OUTBOX }
    public static final class PayloadConflict extends RuntimeException {
        public PayloadConflict() { super("fixture command payload conflict"); }
    }
    public static final class TaskUnavailable extends RuntimeException {
        public TaskUnavailable() { super("fixture task unavailable; outcome is not proven successful"); }
    }
    public LocalCommandExecutor(JdbcTemplate jdbc, PlatformTransactionManager transactionManager,
                                ProcessEngine engine) {}
    public Receipt complete(Command command) { return complete(command, stage -> {}); }
    public Receipt complete(Command command, Consumer<Stage> probe) {
        // RED placeholder: declarations only, no command behavior.
        return null;
    }
}
