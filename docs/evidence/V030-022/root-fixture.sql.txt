CREATE TABLE wf_deployments (
 version_id uuid PRIMARY KEY,
 app_id uuid NOT NULL,
 flow_id uuid NOT NULL,
 version bigint NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991),
 bpmn_sha256 varchar(64) NOT NULL CHECK(bpmn_sha256 ~ '^[0-9a-f]{64}$'),
 status varchar(16) NOT NULL CHECK(status IN ('pending','confirmed')),
 engine_deployment_id varchar(200),
 process_definition_id varchar(255),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(app_id,flow_id,version),
 CHECK((status='pending' AND engine_deployment_id IS NULL AND process_definition_id IS NULL)
 OR(status='confirmed' AND engine_deployment_id IS NOT NULL AND process_definition_id IS NOT NULL))
);
