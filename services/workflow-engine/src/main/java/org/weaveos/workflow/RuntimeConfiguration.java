package org.weaveos.workflow;

import com.google.common.net.InetAddresses;
import java.net.URI;
import java.net.URISyntaxException;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

/** Validated process-lifetime configuration. Parsing performs no network operations. */
public final class RuntimeConfiguration {
    private static final String PREFIX = "WEAVEOS_ENGINE_";
    private static final String INVALID = "invalid workflow runtime configuration";
    private static final Set<String> OPTIONS = Set.of(
        "JDBC_URL", "DB_USER", "DB_PASSWORD", "SERVICE_TOKEN", "SCHEMA", "BIND_HOST", "PORT",
        "POOL_MAX", "RPC_THREADS", "QUEUE_CAPACITY", "CONNECTION_TIMEOUT_MS",
        "STATEMENT_TIMEOUT_MS", "LOCK_TIMEOUT_MS", "SHUTDOWN_TIMEOUT_MS");

    private final String jdbcUrl;
    private final String dbUser;
    private final String dbPassword;
    private final String schema;
    private final String bindHost;
    private final int port;
    private final int poolMax;
    private final int rpcThreads;
    private final int queueCapacity;
    private final int connectionTimeoutMs;
    private final int statementTimeoutMs;
    private final int lockTimeoutMs;
    private final int shutdownTimeoutMs;
    private final ServiceTokenInterceptor serviceIdentity;

    private RuntimeConfiguration(Map<String, String> values) {
        for (String key : values.keySet()) {
            if (key == null || key.startsWith(PREFIX) && !OPTIONS.contains(key.substring(PREFIX.length()))) {
                throw invalid();
            }
        }
        jdbcUrl = required(values, "JDBC_URL");
        validateJdbc(jdbcUrl);
        dbUser = required(values, "DB_USER");
        dbPassword = required(values, "DB_PASSWORD");
        serviceIdentity = new ServiceTokenInterceptor(required(values, "SERVICE_TOKEN"));
        schema = option(values, "SCHEMA", "workflow");
        if (!schema.matches("[a-z_][a-z0-9_]{0,62}") || schema.startsWith("pg_")) throw invalid();
        bindHost = option(values, "BIND_HOST", "127.0.0.1");
        byte[] bind = ipLiteral(bindHost);
        if (bind == null || !privateOrLoopback(bind) && !allZero(bind)) throw invalid();
        port = number(values, "PORT", 50051, 1, 65535);
        poolMax = number(values, "POOL_MAX", 8, 1, 16);
        rpcThreads = number(values, "RPC_THREADS", 4, 1, 16);
        queueCapacity = number(values, "QUEUE_CAPACITY", 32, 1, 128);
        connectionTimeoutMs = number(values, "CONNECTION_TIMEOUT_MS", 3000, 250, 30000);
        statementTimeoutMs = number(values, "STATEMENT_TIMEOUT_MS", 10000, 1, 30000);
        lockTimeoutMs = number(values, "LOCK_TIMEOUT_MS", 1000, 1, statementTimeoutMs);
        shutdownTimeoutMs = number(values, "SHUTDOWN_TIMEOUT_MS", 10000, 100, 30000);
    }

    public static RuntimeConfiguration read(Map<String, String> environment) {
        try {
            return new RuntimeConfiguration(new HashMap<>(environment));
        } catch (RuntimeException failure) {
            // Do not preserve a cause whose message may contain a URL, password or token.
            throw invalid();
        }
    }

    private static IllegalArgumentException invalid() {
        return new IllegalArgumentException(INVALID);
    }

    private static String required(Map<String, String> values, String name) {
        String value = values.get(PREFIX + name);
        if (value == null || value.isBlank()) throw invalid();
        return value;
    }

    private static String option(Map<String, String> values, String name, String fallback) {
        return values.containsKey(PREFIX + name) ? required(values, name) : fallback;
    }

    private static int number(Map<String, String> values, String name, int fallback, int min, int max) {
        String raw = option(values, name, Integer.toString(fallback));
        if (!raw.matches("[0-9]+")) throw invalid();
        int value = Integer.parseInt(raw);
        if (value < min || value > max) throw invalid();
        return value;
    }

    private static void validateJdbc(String value) {
        if (!value.startsWith("jdbc:postgresql://")) throw invalid();
        final URI uri;
        try {
            uri = new URI(value.substring(5));
        } catch (URISyntaxException failure) {
            throw invalid();
        }
        String host = uri.getHost();
        String path = uri.getPath();
        String authority = uri.getRawAuthority();
        if (!"postgresql".equals(uri.getScheme()) || uri.getRawUserInfo() != null
            || uri.getRawQuery() != null || uri.getRawFragment() != null || host == null
            || path == null || !path.startsWith("/") || path.length() < 2
            || path.substring(1).contains("/") || path.substring(1).isBlank()
            || authority == null || uri.getPort() < 1 || uri.getPort() > 65535) throw invalid();
        String rawPort = authority.substring(authority.lastIndexOf(':') + 1);
        if (!rawPort.matches("[0-9]+")) throw invalid();
        if (host.startsWith("[") && host.endsWith("]")) host = host.substring(1, host.length() - 1);
        byte[] address = ipLiteral(host);
        if (address != null) {
            if (!privateOrLoopback(address)) throw invalid();
        } else if (host.indexOf(':') >= 0 || host.matches("[0-9.]+") || !dnsName(host)) {
            throw invalid();
        }
    }

    // DNS validation is syntax-only; it does not establish private routing.
    private static boolean dnsName(String host) {
        if (host.endsWith(".")) host = host.substring(0, host.length() - 1);
        if (host.isEmpty() || host.length() > 253) return false;
        for (String label : host.split("\\.", -1)) {
            if (label.isEmpty() || label.length() > 63
                || !label.matches("[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?")) return false;
        }
        return true;
    }

    // Guava parses numeric literals without DNS. Scoped literals are excluded
    // so no network-interface lookup is needed either.
    private static byte[] ipLiteral(String host) {
        if (host.indexOf('%') >= 0 || !InetAddresses.isInetAddress(host)) return null;
        return InetAddresses.forString(host).getAddress();
    }

    private static boolean privateOrLoopback(byte[] address) {
        if (address.length == 4) {
            int first = address[0] & 255;
            int second = address[1] & 255;
            return first == 127 || first == 10 || first == 172 && second >= 16 && second <= 31
                || first == 192 && second == 168;
        }
        boolean mapped = true;
        for (int i = 0; i < 10; i++) mapped &= address[i] == 0;
        mapped &= (address[10] & 255) == 255 && (address[11] & 255) == 255;
        if (mapped) return privateOrLoopback(new byte[]{address[12], address[13], address[14], address[15]});
        boolean loopback = address[15] == 1;
        for (int i = 0; i < 15; i++) loopback &= address[i] == 0;
        return loopback || (address[0] & 254) == 252
            || (address[0] & 255) == 254 && (address[1] & 192) == 192;
    }

    private static boolean allZero(byte[] address) {
        for (byte value : address) if (value != 0) return false;
        return true;
    }

    public String jdbcUrl() { return jdbcUrl; }
    public String dbUser() { return dbUser; }
    public String dbPassword() { return dbPassword; }
    public String schema() { return schema; }
    public String bindHost() { return bindHost; }
    public int port() { return port; }
    public int poolMax() { return poolMax; }
    public int rpcThreads() { return rpcThreads; }
    public int queueCapacity() { return queueCapacity; }
    public int connectionTimeoutMs() { return connectionTimeoutMs; }
    public int statementTimeoutMs() { return statementTimeoutMs; }
    public int lockTimeoutMs() { return lockTimeoutMs; }
    public int shutdownTimeoutMs() { return shutdownTimeoutMs; }
    public ServiceTokenInterceptor serviceIdentity() { return serviceIdentity; }

    @Override public String toString() { return "[REDACTED workflow runtime configuration]"; }
}
