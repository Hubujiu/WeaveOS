package org.weaveos.workflow;

import com.google.protobuf.ByteString;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.Objects;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.weaveos.workflow.v1.*;

/** Execution transport only; the caller owns authentication and server lifecycle. */
public final class ExecutionGrpcService extends ExecutionServiceGrpc.ExecutionServiceImplBase {
    private static final int REQUEST_LIMIT = 270336;
    private static final int LOOKUP_LIMIT = 1024;
    private static final int RECEIPT_LIMIT = 66560;
    private final ExecutionRegistry registry;

    public ExecutionGrpcService(ExecutionRegistry registry) { this.registry = Objects.requireNonNull(registry); }

    @Override public void execute(ExecutionRequest request, StreamObserver<ExecutionReceipt> output) {
        execute(request, output, false);
    }
    @Override public void establishNoEffect(ExecutionRequest request, StreamObserver<ExecutionReceipt> output) {
        execute(request, output, true);
    }
    private void execute(ExecutionRequest request, StreamObserver<ExecutionReceipt> output, boolean cancel) {
        if (outsideTransaction(output)) return;
        ExecutionReceipt response;
        try {
            if (request == null || request.getSerializedSize() > REQUEST_LIMIT
                || request.getCommandEnvelope().isEmpty() || request.getCommandEnvelope().size() > 538
                || request.getPayload().isEmpty() || request.getPayload().size() > 262144)
                throw new ExecutionRegistry.InvalidCommand();
            var input = new ExecutionRegistry.Request(request.getCommandEnvelope().toByteArray(), request.getPayload().toByteArray());
            // Registry validates exact command/payload framing before entering its own transaction.
            response = wire(cancel ? registry.establishNoEffect(input) : registry.execute(input));
        } catch (ExecutionRegistry.InvalidCommand invalid) {
            fail(output, Status.INVALID_ARGUMENT, "invalid execution request"); return;
        } catch (ExecutionRegistry.CommandConflict conflict) {
            fail(output, Status.ALREADY_EXISTS, "execution identity conflict"); return;
        } catch (RuntimeException unexpected) {
            fail(output, Status.UNAVAILABLE, "execution result unavailable"); return;
        }
        // Registry has committed. A delivery error cannot roll back or reclassify that result.
        output.onNext(response); output.onCompleted();
    }
    @Override public void lookup(ExecutionLookupRequest request, StreamObserver<ExecutionLookupResponse> output) {
        if (outsideTransaction(output)) return;
        ExecutionLookupResponse response;
        try {
            if (request == null || request.getSerializedSize() > LOOKUP_LIMIT || request.getCommandHash().size() != 32)
                throw new ExecutionRegistry.InvalidCommand();
            ExecutionCodec.uuid(request.getCommandId());
            var found = registry.lookup(request.getCommandId(), ExecutionCodec.hex(request.getCommandHash().toByteArray()));
            response = found.isPresent()
                ? ExecutionLookupResponse.newBuilder().setConfirmed(wire(found.get())).build()
                : ExecutionLookupResponse.newBuilder().setNotObserved(NotObserved.getDefaultInstance()).build();
            if (response.getSerializedSize() > RECEIPT_LIMIT) throw new IllegalStateException("oversized receipt");
        } catch (ExecutionRegistry.InvalidCommand invalid) {
            fail(output, Status.INVALID_ARGUMENT, "invalid execution lookup"); return;
        } catch (ExecutionRegistry.CommandConflict conflict) {
            fail(output, Status.FAILED_PRECONDITION, "execution identity conflict"); return;
        } catch (RuntimeException unexpected) {
            fail(output, Status.UNAVAILABLE, "execution result unavailable"); return;
        }
        output.onNext(response); output.onCompleted();
    }
    private static boolean outsideTransaction(StreamObserver<?> output) {
        if (!TransactionSynchronizationManager.isActualTransactionActive()) return false;
        fail(output, Status.FAILED_PRECONDITION, "execution RPC requires its own commit boundary"); return true;
    }
    private static byte[] hash(String value) {
        if (value == null || !value.matches("[0-9a-f]{64}")) throw new IllegalStateException("invalid durable hash");
        return HexFormat.of().parseHex(value);
    }
    private static ExecutionReceipt wire(ExecutionRegistry.Receipt receipt) {
        byte[] bytes = ExecutionCodec.encode(receipt.result());
        byte[] resultHash = hash(receipt.resultHash());
        if (!Arrays.equals(ExecutionCodec.hash(bytes), resultHash)) throw new IllegalStateException("durable result mismatch");
        if (receipt.sequence() < 0 || receipt.sequence() > ExecutionCodec.MAX) throw new IllegalStateException("invalid durable sequence");
        var response = ExecutionReceipt.newBuilder().setCommandId(receipt.commandId())
            .setCommandHash(ByteString.copyFrom(hash(receipt.commandHash()))).setOutcome(receipt.outcome())
            .setSequence(receipt.sequence()).setProofId(receipt.proofId())
            .setResultHash(ByteString.copyFrom(resultHash)).setResultBytes(ByteString.copyFrom(bytes)).build();
        if (response.getSerializedSize() > RECEIPT_LIMIT) throw new IllegalStateException("oversized receipt");
        return response;
    }
    private static void fail(StreamObserver<?> output, Status status, String description) {
        output.onError(status.withDescription(description).asRuntimeException());
    }
}
