-- +goose Up
CREATE TABLE applications.workflow_action_reasons (
 command_id uuid PRIMARY KEY REFERENCES applications.workflow_commands(command_id) ON DELETE RESTRICT,
 reason text NOT NULL,
 CONSTRAINT workflow_action_reason_length CHECK(char_length(reason) BETWEEN 1 AND 500),
 CONSTRAINT workflow_action_reason_nonblank CHECK(length(btrim(reason,U&'\0009\000A\000B\000C\000D\0020\0085\00A0\1680\2000\2001\2002\2003\2004\2005\2006\2007\2008\2009\200A\2028\2029\202F\205F\3000'))>0)
);
-- +goose StatementBegin
CREATE FUNCTION applications.validate_workflow_action_reason() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
BEGIN
 IF NOT EXISTS (SELECT 1 FROM applications.workflow_commands
  WHERE command_id=NEW.command_id AND command_json->>'Action' IN ('reject','return')) THEN
  RAISE EXCEPTION 'reason requires an original reject or return command' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER workflow_action_reason_command BEFORE INSERT ON applications.workflow_action_reasons
 FOR EACH ROW EXECUTE FUNCTION applications.validate_workflow_action_reason();

-- +goose Down
LOCK TABLE applications.workflow_action_reasons IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM applications.workflow_action_reasons LIMIT 1) THEN
  RAISE EXCEPTION 'cannot remove retained human action reasons' USING ERRCODE='55000';
 END IF;
END;
$$;
-- +goose StatementEnd
DROP TABLE applications.workflow_action_reasons;
DROP FUNCTION applications.validate_workflow_action_reason();
