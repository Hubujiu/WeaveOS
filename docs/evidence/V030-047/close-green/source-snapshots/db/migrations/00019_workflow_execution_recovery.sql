-- +goose Up
-- Preserve the original v2 wire payload without changing legacy command JSON.
ALTER TABLE applications.workflow_commands
 ADD COLUMN execution_payload bytea,
 ADD CONSTRAINT ck_workflow_execution_payload CHECK (
  execution_payload IS NULL OR
  (octet_length(execution_payload) BETWEEN 45 AND 262144
   AND COALESCE(command_json->>'ProtocolVersion'='2',false)
   AND substring(execution_payload from 1 for 8)=decode('5756465041590001','hex'))
 );

ALTER TABLE applications.workflow_dispatch
 ADD COLUMN protocol_version smallint NOT NULL DEFAULT 1 CHECK(protocol_version IN (1,2)),
 ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
 ADD COLUMN attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 ADD COLUMN lease_token uuid,
 ADD COLUMN lease_until timestamptz,
 ADD COLUMN last_error text CHECK(last_error IN ('DEPENDENCY_UNAVAILABLE','ENGINE_REPLY_INVALID','COMMAND_INVALID','APPLICATION_CONFIRMATION_FAILED')),
 ADD CONSTRAINT ck_workflow_dispatch_lease CHECK ((lease_token IS NULL)=(lease_until IS NULL));

CREATE INDEX ix_workflow_execution_due
 ON applications.workflow_dispatch(next_attempt_at,created_at,command_id)
 WHERE protocol_version=2;

-- +goose Down
-- Never erase the sole durable request needed to recover an accepted command.
LOCK TABLE applications.workflow_commands,applications.workflow_dispatch IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM applications.workflow_commands WHERE execution_payload IS NOT NULL)
 OR EXISTS(SELECT 1 FROM applications.workflow_dispatch WHERE protocol_version=2) THEN
  RAISE EXCEPTION 'execution recovery history prevents downgrade' USING ERRCODE='55000';
 END IF;
END $$;
-- +goose StatementEnd
DROP INDEX applications.ix_workflow_execution_due;
ALTER TABLE applications.workflow_dispatch
 DROP CONSTRAINT ck_workflow_dispatch_lease,
 DROP COLUMN last_error,
 DROP COLUMN lease_until,
 DROP COLUMN lease_token,
 DROP COLUMN attempts,
 DROP COLUMN next_attempt_at,
 DROP COLUMN protocol_version;
ALTER TABLE applications.workflow_commands
 DROP CONSTRAINT ck_workflow_execution_payload,
 DROP COLUMN execution_payload;
