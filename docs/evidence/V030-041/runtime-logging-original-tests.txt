package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.sql.SQLException;
import java.util.concurrent.TimeUnit;
import java.util.logging.Level;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;

/** Root-owned actual subprocess console output, not a mocked logger. */
class RootRuntimeLoggingTest {
    static final String SECRET = "private_business_and_token_marker_41";
    static final String SLF_LOGGER = "org.springframework.transaction.support.TransactionTemplate";
    static final String JUL_LOGGER = "org.postgresql.jdbc.PgConnection";

    String run(String mode) throws Exception {
        var builder = new ProcessBuilder(Path.of(System.getProperty("java.home"), "bin", "java").toString(),
            "-cp", System.getProperty("java.class.path"), Probe.class.getName(), mode);
        builder.environment().clear();
        builder.environment().put("LANG", "C.UTF-8");
        builder.redirectErrorStream(true);
        Process child = builder.start();
        try {
            assertTrue(child.waitFor(10, TimeUnit.SECONDS), "logging initialization must not hang");
            String output = new String(child.getInputStream().readAllBytes(), StandardCharsets.UTF_8);
            assertEquals(0, child.exitValue(), output);
            assertTrue(output.contains("probe-complete"), "child must actually emit all probe events");
            return output;
        } finally {
            if (child.isAlive()) { child.destroyForcibly(); child.waitFor(5, TimeUnit.SECONDS); }
        }
    }
    void noPayload(String output) {
        for (String forbidden : new String[]{SECRET, "INSERT INTO", "<bpmn>", "forged-line", "SQLException", "Caused by:", "private-debug"})
            assertFalse(output.contains(forbidden), "free-form library payload escaped: " + forbidden);
    }
    @Test void slf4jLibraryFailuresKeepSignalWithoutPayloadOrStack() throws Exception {
        String output = run("slf"); noPayload(output);
        assertTrue(output.contains(SLF_LOGGER)); assertTrue(output.contains("ERROR"));
    }
    @Test void julLibraryFailuresUseTheSameSafeBoundary() throws Exception {
        String output = run("jul"); noPayload(output);
        assertTrue(output.contains(JUL_LOGGER)); assertTrue(output.contains("ERROR"));
    }
    @Test void verboseLibraryMessagesAreNotEnabledByDefault() throws Exception {
        String output = run("verbose"); noPayload(output);
        assertFalse(output.contains("org.flowable.verboseprobe"));
        assertTrue(output.contains(SLF_LOGGER)); assertTrue(output.contains("WARN"));
    }
    @Test void repeatedInitializationDoesNotSilenceOrDuplicateWarnings() throws Exception {
        String output = run("repeat"); noPayload(output);
        assertEquals(1, output.split(java.util.regex.Pattern.quote(SLF_LOGGER), -1).length - 1);
        assertEquals(1, output.split(java.util.regex.Pattern.quote(JUL_LOGGER), -1).length - 1);
        assertTrue(output.contains("WARN"));
    }
    public static final class Probe {
        public static void main(String[] args) {
            RuntimeLogging.initialize();
            if (args[0].equals("repeat")) RuntimeLogging.initialize();
            switch (args[0]) {
                case "slf" -> LoggerFactory.getLogger(SLF_LOGGER).error("INSERT INTO accounts password={}\nforged-line", SECRET, new SQLException("<bpmn>" + SECRET + "</bpmn>"));
                case "jul" -> java.util.logging.Logger.getLogger(JUL_LOGGER).log(Level.SEVERE, "INSERT INTO accounts " + SECRET + "\nforged-line", new SQLException("<bpmn>" + SECRET + "</bpmn>"));
                case "verbose" -> {
                    var logger = LoggerFactory.getLogger("org.flowable.verboseprobe");
                    logger.info("private-debug {}", SECRET); logger.debug("private-debug {}", SECRET);
                    LoggerFactory.getLogger(SLF_LOGGER).warn("warning remains observable");
                }
                case "repeat" -> {
                    LoggerFactory.getLogger(SLF_LOGGER).warn("warning remains observable");
                    java.util.logging.Logger.getLogger(JUL_LOGGER).warning("warning remains observable");
                }
                default -> throw new IllegalArgumentException();
            }
            System.out.println("probe-complete");
        }
    }
}
