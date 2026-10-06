-- Root draft schema contract for isolated execution tests; not a production migration.
CREATE TABLE wf_execution_commands (
 command_id uuid PRIMARY KEY,
 instance_id uuid NOT NULL,
 command_hash text NOT NULL CHECK(command_hash ~ '^[0-9a-f]{64}$'),
 command_bytes bytea NOT NULL CHECK(octet_length(command_bytes) BETWEEN 1 AND 538),
 payload_hash text NOT NULL CHECK(payload_hash ~ '^[0-9a-f]{64}$'),
 payload_bytes bytea NOT NULL CHECK(octet_length(payload_bytes) BETWEEN 1 AND 262144),
 outcome text NOT NULL DEFAULT 'pending' CHECK(outcome IN ('pending','success','no_effect')),
 result_sequence bigint CHECK(result_sequence BETWEEN 0 AND 9007199254740991),
 proof_id uuid,
 result_hash text CHECK(result_hash ~ '^[0-9a-f]{64}$'),
 result_bytes bytea CHECK(octet_length(result_bytes) BETWEEN 1 AND 65536),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK((outcome='pending' AND result_sequence IS NULL AND proof_id IS NULL AND result_hash IS NULL AND result_bytes IS NULL)
    OR(outcome IN ('success','no_effect') AND result_sequence IS NOT NULL AND proof_id IS NOT NULL AND result_hash IS NOT NULL AND result_bytes IS NOT NULL))
);
CREATE INDEX ix_wf_execution_commands_instance ON wf_execution_commands(instance_id,created_at,command_id);
CREATE TABLE wf_execution_instances (
 instance_id uuid PRIMARY KEY,
 app_id uuid NOT NULL, table_id uuid NOT NULL, view_id uuid NOT NULL,
 record_id uuid NOT NULL, flow_id uuid NOT NULL,
 version_id uuid NOT NULL REFERENCES wf_deployments(version_id) ON DELETE RESTRICT,
 definition_version bigint NOT NULL CHECK(definition_version BETWEEN 1 AND 9007199254740991),
 initiator_id uuid NOT NULL,
 start_payload_bytes bytea NOT NULL CHECK(octet_length(start_payload_bytes) BETWEEN 1 AND 262144),
 allow_withdraw boolean NOT NULL,
 engine_process_id text UNIQUE CHECK(length(engine_process_id) BETWEEN 1 AND 200),
 state text NOT NULL CHECK(state IN ('starting','active','completed','rejected','withdrawn')),
 sequence bigint NOT NULL CHECK(sequence BETWEEN 0 AND 9007199254740991),
 fence_epoch bigint NOT NULL CHECK(fence_epoch BETWEEN 1 AND 9007199254740991),
 schema_version bigint NOT NULL CHECK(schema_version BETWEEN 1 AND 9007199254740991),
 record_version bigint NOT NULL CHECK(record_version BETWEEN 1 AND 9007199254740991),
 activation_epoch bigint NOT NULL DEFAULT 0 CHECK(activation_epoch BETWEEN 0 AND 9007199254740991),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK((state='starting' AND sequence=0 AND engine_process_id IS NULL)
    OR(state<>'starting' AND sequence>0 AND engine_process_id IS NOT NULL))
);
CREATE TABLE wf_execution_tasks (
 task_id uuid PRIMARY KEY,
 instance_id uuid NOT NULL REFERENCES wf_execution_instances(instance_id) ON DELETE RESTRICT,
 engine_task_id text NOT NULL UNIQUE CHECK(length(engine_task_id) BETWEEN 1 AND 200),
 node_id uuid NOT NULL,
 activation_epoch bigint NOT NULL CHECK(activation_epoch BETWEEN 1 AND 9007199254740991),
 assignee_id uuid NOT NULL,
 state text NOT NULL CHECK(state IN ('active','completed','invalidated')),
 decision text CHECK(decision IN ('agree','reject')),
 CHECK((state='completed')=(decision IS NOT NULL))
);
CREATE INDEX ix_wf_execution_tasks_active ON wf_execution_tasks(instance_id,state,task_id);
CREATE TABLE wf_execution_visits (
 instance_id uuid NOT NULL REFERENCES wf_execution_instances(instance_id) ON DELETE RESTRICT,
 node_id uuid NOT NULL,
 activation_epoch bigint NOT NULL CHECK(activation_epoch BETWEEN 1 AND 9007199254740991),
 PRIMARY KEY(instance_id,node_id,activation_epoch),
 UNIQUE(instance_id,activation_epoch)
);
CREATE FUNCTION wf_execution_result_required() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE pending boolean;
BEGIN
 EXECUTE format('SELECT EXISTS(SELECT 1 FROM %I.wf_execution_commands WHERE command_id=$1 AND outcome=''pending'')',TG_TABLE_SCHEMA)
 INTO pending USING NEW.command_id;
 IF pending THEN RAISE EXCEPTION 'execution command requires a durable result' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER wf_execution_result_required AFTER INSERT OR UPDATE ON wf_execution_commands
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION wf_execution_result_required();
CREATE FUNCTION wf_execution_instance_confirmed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE pending boolean;
BEGIN
 EXECUTE format('SELECT EXISTS(SELECT 1 FROM %I.wf_execution_instances WHERE instance_id=$1 AND state=''starting'')',TG_TABLE_SCHEMA)
 INTO pending USING NEW.instance_id;
 IF pending THEN RAISE EXCEPTION 'engine instance may not commit provisional state' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER wf_execution_instance_confirmed AFTER INSERT OR UPDATE ON wf_execution_instances
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION wf_execution_instance_confirmed();
