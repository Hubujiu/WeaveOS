package org.weaveos.proof;

import static org.junit.jupiter.api.Assertions.*;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.flowable.engine.ProcessEngine;
import org.flowable.task.api.Task;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.boot.Banner;
import org.springframework.boot.WebApplicationType;
import org.springframework.boot.builder.SpringApplicationBuilder;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Root-authored engine acceptance; XML is exported by the real Go compiler. */
class GeneratedGraphTest {
    private static final String A = "00000008-0000-4000-8000-000000000008";
    private static final String B = "00000009-0000-4000-8000-000000000009";
    private static final String ASSIGNEES = "a_00000002000040008000000000000002";
    private static final String ROUTE = "route_00000003000040008000000000000003";
    private static final String TRUE_END = "n_00000004000040008000000000000004";
    private static final String FALSE_END = "n_00000005000040008000000000000005";
    private ConfigurableApplicationContext context;
    private JdbcTemplate admin;
    private ProcessEngine engine;
    private String schema;
    private String url;

    @BeforeEach void setup() {
        url = System.getenv("B3_TEST_JDBC_URL");
        assertEquals("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture", url,
            "Only the isolated, unexposed B3 fixture is allowed");
        admin = new JdbcTemplate(new DriverManagerDataSource(url, "b3_fixture", "b3_fixture_only"));
        schema = "v019_" + UUID.randomUUID().toString().replace("-", "");
        admin.execute("CREATE SCHEMA " + schema);
        open();
    }
    private void open() {
        context = new SpringApplicationBuilder(LocalCommandExecutorTest.FixtureConfiguration.class)
            .web(WebApplicationType.NONE).bannerMode(Banner.Mode.OFF).logStartupInfo(false)
            .run("--b3.jdbc-url=" + url + "?currentSchema=" + schema, "--b3.schema=" + schema);
        engine = context.getBean(ProcessEngine.class);
    }
    @AfterEach void cleanup() {
        if (context != null) context.close();
        if (admin != null && schema != null) admin.execute("DROP SCHEMA " + schema + " CASCADE");
    }
    private String start(String mode, boolean includeRoute) {
        String deployment = engine.getRepositoryService().createDeployment()
            .addClasspathResource("v019/generated-" + mode + ".bpmn20.xml").deploy().getId();
        String exactDefinition = engine.getRepositoryService().createProcessDefinitionQuery()
            .deploymentId(deployment).singleResult().getId();
        Map<String,Object> variables = new HashMap<>();
        variables.put(ASSIGNEES, List.of(A, B));
        variables.put("wf_rejected", false);
        if (includeRoute) variables.put(ROUTE, true);
        String process = engine.getRuntimeService().startProcessInstanceById(exactDefinition, variables).getId();
        assertEquals(2, tasks(process).size());
        assertEquals(1, engine.getTaskService().createTaskQuery().processInstanceId(process).taskAssignee(A).count());
        assertEquals(1, engine.getTaskService().createTaskQuery().processInstanceId(process).taskAssignee(B).count());
        return process;
    }
    private List<Task> tasks(String process) {
        return engine.getTaskService().createTaskQuery().processInstanceId(process).list();
    }
    private void complete(String process, String assignee, boolean rejected) {
        Task task = engine.getTaskService().createTaskQuery().processInstanceId(process).taskAssignee(assignee).singleResult();
        assertNotNull(task);
        engine.getTaskService().complete(task.getId(), Map.of("wf_rejected", rejected));
    }
    private void ended(String process, String end) {
        assertEquals(0, engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(process).count());
        assertEquals(0, tasks(process).size());
        assertEquals(1, engine.getHistoryService().createHistoricActivityInstanceQuery()
            .processInstanceId(process).activityId(end).finished().count());
    }
    @Test void allRequiresBothApprovers() {
        String p = start("all", true);
        complete(p, A, false);
        assertEquals(1, tasks(p).size());
        assertEquals(B, tasks(p).get(0).getAssignee());
        assertEquals(1, engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(p).count());
        complete(p, B, false);
        ended(p, TRUE_END);
    }
    @Test void anyFirstApprovalCompletesAndRemovesRemainingTask() {
        String p = start("any", true);
        complete(p, A, false);
        ended(p, TRUE_END);
    }
    @Test void allFirstRejectionTerminatesThisInstance() {
        String p = start("all", true);
        complete(p, B, true);
        ended(p, "reject_end");
    }
    @Test void anyFirstRejectionTerminatesThisInstance() {
        String p = start("any", true);
        complete(p, A, true);
        ended(p, "reject_end");
    }
    @Test void earlierApprovalDoesNotPreventLaterAllModeRejection() {
        String p = start("all", true);
        String first = engine.getTaskService().createTaskQuery().processInstanceId(p).taskAssignee(A).singleResult().getId();
        complete(p, A, false);
        complete(p, B, true);
        ended(p, "reject_end");
        assertEquals(1, engine.getHistoryService().createHistoricTaskInstanceQuery().taskId(first).finished().count());
    }
    @Test void conditionUsesLatestTrustedRouteVariable() {
        String p = start("all", true);
        complete(p, A, false);
        Task last = tasks(p).get(0);
        engine.getTaskService().complete(last.getId(), Map.of("wf_rejected", false, ROUTE, false));
        ended(p, FALSE_END);
    }
    @Test void requiredOuterTransactionRollsBackNodeProgress() {
        String p = start("all", true);
        String first = engine.getTaskService().createTaskQuery().processInstanceId(p).taskAssignee(A).singleResult().getId();
        new TransactionTemplate(context.getBean(PlatformTransactionManager.class)).executeWithoutResult(status -> {
            engine.getTaskService().complete(first, Map.of("wf_rejected", true));
            assertEquals(0, tasks(p).size());
            status.setRollbackOnly();
        });
        assertEquals(2, tasks(p).size());
        assertEquals(1, engine.getTaskService().createTaskQuery().taskId(first).count());
        assertEquals(0, engine.getHistoryService().createHistoricTaskInstanceQuery().taskId(first).finished().count());
        assertEquals(false, engine.getRuntimeService().getVariable(p, "wf_rejected"));
    }
    @Test void partialAllApprovalSurvivesEngineContextRestart() {
        String p = start("all", true);
        complete(p, A, false);
        context.close(); context = null; open();
        assertEquals(1, tasks(p).size());
        assertEquals(B, tasks(p).get(0).getAssignee());
        complete(p, B, false);
        ended(p, TRUE_END);
    }
    @Test void missingRouteCannotSilentlyTakeDefault() {
        String p = start("all", false);
        complete(p, A, false);
        assertThrows(RuntimeException.class, () -> complete(p, B, false));
        assertEquals(1, tasks(p).size());
        assertEquals(B, tasks(p).get(0).getAssignee());
        assertEquals(1, engine.getRuntimeService().createProcessInstanceQuery().processInstanceId(p).count());
    }
}
