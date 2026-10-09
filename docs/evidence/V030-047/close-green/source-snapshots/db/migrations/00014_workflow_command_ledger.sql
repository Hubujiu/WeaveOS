-- +goose Up
CREATE TABLE applications.workflow_commands (
    command_id uuid PRIMARY KEY,
    command_json jsonb NOT NULL,
    command_hash bytea NOT NULL,
    state text NOT NULL,
    receipt_json jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workflow_commands_command_json_object
        CHECK (jsonb_typeof(command_json) = 'object'),
    CONSTRAINT workflow_commands_command_id_matches_json
        CHECK (COALESCE(
            jsonb_typeof(command_json->'CommandID') = 'string'
            AND command_json->>'CommandID' = command_id::text,
            false
        )),
    CONSTRAINT workflow_commands_command_hash_length
        CHECK (octet_length(command_hash) = 32),
    CONSTRAINT workflow_commands_state_valid
        CHECK (state IN ('pending', 'success', 'no_effect')),
    CONSTRAINT workflow_commands_receipt_object
        CHECK (receipt_json IS NULL OR COALESCE(jsonb_typeof(receipt_json) = 'object', false)),
    CONSTRAINT workflow_commands_pending_receipt_consistent
        CHECK ((state = 'pending') = (receipt_json IS NULL))
);

CREATE TABLE applications.workflow_dispatch (
    command_id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workflow_dispatch_command_fk
        FOREIGN KEY (command_id)
        REFERENCES applications.workflow_commands (command_id)
        ON DELETE RESTRICT
);

CREATE INDEX workflow_dispatch_created_at_command_id_idx
    ON applications.workflow_dispatch (created_at, command_id);

-- +goose Down
LOCK TABLE applications.workflow_commands, applications.workflow_dispatch
    IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM applications.workflow_commands LIMIT 1)
       OR EXISTS (SELECT 1 FROM applications.workflow_dispatch LIMIT 1) THEN
        RAISE EXCEPTION 'cannot remove workflow command ledger while history exists'
            USING ERRCODE = '55000';
    END IF;
END;
$$;
-- +goose StatementEnd

DROP INDEX applications.workflow_dispatch_created_at_command_id_idx;
DROP TABLE applications.workflow_dispatch;
DROP TABLE applications.workflow_commands;
