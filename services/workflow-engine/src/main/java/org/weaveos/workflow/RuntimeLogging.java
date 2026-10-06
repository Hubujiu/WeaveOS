package org.weaveos.workflow;

import ch.qos.logback.classic.LoggerContext;
import ch.qos.logback.classic.joran.JoranConfigurator;
import org.slf4j.LoggerFactory;
import org.slf4j.bridge.SLF4JBridgeHandler;

/** A single safe diagnostic boundary; business/audit logs remain separate data. */
public final class RuntimeLogging {
    private static boolean initialized;
    private RuntimeLogging() {}
    public static synchronized void initialize() {
        if (initialized) return;
        try {
            var resource = RuntimeLogging.class.getResource("/logback.xml");
            if (resource == null) throw new IllegalStateException();
            var context = (LoggerContext) LoggerFactory.getILoggerFactory();
            context.reset();
            var configuration = new JoranConfigurator();
            configuration.setContext(context);
            configuration.doConfigure(resource);
            SLF4JBridgeHandler.removeHandlersForRootLogger();
            java.util.logging.Logger.getLogger("").setLevel(java.util.logging.Level.WARNING);
            SLF4JBridgeHandler.install();
            initialized = true;
        } catch (Exception failure) {
            throw new IllegalStateException("workflow logging initialization failed");
        }
    }
}
