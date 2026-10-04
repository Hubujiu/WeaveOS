package org.weaveos.workflow;
import java.util.Optional;
import java.util.function.Consumer;
import org.flowable.engine.ProcessEngine;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;

/** Trusted service-internal port. Not an authorization or public RPC endpoint. */
public final class DeploymentRegistry {
 public record Request(String appId,String flowId,String versionId,long version,String bpmnXml){}
 public record Receipt(String appId,String flowId,String versionId,long version,String bpmnSha256,String engineDeploymentId,String processDefinitionId){}
 public enum Stage {AFTER_LEDGER,AFTER_ENGINE,AFTER_RECEIPT}
 public static final class InvalidDeployment extends IllegalArgumentException {public InvalidDeployment(){super("invalid controlled deployment");}}
 public static final class DeploymentConflict extends RuntimeException {public DeploymentConflict(){super("deployment identity conflict");}}
 public DeploymentRegistry(JdbcTemplate jdbc,PlatformTransactionManager transactionManager,ProcessEngine engine){}
 public Receipt deploy(Request request){return deploy(request,stage->{});}
 public Receipt deploy(Request request,Consumer<Stage> probe){throw new IllegalStateException("V030-022 P1 not implemented");}
 public Optional<Receipt> lookup(String versionId){throw new IllegalStateException("V030-022 P1 not implemented");}
}
