package org.weaveos.workflow;

import com.google.common.net.InetAddresses;
import com.zaxxer.hikari.HikariDataSource;
import io.grpc.Context;
import io.grpc.ForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.ServerCall;
import io.grpc.ServerCallExecutorSupplier;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import io.grpc.health.v1.HealthCheckRequest;
import io.grpc.health.v1.HealthCheckResponse;
import io.grpc.health.v1.HealthGrpc;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.StreamObserver;
import java.net.InetSocketAddress;
import java.sql.Connection;
import java.sql.ResultSet;
import java.sql.Statement;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.Executor;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.Semaphore;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import org.flowable.common.engine.impl.history.HistoryLevel;
import org.flowable.engine.ProcessEngine;
import org.flowable.engine.ProcessEngineConfiguration;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;

/** Owns one native engine, its restricted database pool and its authenticated RPC listener. */
public final class WorkflowRuntime implements AutoCloseable {
    private static final String FAILURE = "workflow runtime initialization failed";
    private static final int MAX_MESSAGE = 1024 * 1024 + 16384;
    private static final int MAX_METADATA = 8192;
    private static final Executor DIRECT = Runnable::run;
    private static final Status UNAVAILABLE =
        Status.UNAVAILABLE.withDescription("workflow runtime unavailable");
    private static final Status CAPACITY =
        Status.RESOURCE_EXHAUSTED.withDescription("workflow runtime capacity exhausted");

    private final RuntimeConfiguration configuration;
    private final Dispatcher dispatcher;
    private volatile boolean ready;
    private volatile boolean closing;
    private HikariDataSource pool;
    private SpringProcessEngineConfiguration engineConfiguration;
    private ProcessEngine engine;
    private ThreadPoolExecutor workers;
    private Server server;
    private int boundPort;

    private WorkflowRuntime(RuntimeConfiguration configuration) {
        this.configuration = configuration;
        dispatcher = new Dispatcher(configuration.rpcThreads() + configuration.queueCapacity());
    }

    public static WorkflowRuntime start(RuntimeConfiguration configuration) {
        WorkflowRuntime runtime = null;
        try {
            if (configuration == null) throw new IllegalArgumentException();
            runtime = new WorkflowRuntime(configuration);
            runtime.initialize();
            return runtime;
        } catch (Exception failure) {
            if (runtime != null) quietly(runtime::close);
            // JDBC, bind and cleanup exceptions must not expose their inputs or causes.
            throw new IllegalStateException(FAILURE);
        }
    }

    private void initialize() throws Exception {
        pool = RuntimeDataSource.open(configuration);
        RuntimeSchema.verify(pool, configuration.schema(), configuration.statementTimeoutMs());
        var transactionManager = new DataSourceTransactionManager(pool);
        engineConfiguration = new SpringProcessEngineConfiguration();
        engineConfiguration.setDataSource(pool);
        engineConfiguration.setTransactionManager(transactionManager);
        engineConfiguration.setDatabaseSchema(configuration.schema());
        engineConfiguration.setDatabaseSchemaUpdate(ProcessEngineConfiguration.DB_SCHEMA_UPDATE_FALSE);
        engineConfiguration.setDisableIdmEngine(true);
        engineConfiguration.setDisableEventRegistry(true);
        engineConfiguration.setAsyncExecutorActivate(false);
        engineConfiguration.setAsyncHistoryExecutorActivate(false);
        engineConfiguration.setAsyncHistoryEnabled(false);
        engineConfiguration.setHistoryLevel(HistoryLevel.AUDIT);
        engineConfiguration.setEnableHistoryCleaning(false);
        engine = engineConfiguration.buildProcessEngine();

        var jdbc = new JdbcTemplate(pool);
        var deployments = new DeploymentRegistry(jdbc, transactionManager, engine);
        var executions = new ExecutionRegistry(jdbc, transactionManager, engine);
        var threadNumber = new AtomicInteger();
        workers = new ThreadPoolExecutor(configuration.rpcThreads(), configuration.rpcThreads(),
            0, TimeUnit.MILLISECONDS, new ArrayBlockingQueue<>(configuration.queueCapacity()),
            task -> new Thread(task, "workflow-rpc-" + threadNumber.incrementAndGet()),
            new ThreadPoolExecutor.AbortPolicy());
        workers.prestartAllCoreThreads();

        server = NettyServerBuilder.forAddress(new InetSocketAddress(
                InetAddresses.forString(configuration.bindHost()), configuration.port()))
            // With callExecutor, only bounded admission/method lookup and cancellation run here.
            .executor(DIRECT)
            .callExecutor(dispatcher)
            .maxInboundMessageSize(MAX_MESSAGE)
            .maxInboundMetadataSize(MAX_METADATA)
            .intercept(dispatcher)
            .intercept(configuration.serviceIdentity())
            .addService(new DeploymentGrpcService(deployments))
            .addService(new ExecutionGrpcService(executions))
            .addService(new DatabaseHealth())
            .build();
        server.start();
        boundPort = server.getPort();
        synchronized (dispatcher) {
            ready = true;
        }
    }

    public int port() { return boundPort; }

    public void awaitTermination() throws InterruptedException {
        server.awaitTermination();
        close();
    }

    @Override public synchronized void close() {
        if (closing) return;
        synchronized (dispatcher) {
            closing = true;
            ready = false;
        }
        boolean interrupted = false;
        boolean graceful = false;
        try {
            if (server != null) {
                server.shutdown();
                graceful = server.awaitTermination(configuration.shutdownTimeoutMs(), TimeUnit.MILLISECONDS);
            }
        } catch (InterruptedException interruption) {
            interrupted = true;
        } catch (RuntimeException ignored) {
            // Continue closing every owned resource, without retaining failure details.
        }

        if (!graceful) {
            dispatcher.forceStop = true;
            quietly(() -> { if (server != null) server.shutdownNow(); });
        }
        if (workers != null) {
            if (graceful) quietly(workers::shutdown);
            else quietly(this::stopWorkers);
        }
        quietly(() -> {
            if (engine != null) engine.close();
            else if (engineConfiguration != null) engineConfiguration.close();
        });
        quietly(() -> { if (pool != null) pool.close(); });

        try {
            if (workers != null && !workers.awaitTermination(
                    configuration.shutdownTimeoutMs(), TimeUnit.MILLISECONDS)) {
                dispatcher.forceStop = true;
                quietly(this::stopWorkers);
            }
            if (server != null && !server.isTerminated()) {
                server.awaitTermination(configuration.shutdownTimeoutMs(), TimeUnit.MILLISECONDS);
            }
        } catch (InterruptedException interruption) {
            interrupted = true;
            dispatcher.forceStop = true;
            quietly(() -> { if (server != null) server.shutdownNow(); });
            if (workers != null) quietly(this::stopWorkers);
        } finally {
            if (interrupted) Thread.currentThread().interrupt();
        }
    }

    private void stopWorkers() {
        for (Runnable task : workers.shutdownNow()) {
            if (task instanceof DispatchTask pending) quietly(pending::cancelAndDrain);
        }
    }

    private static void quietly(Runnable cleanup) {
        try { cleanup.run(); } catch (RuntimeException ignored) { }
    }

    private final class DatabaseHealth extends HealthGrpc.HealthImplBase {
        @Override public void check(HealthCheckRequest request, StreamObserver<HealthCheckResponse> output) {
            String name = request.getService();
            if (!name.isEmpty() && !name.equals("weaveos.workflow.v1.DeploymentService")
                    && !name.equals("weaveos.workflow.v1.ExecutionService")) {
                output.onError(Status.NOT_FOUND.withDescription("unknown workflow service").asRuntimeException());
                return;
            }
            boolean healthy = false;
            if (ready && !closing) {
                try (Connection connection = pool.getConnection(); Statement query = connection.createStatement()) {
                    int timeout = Math.min(1000, configuration.statementTimeoutMs());
                    query.setQueryTimeout((timeout + 999) / 1000);
                    query.setMaxRows(1);
                    try (ResultSet result = query.executeQuery("SELECT 1")) {
                        healthy = result.next() && result.getInt(1) == 1;
                    }
                } catch (Exception unavailable) {
                    // A fresh failed borrow/query means not serving; no driver detail leaves Health.
                }
            }
            output.onNext(HealthCheckResponse.newBuilder().setStatus(healthy && ready && !closing
                ? HealthCheckResponse.ServingStatus.SERVING
                : HealthCheckResponse.ServingStatus.NOT_SERVING).build());
            output.onCompleted();
        }
    }

    /**
     * A permit covers the entire call, including time awaiting its unary request.
     * All registered handlers consume a single request: generated startCall and
     * terminal callbacks only manage transport state; service/JDBC work starts at
     * onHalfClose. The services install no terminal application callbacks.
     * Before any direct cleanup run, CallExecution.stopped is set. This interceptor
     * then prevents handler creation or forwards none of onMessage/onHalfClose/onReady.
     * Thus a rejected/cancelled generic gRPC Runnable cannot reach business on I/O.
     */
    private final class Dispatcher implements ServerCallExecutorSupplier, ServerInterceptor {
        private final Semaphore permits;
        private final ThreadLocal<CallExecution> current = new ThreadLocal<>();
        private volatile boolean forceStop;

        private Dispatcher(int capacity) { permits = new Semaphore(capacity); }

        @Override public synchronized <Q, S> Executor getExecutor(ServerCall<Q, S> call, Metadata headers) {
            if (!ready || closing) throw UNAVAILABLE.asRuntimeException();
            if (!permits.tryAcquire()) throw CAPACITY.asRuntimeException();
            var execution = new CallExecution(call, Context.current());
            try {
                execution.context.addListener(context -> execution.release(), DIRECT);
                return execution;
            } catch (RuntimeException failure) {
                execution.release();
                throw UNAVAILABLE.asRuntimeException();
            }
        }

        @Override public <Q, S> ServerCall.Listener<Q> interceptCall(
                ServerCall<Q, S> call, Metadata headers, ServerCallHandler<Q, S> next) {
            CallExecution execution = current.get();
            if (execution == null || !execution.businessAllowed()) return new ServerCall.Listener<>() {};
            var listener = next.startCall(call, headers);
            return new ForwardingServerCallListener.SimpleForwardingServerCallListener<Q>(listener) {
                @Override public void onMessage(Q message) {
                    if (execution.businessAllowed()) super.onMessage(message);
                }
                @Override public void onHalfClose() {
                    if (execution.businessAllowed()) super.onHalfClose();
                }
                @Override public void onReady() {
                    if (execution.businessAllowed()) super.onReady();
                }
            };
        }

        private final class CallExecution implements Executor {
            private final ServerCall<?, ?> call;
            private final Context context;
            private final AtomicBoolean released = new AtomicBoolean();
            private volatile boolean stopped;

            private CallExecution(ServerCall<?, ?> call, Context context) {
                this.call = call;
                this.context = context;
            }

            private boolean businessAllowed() { return !stopped && !forceStop && !context.isCancelled(); }

            @Override public void execute(Runnable command) {
                if (!businessAllowed()) {
                    stopped = true;
                    run(command);
                    return;
                }
                try {
                    workers.execute(new DispatchTask(this, command));
                } catch (RejectedExecutionException rejected) {
                    stop(workers.isShutdown() ? UNAVAILABLE : CAPACITY);
                    // Never use caller-runs for business. The guard now suppresses every
                    // business callback; direct execution only drains gRPC lifecycle cleanup.
                    run(command);
                }
            }

            private void run(Runnable command) {
                context.run(() -> {
                    CallExecution previous = current.get();
                    current.set(this);
                    try { command.run(); }
                    finally {
                        if (previous == null) current.remove();
                        else current.set(previous);
                    }
                });
            }

            private void stop(Status status) {
                stopped = true;
                quietly(() -> call.close(status, new Metadata()));
            }

            private void release() {
                if (released.compareAndSet(false, true)) permits.release();
            }
        }
    }

    private final class DispatchTask implements Runnable {
        private final Dispatcher.CallExecution execution;
        private final Runnable command;

        private DispatchTask(Dispatcher.CallExecution execution, Runnable command) {
            this.execution = execution;
            this.command = command;
        }
        @Override public void run() { execution.run(command); }
        private void cancelAndDrain() {
            execution.stop(UNAVAILABLE);
            execution.run(command);
        }
    }
}
