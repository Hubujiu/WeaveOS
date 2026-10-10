package org.weaveos.workflow;

import java.time.Instant;
import java.util.Optional;
import java.util.function.Consumer;
import org.flowable.engine.ProcessEngine;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;

/** Compile-only R1 declaration. No deletion behavior implemented yet. */
public final class FlowDeletionRegistry {
 public record Request(String appId, String flowId, String operationId) {}
 public record Receipt(String appId, String flowId, String operationId, long deletedVersions, Instant deletedAt) {}
 public enum Stage { AFTER_CHECK, AFTER_DELETION, AFTER_RECEIPT }
 public static final class Conflict extends RuntimeException {}
 public static final class NotDrained extends RuntimeException {}
 public FlowDeletionRegistry(JdbcTemplate jdbc, PlatformTransactionManager manager, ProcessEngine engine) {}
 public Receipt delete(Request request) { return delete(request, stage -> {}); }
 public Receipt delete(Request request, Consumer<Stage> probe) { return null; }
 public Optional<Receipt> lookup(Request request) { return Optional.empty(); }
}
