-- +goose Up
-- Immutable confirmed command events; record values remain in record history.
ALTER TABLE applications.workflow_instances
 ADD COLUMN engine_process_id text,
 ADD CONSTRAINT ck_workflow_instance_engine_id CHECK(engine_process_id IS NULL OR length(engine_process_id) BETWEEN 1 AND 200),
 ADD CONSTRAINT uq_workflow_instance_scope UNIQUE(app_id,id);

CREATE UNIQUE INDEX uq_workflow_instance_engine_process ON applications.workflow_instances(engine_process_id) WHERE engine_process_id IS NOT NULL;
-- One v2 start command owns one reserved instance; retrying a rejected/withdrawn
-- business flow creates a distinct instance rather than a second start command.
CREATE UNIQUE INDEX uq_workflow_v2_start_instance ON applications.workflow_commands((command_json->>'InstanceID'))
 WHERE command_json->>'ProtocolVersion'='2' AND command_json->>'Action'='start';

CREATE TABLE applications.workflow_execution_events (
 command_id uuid PRIMARY KEY REFERENCES applications.workflow_commands(command_id) ON DELETE RESTRICT,
 app_id uuid NOT NULL,
 instance_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 action text NOT NULL CHECK(action IN ('start','agree','reject','withdraw','return')),
 outcome text NOT NULL CHECK(outcome IN ('success','no_effect')),
 sequence bigint NOT NULL CHECK(sequence BETWEEN 0 AND 9007199254740991),
 schema_version bigint NOT NULL CHECK(schema_version BETWEEN 1 AND 9007199254740991),
 record_version bigint NOT NULL CHECK(record_version BETWEEN 1 AND 9007199254740991),
 evidence_hash bytea NOT NULL CHECK(octet_length(evidence_hash)=32),
 payload_bytes bytea NOT NULL CHECK(octet_length(payload_bytes) BETWEEN 1 AND 262144),
 result_bytes bytea NOT NULL CHECK(octet_length(result_bytes) BETWEEN 8 AND 65536),
 proof_id uuid NOT NULL UNIQUE,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(app_id,instance_id) REFERENCES applications.workflow_instances(app_id,id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_workflow_execution_success_sequence
 ON applications.workflow_execution_events(app_id,instance_id,sequence) WHERE outcome='success';
CREATE INDEX ix_workflow_execution_events_instance
 ON applications.workflow_execution_events(app_id,instance_id,sequence,created_at,command_id);

CREATE TABLE applications.workflow_tasks (
 id uuid PRIMARY KEY,
 app_id uuid NOT NULL,
 instance_id uuid NOT NULL,
 node_id uuid NOT NULL,
 assignee_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 engine_task_id text NOT NULL UNIQUE CHECK(length(engine_task_id) BETWEEN 1 AND 200),
 activation_epoch bigint NOT NULL CHECK(activation_epoch BETWEEN 1 AND 9007199254740991),
 opened_command_id uuid NOT NULL REFERENCES applications.workflow_execution_events(command_id) ON DELETE RESTRICT,
 closed_command_id uuid REFERENCES applications.workflow_execution_events(command_id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(app_id,instance_id) REFERENCES applications.workflow_instances(app_id,id) ON DELETE RESTRICT
);
CREATE INDEX ix_workflow_tasks_active_instance ON applications.workflow_tasks(app_id,instance_id,id) WHERE closed_command_id IS NULL;
CREATE INDEX ix_workflow_tasks_inbox ON applications.workflow_tasks(app_id,assignee_id,created_at,id) WHERE closed_command_id IS NULL;
CREATE INDEX ix_workflow_tasks_activation ON applications.workflow_tasks(app_id,instance_id,activation_epoch DESC,id);
REVOKE ALL ON applications.workflow_execution_events,applications.workflow_tasks FROM PUBLIC;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 LOCK TABLE applications.workflow_instances,applications.workflow_execution_events,applications.workflow_tasks IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM applications.workflow_execution_events)
 OR EXISTS(SELECT 1 FROM applications.workflow_tasks)
 OR EXISTS(SELECT 1 FROM applications.workflow_instances WHERE engine_process_id IS NOT NULL)
 OR EXISTS(SELECT 1 FROM applications.workflow_commands WHERE command_json->>'ProtocolVersion'='2' AND command_json->>'Action'='start') THEN
  RAISE EXCEPTION 'durable execution projection exists; Down refused' USING ERRCODE='55000';
 END IF;
 DROP INDEX applications.uq_workflow_v2_start_instance;
 DROP INDEX applications.uq_workflow_instance_engine_process;
 DROP TABLE applications.workflow_tasks;
 DROP TABLE applications.workflow_execution_events;
 ALTER TABLE applications.workflow_instances DROP CONSTRAINT uq_workflow_instance_scope;
 ALTER TABLE applications.workflow_instances DROP CONSTRAINT ck_workflow_instance_engine_id;
 ALTER TABLE applications.workflow_instances DROP COLUMN engine_process_id;
END $$;
-- +goose StatementEnd
