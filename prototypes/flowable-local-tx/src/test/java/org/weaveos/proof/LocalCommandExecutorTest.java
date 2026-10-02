package org.weaveos.proof;

import static org.junit.jupiter.api.Assertions.*;

import java.sql.Connection;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import javax.sql.DataSource;
import org.flowable.engine.ProcessEngine;
import org.flowable.engine.ProcessEngineConfiguration;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.flowable.task.api.Task;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;
import org.postgresql.PGConnection;
import org.springframework.boot.Banner;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.WebApplicationType;
import org.springframework.boot.builder.SpringApplicationBuilder;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.context.annotation.Bean;
import org.springframework.core.env.Environment;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DataSourceUtils;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.TransactionAwareDataSourceProxy;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;
import org.weaveos.proof.LocalCommandExecutor.Command;
import org.weaveos.proof.LocalCommandExecutor.Receipt;
import org.weaveos.proof.LocalCommandExecutor.Stage;

class LocalCommandExecutorTest {
    private ConfigurableApplicationContext context;
    private JdbcTemplate admin;
    private JdbcTemplate jdbc;
    private ProcessEngine engine;
    private LocalCommandExecutor executor;
    private String schema;
    private String url;
    private String processId;
    private String taskId;

    @BeforeEach
    void createIsolatedFixture() {
        url = System.getenv("B3_TEST_JDBC_URL");
        assertEquals("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture", url,
            "Run through run-proof.sh; this test only accepts its dedicated unexposed fixture database");
        admin = new JdbcTemplate(new DriverManagerDataSource(url, "b3_fixture", "b3_fixture_only"));
        schema = "b3_" + UUID.randomUUID().toString().replace("-", "");
        admin.execute("CREATE SCHEMA " + schema);
        openContext();
        jdbc.execute("CREATE TABLE b3_command_ledger (command_id varchar(255) PRIMARY KEY, task_id varchar(255) NOT NULL, needs_review boolean NOT NULL, payload text NOT NULL)");
        jdbc.execute("CREATE TABLE b3_result_outbox (command_id varchar(255) PRIMARY KEY REFERENCES b3_command_ledger(command_id), process_instance_id varchar(255) NOT NULL, completed_task_id varchar(255) NOT NULL, next_task_id varchar(255), next_task_key varchar(255), ended boolean NOT NULL)");
        String definitionId = engine.getRepositoryService().createDeployment()
            .addClasspathResource("controlled-approval.bpmn20.xml").deploy().getId();
        definitionId = engine.getRepositoryService().createProcessDefinitionQuery()
            .deploymentId(definitionId).singleResult().getId();
        processId = engine.getRuntimeService().startProcessInstanceById(definitionId).getId();
        taskId = engine.getTaskService().createTaskQuery().processInstanceId(processId).singleResult().getId();
    }

    private void openContext() {
        context = new SpringApplicationBuilder(FixtureConfiguration.class)
            .web(WebApplicationType.NONE).bannerMode(Banner.Mode.OFF).logStartupInfo(false)
            .run("--b3.jdbc-url=" + url + "?currentSchema=" + schema, "--b3.schema=" + schema);
        jdbc = context.getBean(JdbcTemplate.class);
        engine = context.getBean(ProcessEngine.class);
        executor = context.getBean(LocalCommandExecutor.class);
    }

    @AfterEach
    void removeIsolatedFixture() {
        if (context != null) context.close();
        if (admin != null && schema != null) admin.execute("DROP SCHEMA " + schema + " CASCADE");
    }

    @Test
    void bootAndEngineUseTheSameUnderlyingDataSourceAndTransactionManager() {
        assertEquals("4.0.2", org.springframework.boot.SpringBootVersion.getVersion());
        assertEquals("7.0.3", org.springframework.core.SpringVersion.getVersion());
        SpringProcessEngineConfiguration config = (SpringProcessEngineConfiguration) engine.getProcessEngineConfiguration();
        assertSame(context.getBean(PlatformTransactionManager.class), config.getTransactionManager());
        assertSame(context.getBean(DataSource.class), ((TransactionAwareDataSourceProxy) config.getDataSource()).getTargetDataSource());
        assertEquals("REQUIRED", config.getDefaultCommandConfig().getTransactionPropagation().name());
        transaction().executeWithoutResult(status -> {
            assertTrue(TransactionSynchronizationManager.isActualTransactionActive());
            Connection application = DataSourceUtils.getConnection(context.getBean(DataSource.class));
            try (Connection flowable = config.getDataSource().getConnection()) {
                assertSame(application.unwrap(PGConnection.class), flowable.unwrap(PGConnection.class));
            } catch (java.sql.SQLException error) { throw new AssertionError(error); }
        });
    }

    @Test
    void completeCommitsLedgerEngineAndOutboxTogether() {
        Receipt receipt = executor.complete(command("command-success", true, "fixture-payload"));
        assertReceipt(receipt, true);
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
        assertNull(engine.getTaskService().createTaskQuery().taskId(taskId).singleResult());
        assertEquals("review", engine.getTaskService().createTaskQuery().taskId(receipt.nextTaskId()).singleResult().getTaskDefinitionKey());
    }

    @Test
    void pureDefaultConditionEndsTheProcess() {
        Receipt receipt = executor.complete(command("command-end", false, "fixture-payload"));
        assertReceipt(receipt, false);
        assertNull(engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(processId).singleResult());
        assertEquals(1, engine.getHistoryService().createHistoricProcessInstanceQuery().processInstanceId(processId).finished().count());
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
    }

    @ParameterizedTest(name = "rollback all stores at {0}")
    @EnumSource(Stage.class)
    void failureBeforeCommitRollsBackAllThreeStores(Stage failurePoint) {
        assertThrows(InjectedFailure.class, () -> executor.complete(command("command-failure", true, "fixture-payload"), stage -> {
            if (stage == failurePoint) throw new InjectedFailure();
        }));
        assertRolledBack();
    }

    @Test
    void completeJoinsAnOuterRequiredTransaction() {
        transaction().executeWithoutResult(status -> {
            Receipt receipt = executor.complete(command("outer-required", true, "fixture-payload"));
            assertReceipt(receipt, true);
            assertEquals(1, count("b3_command_ledger"));
            assertEquals(1, count("b3_result_outbox"));
            status.setRollbackOnly();
        });
        assertRolledBack();
    }

    @Test
    void duplicateCommandReplaysExactReceiptWithoutCompletingNextTask() {
        Command command = command("same-id", true, "same-payload");
        Receipt original = executor.complete(command);
        assertReceipt(original, true);
        assertEquals(original, executor.complete(command));
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
        assertEquals(1, engine.getTaskService().createTaskQuery().taskId(original.nextTaskId()).count());
    }

    @Test
    void sameIdWithDifferentPayloadIsAConflict() {
        Receipt original = executor.complete(command("conflict-id", true, "payload-A"));
        assertReceipt(original, true);
        assertThrows(LocalCommandExecutor.PayloadConflict.class,
            () -> executor.complete(command("conflict-id", true, "payload-B")));
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
        assertEquals(1, engine.getTaskService().createTaskQuery().taskId(original.nextTaskId()).count());
    }

    @Test
    void sameIdCannotChangeTaskOrConditionDespiteIdenticalOpaquePayload() {
        Receipt original = executor.complete(command("structural-conflict", true, "opaque"));
        assertReceipt(original, true);
        assertThrows(LocalCommandExecutor.PayloadConflict.class,
            () -> executor.complete(command("structural-conflict", false, "opaque")));
        assertThrows(LocalCommandExecutor.PayloadConflict.class,
            () -> executor.complete(new Command("structural-conflict", original.nextTaskId(), true, "opaque")));
    }

    @Test
    void simultaneousIdenticalCommandsAdvanceOnce() throws Exception {
        Command command = command("concurrent-same", true, "payload");
        List<Object> results = race(command, command);
        assertInstanceOf(Receipt.class, results.get(0));
        assertInstanceOf(Receipt.class, results.get(1));
        assertReceipt((Receipt) results.get(0), true);
        assertEquals(results.get(0), results.get(1));
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
        assertEquals(1, engine.getTaskService().createTaskQuery().processInstanceId(processId).count());
    }

    @Test
    void simultaneousConflictingCommandsHaveOneWinnerAndOneConflict() throws Exception {
        List<Object> results = race(command("concurrent-conflict", true, "A"), command("concurrent-conflict", true, "B"));
        assertEquals(1, results.stream().filter(Receipt.class::isInstance).count());
        assertEquals(1, results.stream().filter(LocalCommandExecutor.PayloadConflict.class::isInstance).count());
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
        assertEquals(1, engine.getTaskService().createTaskQuery().processInstanceId(processId).count());
    }

    @Test
    void responseLossAfterCommitReplaysAfterEngineRestart() {
        Command command = command("lost-response", true, "payload");
        Receipt[] committed = new Receipt[1];
        assertThrows(ResponseLost.class, () -> {
            committed[0] = executor.complete(command);
            assertReceipt(committed[0], true);
            throw new ResponseLost(); // Transport failure after complete returned and its transaction committed.
        });
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
        assertNull(engine.getTaskService().createTaskQuery().taskId(taskId).singleResult());
        context.close();
        context = null;
        openContext();
        assertEquals(committed[0], executor.complete(command));
        assertEquals(1, engine.getTaskService().createTaskQuery().taskId(committed[0].nextTaskId()).count());
        assertEquals(1, count("b3_result_outbox"));
    }

    @Test
    void rolledBackCommandIdCanBeRetried() {
        Command command = command("retry-after-rollback", true, "payload");
        assertThrows(InjectedFailure.class, () -> executor.complete(command, stage -> {
            if (stage == Stage.AFTER_ENGINE) throw new InjectedFailure();
        }));
        assertRolledBack();
        assertReceipt(executor.complete(command), true);
        assertEquals(1, count("b3_command_ledger"));
        assertEquals(1, count("b3_result_outbox"));
    }

    @Test
    void missingTaskDoesNotInventSuccessOrPersistAReceipt() {
        assertThrows(LocalCommandExecutor.TaskUnavailable.class,
            () -> executor.complete(new Command("missing-task", "does-not-exist", true, "payload")));
        assertRolledBack();
    }

    private List<Object> race(Command first, Command second) throws Exception {
        var workers = Executors.newFixedThreadPool(2);
        var ready = new CountDownLatch(2);
        var start = new CountDownLatch(1);
        try {
            var futures = List.of(first, second).stream().map(command -> workers.submit(() -> {
                ready.countDown();
                if (!start.await(30, TimeUnit.SECONDS)) throw new AssertionError("race start timeout");
                try { return (Object) executor.complete(command); }
                catch (LocalCommandExecutor.PayloadConflict conflict) { return conflict; }
            })).toList();
            assertTrue(ready.await(30, TimeUnit.SECONDS));
            start.countDown();
            return java.util.Arrays.asList(futures.get(0).get(30, TimeUnit.SECONDS), futures.get(1).get(30, TimeUnit.SECONDS));
        } finally {
            start.countDown();
            workers.shutdownNow();
            assertTrue(workers.awaitTermination(30, TimeUnit.SECONDS));
        }
    }

    private Command command(String id, boolean needsReview, String payload) {
        return new Command(id, taskId, needsReview, payload);
    }

    private void assertReceipt(Receipt receipt, boolean needsReview) {
        assertNotNull(receipt, "command must return a persisted engine receipt");
        assertEquals(processId, receipt.processInstanceId());
        assertEquals(taskId, receipt.completedTaskId());
        assertEquals(!needsReview, receipt.ended());
        if (needsReview) {
            assertNotNull(receipt.nextTaskId());
            assertEquals("review", receipt.nextTaskKey());
        } else {
            assertNull(receipt.nextTaskId());
            assertNull(receipt.nextTaskKey());
        }
    }

    private void assertRolledBack() {
        assertEquals(0, count("b3_command_ledger"));
        assertEquals(0, count("b3_result_outbox"));
        Task task = engine.getTaskService().createTaskQuery().taskId(taskId).singleResult();
        assertNotNull(task, "original task must survive rollback");
        assertEquals("approval", task.getTaskDefinitionKey());
        assertEquals(1, engine.getTaskService().createTaskQuery().processInstanceId(processId).count());
        assertEquals(0, engine.getHistoryService().createHistoricTaskInstanceQuery().taskId(taskId).finished().count());
        assertNotNull(engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(processId).singleResult());
        assertEquals(0, jdbc.queryForObject("SELECT count(*) FROM ACT_RU_VARIABLE WHERE NAME_ = 'needsReview'", Long.class));
    }

    private long count(String table) {
        return jdbc.queryForObject("SELECT count(*) FROM " + table, Long.class);
    }

    private TransactionTemplate transaction() {
        TransactionTemplate transaction = new TransactionTemplate(context.getBean(PlatformTransactionManager.class));
        transaction.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRED);
        return transaction;
    }

    private static final class InjectedFailure extends RuntimeException {}
    private static final class ResponseLost extends RuntimeException {}

    @SpringBootConfiguration
    static class FixtureConfiguration {
        @Bean DataSource dataSource(Environment environment) {
            return new DriverManagerDataSource(environment.getRequiredProperty("b3.jdbc-url"), "b3_fixture", "b3_fixture_only");
        }
        @Bean PlatformTransactionManager transactionManager(DataSource dataSource) {
            return new DataSourceTransactionManager(dataSource);
        }
        @Bean JdbcTemplate jdbcTemplate(DataSource dataSource) { return new JdbcTemplate(dataSource); }
        @Bean(destroyMethod = "close") ProcessEngine processEngine(DataSource dataSource,
                                                                   PlatformTransactionManager transactionManager,
                                                                   Environment environment) {
            SpringProcessEngineConfiguration configuration = new SpringProcessEngineConfiguration();
            configuration.setDataSource(dataSource);
            configuration.setTransactionManager(transactionManager);
            configuration.setDatabaseSchema(environment.getRequiredProperty("b3.schema"));
            // Only this freshly created, random test schema; no production migration or configuration.
            configuration.setDatabaseSchemaUpdate(ProcessEngineConfiguration.DB_SCHEMA_UPDATE_TRUE);
            configuration.setAsyncExecutorActivate(false);
            configuration.setDisableIdmEngine(true);
            configuration.setDisableEventRegistry(true);
            return configuration.buildProcessEngine();
        }
        @Bean LocalCommandExecutor commandExecutor(JdbcTemplate jdbc, PlatformTransactionManager transactionManager, ProcessEngine engine) {
            return new LocalCommandExecutor(jdbc, transactionManager, engine);
        }
    }
}
