-- +goose Up
-- Version metadata only; business records remain real typed columns.
-- Existing definitions have no triggers until explicitly configured/published.
ALTER TABLE applications.workflow_versions
 ADD COLUMN triggers_json jsonb NOT NULL DEFAULT '[]'::jsonb,
 ADD CONSTRAINT workflow_versions_triggers_shape CHECK (
   jsonb_typeof(triggers_json)='array' AND jsonb_array_length(triggers_json)<=3
 );

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 LOCK TABLE applications.workflow_versions IN ACCESS EXCLUSIVE MODE;
 IF EXISTS (SELECT 1 FROM applications.workflow_versions WHERE triggers_json <> '[]'::jsonb) THEN
  RAISE EXCEPTION 'cannot remove durable workflow trigger configuration' USING ERRCODE='55000';
 END IF;
END;
$$;
-- +goose StatementEnd
ALTER TABLE applications.workflow_versions
 DROP CONSTRAINT workflow_versions_triggers_shape,
 DROP COLUMN triggers_json;
