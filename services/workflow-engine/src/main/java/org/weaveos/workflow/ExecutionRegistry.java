package org.weaveos.workflow;

import java.util.List;
import java.util.Optional;
import java.util.function.Consumer;
import org.flowable.engine.ProcessEngine;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;

/** Root declaration draft. No execution behavior has been implemented or authorized here. */
public final class ExecutionRegistry {
    public record Request(byte[] commandBytes, byte[] payloadBytes) {}
    public record Task(String id, String nodeId, String assigneeId, String engineTaskId, long activationEpoch) {}
    public record Result(String instanceId, String engineProcessId, String state, String reason,
                         long schemaVersion, long recordVersion, List<Task> tasks) {}
    public record Receipt(String commandId, String commandHash, String outcome, long sequence,
                          String proofId, String resultHash, Result result) {}
    public enum Stage { AFTER_LEDGER, AFTER_ENGINE, AFTER_RECEIPT }
    public static final class InvalidCommand extends IllegalArgumentException {
        public InvalidCommand() { super("invalid execution command"); }
    }
    public static final class CommandConflict extends RuntimeException {
        public CommandConflict() { super("execution command identity conflict"); }
    }
    public ExecutionRegistry(JdbcTemplate jdbc, PlatformTransactionManager manager, ProcessEngine engine) {}
    public Receipt execute(Request request) { throw new UnsupportedOperationException("not implemented"); }
    public Receipt execute(Request request, Consumer<Stage> probe) { throw new UnsupportedOperationException("not implemented"); }
    public Optional<Receipt> lookup(String commandId, String commandHash) { return Optional.empty(); }
    public Receipt establishNoEffect(Request request) { throw new UnsupportedOperationException("not implemented"); }
}
