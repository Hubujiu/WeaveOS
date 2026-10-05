package org.weaveos.workflow;

/** Root declaration only, intentionally inherits UNIMPLEMENTED responses. */
public final class ExecutionGrpcService extends org.weaveos.workflow.v1.ExecutionServiceGrpc.ExecutionServiceImplBase {
 public ExecutionGrpcService(ExecutionRegistry registry) { java.util.Objects.requireNonNull(registry); }
}
