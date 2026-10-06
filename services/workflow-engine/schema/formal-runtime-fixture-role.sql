-- Synthetic role for the disposable formal-runtime acceptance database only.
CREATE ROLE v041_runtime LOGIN PASSWORD 'v041_synthetic_only'
 NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
REVOKE TEMPORARY ON DATABASE b3_flowable_fixture FROM PUBLIC;
GRANT USAGE ON SCHEMA workflow TO v041_runtime;
SELECT format('GRANT SELECT,INSERT,UPDATE,DELETE ON TABLE %I.%I TO v041_runtime',schemaname,tablename)
 FROM pg_tables WHERE schemaname='workflow' AND left(tablename,4) IN ('act_','flw_') \gexec
SELECT format('GRANT USAGE,SELECT ON SEQUENCE %I.%I TO v041_runtime',schemaname,sequencename)
 FROM pg_sequences WHERE schemaname='workflow' AND left(sequencename,4) IN ('act_','flw_') \gexec
GRANT SELECT,INSERT ON workflow.wf_deployments,workflow.wf_execution_commands,
 workflow.wf_execution_instances,workflow.wf_execution_tasks TO v041_runtime;
GRANT UPDATE(status,engine_deployment_id,process_definition_id) ON workflow.wf_deployments TO v041_runtime;
GRANT UPDATE(outcome,result_sequence,proof_id,result_hash,result_bytes) ON workflow.wf_execution_commands TO v041_runtime;
GRANT UPDATE(engine_process_id,state,sequence,fence_epoch,schema_version,record_version,activation_epoch,updated_at)
 ON workflow.wf_execution_instances TO v041_runtime;
GRANT UPDATE(state,decision) ON workflow.wf_execution_tasks TO v041_runtime;
