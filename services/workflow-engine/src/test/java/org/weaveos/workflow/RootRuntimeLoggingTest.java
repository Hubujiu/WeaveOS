package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.sql.SQLException;
import java.util.concurrent.TimeUnit;
import java.util.List;
import java.util.ArrayList;
import java.util.regex.Pattern;
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
    void exactSignals(String output,List<String> expected) {
        var safe=Pattern.compile("^\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2},\\d{3} (WARN|ERROR) +([A-Za-z0-9_.$]+) event=runtime_library_log$");
        var events=new ArrayList<String>();int completed=0;
        for(String line:output.lines().toList()) {
            if(line.equals("probe-complete")){completed++;continue;}
            var match=safe.matcher(line);
            assertTrue(match.matches(),"every line must be a bounded diagnostic record: "+line);
            events.add(match.group(1)+" "+match.group(2));
        }
        assertEquals(1,completed);assertEquals(expected,events);
    }
    @Test void slf4jLibraryFailuresKeepSignalWithoutPayloadOrStack() throws Exception {
        String output = run("slf"); noPayload(output);
        exactSignals(output,List.of("ERROR "+SLF_LOGGER));
    }
    @Test void julLibraryFailuresUseTheSameSafeBoundary() throws Exception {
        String output = run("jul"); noPayload(output);
        exactSignals(output,List.of("ERROR "+JUL_LOGGER));
    }
    @Test void verboseLibraryMessagesAreNotEnabledByDefault() throws Exception {
        String output = run("verbose"); noPayload(output);
        exactSignals(output,List.of("WARN "+SLF_LOGGER));
    }
    @Test void repeatedInitializationDoesNotSilenceOrDuplicateWarnings() throws Exception {
        String output = run("repeat"); noPayload(output);
        exactSignals(output,List.of("WARN "+SLF_LOGGER,"WARN "+JUL_LOGGER));
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
