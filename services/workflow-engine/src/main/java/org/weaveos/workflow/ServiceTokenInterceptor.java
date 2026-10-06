package org.weaveos.workflow;

import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;

/** Process-lifetime service identity; authentication precedes business dispatch. */
public final class ServiceTokenInterceptor implements ServerInterceptor {
    private static final Metadata.Key<String> AUTHORIZATION =
        Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);
    private static final String SCHEME = "Bearer ";
    private final byte[] tokenDigest;

    public ServiceTokenInterceptor(String token) {
        if (!validToken(token)) {
            throw new IllegalArgumentException("invalid service token configuration");
        }
        tokenDigest = digest(token);
    }

    @Override
    public <Q, S> ServerCall.Listener<Q> interceptCall(
        ServerCall<Q, S> call, Metadata headers, ServerCallHandler<Q, S> next) {
        var values = headers == null ? null : headers.getAll(AUTHORIZATION);
        if (values == null) return reject(call);
        var iterator = values.iterator();
        if (!iterator.hasNext()) return reject(call);
        String authorization = iterator.next();
        if (iterator.hasNext() || authorization == null
            || authorization.length() < SCHEME.length() + 32
            || authorization.length() > SCHEME.length() + 256
            || !authorization.startsWith(SCHEME)) {
            return reject(call);
        }

        String token = authorization.substring(SCHEME.length());
        if (!validToken(token) || !MessageDigest.isEqual(tokenDigest, digest(token))) {
            return reject(call);
        }
        return next.startCall(call, headers);
    }

    private static boolean validToken(String token) {
        if (token == null || token.length() < 32 || token.length() > 256) return false;
        for (int i = 0; i < token.length(); i++) {
            char c = token.charAt(i);
            if (!(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z')
                && !(c >= '0' && c <= '9') && c != '_' && c != '-') {
                return false;
            }
        }
        return true;
    }

    private static byte[] digest(String token) {
        byte[] bytes = token.getBytes(StandardCharsets.US_ASCII);
        try {
            return MessageDigest.getInstance("SHA-256").digest(bytes);
        } catch (NoSuchAlgorithmException unavailable) {
            throw new IllegalStateException("service identity digest unavailable");
        } finally {
            Arrays.fill(bytes, (byte) 0);
        }
    }

    private static <Q, S> ServerCall.Listener<Q> reject(ServerCall<Q, S> call) {
        call.close(Status.UNAUTHENTICATED.withDescription("unauthenticated service"), new Metadata());
        return new ServerCall.Listener<>() {};
    }

    @Override
    public String toString() {
        return "[REDACTED service identity]";
    }
}
