package org.weaveos.proof;

import java.util.function.Consumer;
import java.util.Map;
import org.flowable.engine.ProcessEngine;
import org.flowable.task.api.Task;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;

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
    private final JdbcTemplate jdbc;
    private final ProcessEngine engine;
    private final TransactionTemplate transaction;

    public LocalCommandExecutor(JdbcTemplate jdbc, PlatformTransactionManager transactionManager,
                                ProcessEngine engine) {
        this.jdbc = jdbc;
        this.engine = engine;
        this.transaction = new TransactionTemplate(transactionManager);
        this.transaction.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRED);
    }
    public Receipt complete(Command command) { return complete(command, stage -> {}); }
    public Receipt complete(Command command, Consumer<Stage> probe) {
        return transaction.execute(status -> {
            // PostgreSQL's unique-key conflict wait serializes equal IDs until the owner commits/rolls back.
            int inserted = jdbc.update("""
                INSERT INTO b3_command_ledger(command_id, task_id, needs_review, payload)
                VALUES (?, ?, ?, ?) ON CONFLICT (command_id) DO NOTHING
                """, command.id(), command.taskId(), command.needsReview(), command.payload());
            if (inserted == 0) {
                Command stored = jdbc.queryForObject("""
                    SELECT command_id, task_id, needs_review, payload FROM b3_command_ledger
                    WHERE command_id = ?
                    """, (row, index) -> new Command(row.getString("command_id"), row.getString("task_id"),
                        row.getBoolean("needs_review"), row.getString("payload")), command.id());
                if (!command.equals(stored)) throw new PayloadConflict();
                // Replay durable evidence before looking up the now-completed task.
                return jdbc.queryForObject("SELECT * FROM b3_result_outbox WHERE command_id = ?",
                    (row, index) -> new Receipt(row.getString("command_id"), row.getString("process_instance_id"),
                        row.getString("completed_task_id"), row.getString("next_task_id"),
                        row.getString("next_task_key"), row.getBoolean("ended")), command.id());
            }
            probe.accept(Stage.AFTER_LEDGER);
            Task task = engine.getTaskService().createTaskQuery().taskId(command.taskId()).singleResult();
            if (task == null) throw new TaskUnavailable();
            String processId = task.getProcessInstanceId();
            engine.getTaskService().complete(task.getId(), Map.of("needsReview", command.needsReview()));
            probe.accept(Stage.AFTER_ENGINE);
            Task next = engine.getTaskService().createTaskQuery().processInstanceId(processId).singleResult();
            boolean ended = engine.getRuntimeService().createProcessInstanceQuery()
                .processInstanceId(processId).singleResult() == null;
            Receipt receipt = new Receipt(command.id(), processId, task.getId(), next == null ? null : next.getId(),
                next == null ? null : next.getTaskDefinitionKey(), ended);
            // Write before the outer transaction commits, never from an afterCommit callback.
            jdbc.update("""
                INSERT INTO b3_result_outbox(command_id, process_instance_id, completed_task_id,
                    next_task_id, next_task_key, ended) VALUES (?, ?, ?, ?, ?, ?)
                """, receipt.commandId(), receipt.processInstanceId(), receipt.completedTaskId(),
                receipt.nextTaskId(), receipt.nextTaskKey(), receipt.ended());
            probe.accept(Stage.AFTER_OUTBOX);
            return receipt;
        });
    }
}
