package org.weaveos.workflow;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Consumer;
import javax.sql.DataSource;
import org.flowable.engine.ProcessEngine;
import org.flowable.spring.SpringProcessEngineConfiguration;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.TransactionAwareDataSourceProxy;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;

/** Trusted service-internal port. Not an authorization or public RPC endpoint.
 * A returned receipt is provisional until any enclosing REQUIRED transaction commits.
 */
public final class DeploymentRegistry {
    public record Request(String appId, String flowId, String versionId, long version, String bpmnXml) {}
    public record Receipt(String appId, String flowId, String versionId, long version, String bpmnSha256,
                          String engineDeploymentId, String processDefinitionId) {}
    public enum Stage { AFTER_LEDGER, AFTER_ENGINE, AFTER_RECEIPT }
    public static final class InvalidDeployment extends IllegalArgumentException {
        public InvalidDeployment() { super("invalid controlled deployment"); }
    }
    public static final class DeploymentConflict extends RuntimeException {
        public DeploymentConflict() { super("deployment identity conflict"); }
    }
    private final JdbcTemplate jdbc;
    private final ProcessEngine engine;
    private final TransactionTemplate transaction;
    private static final RowMapper<Receipt> RECEIPT = (row, index) -> new Receipt(
        row.getString("app_id"), row.getString("flow_id"), row.getString("version_id"), row.getLong("version"),
        row.getString("bpmn_sha256"), row.getString("engine_deployment_id"), row.getString("process_definition_id"));

    public DeploymentRegistry(JdbcTemplate jdbc, PlatformTransactionManager transactionManager, ProcessEngine engine) {
        this.jdbc = Objects.requireNonNull(jdbc);
        this.engine = Objects.requireNonNull(engine);
        // Fail closed on wiring that could commit the engine independently of the ledger.
        if (!(engine.getProcessEngineConfiguration() instanceof SpringProcessEngineConfiguration configuration)
            || configuration.getTransactionManager() != transactionManager
            || !(transactionManager instanceof DataSourceTransactionManager manager)
            || underlying(jdbc.getDataSource()) != underlying(manager.getDataSource())
            || underlying(jdbc.getDataSource()) != underlying(configuration.getDataSource())) {
            throw new IllegalArgumentException("registry and engine require the same Spring JDBC transaction boundary");
        }
        transaction = new TransactionTemplate(transactionManager);
        transaction.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRED);
    }

    private static DataSource underlying(DataSource source) {
        while (source instanceof TransactionAwareDataSourceProxy proxy) source = proxy.getTargetDataSource();
        return Objects.requireNonNull(source);
    }

    public Receipt deploy(Request request) { return deploy(request, stage -> {}); }

    public Receipt deploy(Request request, Consumer<Stage> probe) {
        Objects.requireNonNull(probe);
        if (request == null) throw new InvalidDeployment();
        ControlledBpmn.requireUuid(request.appId());
        ControlledBpmn.requireUuid(request.flowId());
        ControlledBpmn.requireUuid(request.versionId());
        if (request.version() < 1 || request.version() > 9007199254740991L) throw new InvalidDeployment();
        var validated = ControlledBpmn.validate(request.bpmnXml(), request.versionId(), request.appId(), request.flowId());
        byte[] bytes = validated.bytes();
        String hash = sha256(bytes);
        return transaction.execute(status -> {
            // Both unique keys arbitrate before deployment. DO NOTHING avoids aborting a PG transaction
            // on a logical-version conflict; concurrent equal keys wait for the owner's commit/rollback.
            int inserted = jdbc.update("""
                INSERT INTO wf_deployments(version_id, app_id, flow_id, version, bpmn_sha256, status)
                VALUES (?::uuid, ?::uuid, ?::uuid, ?, ?, 'pending') ON CONFLICT DO NOTHING
                """, request.versionId(), request.appId(), request.flowId(), request.version(), hash);
            if (inserted == 0) {
                Receipt prior = lookup(request.versionId()).orElseThrow(DeploymentConflict::new);
                if (!prior.appId().equals(request.appId()) || !prior.flowId().equals(request.flowId())
                    || prior.version() != request.version() || !prior.bpmnSha256().equals(hash)) {
                    throw new DeploymentConflict();
                }
                return prior;
            }
            if (FlowIdentityGate.lock(jdbc, request.appId(), request.flowId())) throw new DeploymentConflict();
            probe.accept(Stage.AFTER_LEDGER);
            var repository = engine.getRepositoryService();
            var deployment = repository.createDeployment().name(request.versionId())
                .addBytes(request.versionId() + ".bpmn20.xml", bytes).deploy();
            var definition = repository.createProcessDefinitionQuery().deploymentId(deployment.getId()).singleResult();
            if (definition == null || !definition.getKey().equals(validated.processKey())) {
                throw new IllegalStateException("controlled process definition missing");
            }
            probe.accept(Stage.AFTER_ENGINE);
            Receipt receipt = new Receipt(request.appId(), request.flowId(), request.versionId(), request.version(),
                hash, deployment.getId(), definition.getId());
            int updated = jdbc.update("""
                UPDATE wf_deployments SET status='confirmed', engine_deployment_id=?, process_definition_id=?
                WHERE version_id=?::uuid AND status='pending'
                """, receipt.engineDeploymentId(), receipt.processDefinitionId(), receipt.versionId());
            if (updated != 1) throw new IllegalStateException("deployment receipt was not recorded");
            probe.accept(Stage.AFTER_RECEIPT);
            return receipt;
        });
    }

    /** Absence is only a read result, never proof that an in-flight request failed. */
    public Optional<Receipt> lookup(String versionId) {
        ControlledBpmn.requireUuid(versionId);
        return jdbc.query("SELECT * FROM wf_deployments WHERE version_id=?::uuid AND status='confirmed'",
            RECEIPT, versionId).stream().findFirst();
    }

    private static String sha256(byte[] bytes) {
        try { return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes)); }
        catch (NoSuchAlgorithmException impossible) { throw new IllegalStateException(impossible); }
    }
}
