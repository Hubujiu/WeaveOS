package org.weaveos.workflow;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/** Immutable v2 command identity. Decoding neither authorizes nor executes a command. */
public final class CommandEnvelope {
    private static final int MAX_BYTES = 538;
    private static final int TRAILER_BYTES = 6 * Long.BYTES + 32;
    private static final long MAX_SAFE_INTEGER = 9007199254740991L;
    private static final byte[] PREFIX = {0x57, 0x56, 0x46, 0x43, 0x4d, 0x44, 0, 2};

    private final String[] fields;
    private final long[] integers;
    private final byte[] payloadHash;
    private final byte[] canonicalBytes;
    private final byte[] fingerprint;

    private CommandEnvelope(String[] fields, long[] integers, byte[] payloadHash, byte[] canonicalBytes) {
        // All arrays are allocated by decode and are never exposed by reference.
        this.fields = fields;
        this.integers = integers;
        this.payloadHash = payloadHash;
        this.canonicalBytes = canonicalBytes;
        try {
            this.fingerprint = MessageDigest.getInstance("SHA-256").digest(canonicalBytes);
        } catch (NoSuchAlgorithmException impossible) {
            throw new IllegalStateException("SHA-256 unavailable");
        }
    }

    public static CommandEnvelope decode(byte[] bytes) {
        if (bytes == null || bytes.length < PREFIX.length + 12 * Integer.BYTES + TRAILER_BYTES
                || bytes.length > MAX_BYTES) throw invalid();
        // Bound the copy first, then parse and hash the same independently owned bytes.
        byte[] canonical = bytes.clone();
        ByteBuffer input = ByteBuffer.wrap(canonical);
        for (byte expected : PREFIX) if (input.get() != expected) throw invalid();

        String[] fields = new String[12];
        for (int index = 0; index < fields.length; index++) {
            if (input.remaining() < Integer.BYTES) throw invalid();
            long length = Integer.toUnsignedLong(input.getInt());
            boolean optionalUuid = index == 8 || index == 11;
            if (index == 10) {
                if (length < 1 || length > 8) throw invalid();
            } else if (length != 36 && !(optionalUuid && length == 0)) {
                throw invalid();
            }
            // Reject unsigned lengths before conversion, allocation or reading any text.
            int size = (int) length;
            int remainingHeaders = (fields.length - index - 1) * Integer.BYTES;
            if (input.remaining() < size + remainingHeaders + TRAILER_BYTES) throw invalid();
            int offset = input.position();
            // Legal identifiers/actions are ASCII, a strict subset of UTF-8. No replacement decoding.
            for (int end = offset + size; input.position() < end;) {
                if (input.get() < 0) throw invalid();
            }
            fields[index] = new String(canonical, offset, size, StandardCharsets.US_ASCII);
            if (index != 10 && size != 0 && !isCanonicalNonzeroUuid(fields[index])) throw invalid();
        }
        if (input.remaining() != TRAILER_BYTES) throw invalid();

        long[] integers = new long[6];
        for (int index = 0; index < integers.length; index++) {
            long value = input.getLong();
            // uint64 values with their high bit set become negative and are rejected here too.
            if (value < (index < 4 ? 1 : 0) || value > MAX_SAFE_INTEGER) throw invalid();
            integers[index] = value;
        }
        switch (fields[10]) {
            case "start":
                if (integers[5] != 0) throw invalid();
                // Both start and withdraw are instance commands without a task or target.
            case "withdraw":
                if (!fields[8].isEmpty() || integers[4] != 0 || !fields[11].isEmpty()) throw invalid();
                break;
            case "agree":
            case "reject":
                if (fields[8].isEmpty() || integers[4] == 0 || !fields[11].isEmpty()) throw invalid();
                break;
            case "return":
                if (fields[8].isEmpty() || integers[4] == 0 || fields[11].isEmpty()) throw invalid();
                break;
            default:
                throw invalid();
        }

        byte[] payload = new byte[32];
        input.get(payload);
        boolean nonzero = false;
        for (byte value : payload) nonzero |= value != 0;
        if (!nonzero) throw invalid();
        return new CommandEnvelope(fields, integers, payload, canonical);
    }

    private static boolean isCanonicalNonzeroUuid(String value) {
        boolean nonzero = false;
        for (int index = 0; index < 36; index++) {
            char character = value.charAt(index);
            if (index == 8 || index == 13 || index == 18 || index == 23) {
                if (character != '-') return false;
            } else if (character >= '1' && character <= '9' || character >= 'a' && character <= 'f') {
                nonzero = true;
            } else if (character != '0') {
                return false;
            }
        }
        return nonzero;
    }

    private static IllegalArgumentException invalid() {
        return new IllegalArgumentException("invalid command envelope");
    }

    public int protocolVersion() { return 2; }
    public String commandId() { return fields[0]; }
    public String appId() { return fields[1]; }
    public String tableId() { return fields[2]; }
    public String viewId() { return fields[3]; }
    public String recordId() { return fields[4]; }
    public String flowId() { return fields[5]; }
    public String versionId() { return fields[6]; }
    public String instanceId() { return fields[7]; }
    public String taskId() { return fields[8]; }
    public String actorId() { return fields[9]; }
    public String action() { return fields[10]; }
    public String targetNodeId() { return fields[11]; }
    public long definitionVersion() { return integers[0]; }
    public long schemaVersion() { return integers[1]; }
    public long recordVersion() { return integers[2]; }
    public long fenceEpoch() { return integers[3]; }
    public long taskEpoch() { return integers[4]; }
    public long expectedSequence() { return integers[5]; }
    public byte[] payloadHash() { return payloadHash.clone(); }
    public byte[] canonicalBytes() { return canonicalBytes.clone(); }
    public byte[] fingerprint() { return fingerprint.clone(); }
}
