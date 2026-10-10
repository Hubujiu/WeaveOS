package org.weaveos.workflow;

import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.Objects;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.weaveos.workflow.v1.*;

/** Deployment transport only; authentication and server lifecycle belong to the caller. */
public final class DeploymentGrpcService extends DeploymentServiceGrpc.DeploymentServiceImplBase {
    private static final int MAX_XML = 1024 * 1024;
    private static final int MAX_MESSAGE = MAX_XML + 16384;
    private final DeploymentRegistry registry;
    private final FlowDeletionRegistry deletion;
    public DeploymentGrpcService(DeploymentRegistry registry) {
        this.registry = Objects.requireNonNull(registry); this.deletion = null;
    }
    public DeploymentGrpcService(DeploymentRegistry registry, FlowDeletionRegistry deletion) {
        this.registry = Objects.requireNonNull(registry); this.deletion = Objects.requireNonNull(deletion);
    }

    @Override public void deleteFlow(FlowDeletionRequest request, StreamObserver<FlowDeletionReceipt> output) {
        if (outsideTransaction(output)) return;
        FlowDeletionReceipt response;
        try {
            var identity = deletionIdentity(request);
            if (deletion == null) throw new IllegalStateException();
            response = deletionWire(deletion.delete(identity));
        } catch (IllegalArgumentException invalid) {
            fail(output, Status.INVALID_ARGUMENT, "invalid flow deletion request"); return;
        } catch (FlowDeletionRegistry.Conflict conflict) {
            fail(output, Status.ALREADY_EXISTS, "flow deletion identity conflict"); return;
        } catch (FlowDeletionRegistry.NotDrained active) {
            fail(output, Status.FAILED_PRECONDITION, "flow is not drained"); return;
        } catch (RuntimeException unexpected) {
            fail(output, Status.UNAVAILABLE, "flow deletion result unavailable"); return;
        }
        output.onNext(response); output.onCompleted();
    }

    @Override public void lookupFlowDeletion(FlowDeletionRequest request,
            StreamObserver<FlowDeletionLookupResponse> output) {
        if (outsideTransaction(output)) return;
        FlowDeletionLookupResponse response;
        try {
            var identity = deletionIdentity(request);
            if (deletion == null) throw new IllegalStateException();
            var found = deletion.lookup(identity);
            response = found.isPresent()
                ? FlowDeletionLookupResponse.newBuilder().setConfirmed(deletionWire(found.get())).build()
                : FlowDeletionLookupResponse.newBuilder().setNotObserved(NotObserved.getDefaultInstance()).build();
        } catch (IllegalArgumentException invalid) {
            fail(output, Status.INVALID_ARGUMENT, "invalid flow deletion lookup"); return;
        } catch (RuntimeException unexpected) {
            fail(output, Status.UNAVAILABLE, "flow deletion result unavailable"); return;
        }
        output.onNext(response); output.onCompleted();
    }

    private static FlowDeletionRegistry.Request deletionIdentity(FlowDeletionRequest request) {
        if (request == null || request.getSerializedSize() > 1024) throw new IllegalArgumentException();
        ControlledBpmn.requireUuid(request.getAppId()); ControlledBpmn.requireUuid(request.getFlowId());
        ControlledBpmn.requireUuid(request.getOperationId());
        return new FlowDeletionRegistry.Request(request.getAppId(), request.getFlowId(), request.getOperationId());
    }
    private static FlowDeletionReceipt deletionWire(FlowDeletionRegistry.Receipt receipt) {
        long seconds = receipt.deletedAt().getEpochSecond(); int nanos = receipt.deletedAt().getNano();
        if (receipt.deletedVersions() < 0 || receipt.deletedVersions() > 9007199254740991L
            || seconds < 1 || seconds > 253402300799L || nanos < 0 || nanos > 999999999 || nanos % 1000 != 0)
            throw new IllegalStateException();
        return FlowDeletionReceipt.newBuilder().setAppId(receipt.appId()).setFlowId(receipt.flowId())
            .setOperationId(receipt.operationId()).setDeletedVersions(receipt.deletedVersions())
            .setDeletedAtSeconds(seconds).setDeletedAtNanos(nanos).build();
    }

    @Override public void deploy(DeployRequest request, StreamObserver<DeploymentReceipt> output) {
        if (outsideTransaction(output)) return;
        DeploymentRegistry.Receipt receipt;
        try {
            if (request == null || request.getSerializedSize() > MAX_MESSAGE
                || request.getBpmnXml().isEmpty() || request.getBpmnXml().size() > MAX_XML
                || !request.getBpmnXml().isValidUtf8()) throw new DeploymentRegistry.InvalidDeployment();
            identity(request.getAppId(), request.getFlowId(), request.getVersionId(), request.getVersion());
            // Strict UTF-8 validation precedes decoding; preserve whitespace and exact bytes.
            receipt = registry.deploy(new DeploymentRegistry.Request(request.getAppId(), request.getFlowId(),
                request.getVersionId(), request.getVersion(), request.getBpmnXml().toStringUtf8()));
        } catch (DeploymentRegistry.InvalidDeployment invalid) {
            fail(output, Status.INVALID_ARGUMENT, "invalid deployment request"); return;
        } catch (DeploymentRegistry.DeploymentConflict conflict) {
            fail(output, Status.ALREADY_EXISTS, "deployment identity conflict"); return;
        } catch (RuntimeException unexpected) {
            fail(output, Status.UNAVAILABLE, "deployment result unavailable"); return;
        }
        // No enclosing transaction can make this provisional; registry.execute has committed.
        // Delivery failure cannot undo that commit; recover using the original identity.
        output.onNext(wire(receipt)); output.onCompleted();
    }

    @Override public void lookup(LookupRequest request, StreamObserver<LookupResponse> output) {
        if (outsideTransaction(output)) return;
        LookupResponse response;
        try {
            if (request == null || request.getSerializedSize() > MAX_MESSAGE
                || !request.getBpmnSha256().matches("[0-9a-f]{64}")) throw new DeploymentRegistry.InvalidDeployment();
            identity(request.getAppId(), request.getFlowId(), request.getVersionId(), request.getVersion());
            var found = registry.lookup(request.getVersionId());
            if (found.isEmpty()) {
                response = LookupResponse.newBuilder().setNotObserved(NotObserved.getDefaultInstance()).build();
            } else {
                var receipt = found.get();
                if (!receipt.appId().equals(request.getAppId()) || !receipt.flowId().equals(request.getFlowId())
                    || receipt.version() != request.getVersion() || !receipt.versionId().equals(request.getVersionId())
                    || !receipt.bpmnSha256().equals(request.getBpmnSha256())) {
                    fail(output, Status.FAILED_PRECONDITION, "deployment context mismatch"); return;
                }
                response = LookupResponse.newBuilder().setConfirmed(wire(receipt)).build();
            }
        } catch (DeploymentRegistry.InvalidDeployment invalid) {
            fail(output, Status.INVALID_ARGUMENT, "invalid deployment lookup"); return;
        } catch (RuntimeException unexpected) {
            fail(output, Status.UNAVAILABLE, "deployment result unavailable"); return;
        }
        output.onNext(response); output.onCompleted();
    }

    private static boolean outsideTransaction(StreamObserver<?> output) {
        if (!TransactionSynchronizationManager.isActualTransactionActive()) return false;
        fail(output, Status.FAILED_PRECONDITION, "deployment RPC requires its own commit boundary"); return true;
    }
    private static void identity(String app, String flow, String versionId, long version) {
        ControlledBpmn.requireUuid(app); ControlledBpmn.requireUuid(flow); ControlledBpmn.requireUuid(versionId);
        if (version < 1 || version > 9007199254740991L) throw new DeploymentRegistry.InvalidDeployment();
    }
    private static DeploymentReceipt wire(DeploymentRegistry.Receipt receipt) {
        return DeploymentReceipt.newBuilder().setAppId(receipt.appId()).setFlowId(receipt.flowId())
            .setVersionId(receipt.versionId()).setVersion(receipt.version()).setBpmnSha256(receipt.bpmnSha256())
            .setEngineDeploymentId(receipt.engineDeploymentId()).setProcessDefinitionId(receipt.processDefinitionId()).build();
    }
    private static void fail(StreamObserver<?> output, Status status, String description) {
        output.onError(status.withDescription(description).asRuntimeException());
    }
}
