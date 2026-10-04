-- +goose Up
-- Durable, narrowly mediated record protection for accepted workflow commands.
CREATE SEQUENCE applications.record_command_fence_epoch_seq
 AS bigint
 INCREMENT BY 1
 MINVALUE 1
 MAXVALUE 9007199254740991
 START WITH 1
 CACHE 1
 NO CYCLE;

ALTER TABLE applications.record_command_fences
 ADD COLUMN fence_epoch bigint NOT NULL DEFAULT 0
 CHECK (fence_epoch BETWEEN 0 AND 9007199254740991);

CREATE UNIQUE INDEX uq_record_command_fences_record
 ON applications.record_command_fences(app_id, table_id, record_id);
CREATE UNIQUE INDEX uq_record_command_fences_command
 ON applications.record_command_fences(command_id);

-- +goose StatementBegin
CREATE FUNCTION applications.acquire_record_command_fence(
 app_uuid uuid,
 table_uuid uuid,
 view_uuid uuid,
 record_uuid uuid,
 command_uuid uuid,
 expected_schema bigint,
 expected_record bigint
) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 max_safe_integer constant bigint := 9007199254740991;
 zero_uuid constant uuid := '00000000-0000-0000-0000-000000000000';
 current_schema_version bigint;
 current_record bigint;
 schema_is_ready boolean;
 existing_command uuid;
 existing_version bigint;
 existing_epoch bigint;
 next_epoch bigint;
 physical_table text;
BEGIN
 IF app_uuid IS NULL OR app_uuid=zero_uuid
 OR table_uuid IS NULL OR table_uuid=zero_uuid
 OR view_uuid IS NULL OR view_uuid=zero_uuid
 OR record_uuid IS NULL OR record_uuid=zero_uuid
 OR command_uuid IS NULL OR command_uuid=zero_uuid
 OR expected_schema IS NULL OR expected_schema<0 OR expected_schema>max_safe_integer
 OR expected_record IS NULL OR expected_record<1 OR expected_record>max_safe_integer THEN
  RAISE EXCEPTION 'invalid record command fence input' USING ERRCODE='23514';
 END IF;

 SELECT schema_version,schema_ready
 INTO current_schema_version,schema_is_ready
 FROM applications.logical_tables
 WHERE app_id=app_uuid AND id=table_uuid
 FOR UPDATE;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'record command fence table not found' USING ERRCODE='23503';
 END IF;

 IF NOT EXISTS (
  SELECT 1
  FROM applications.form_views v
  JOIN applications.menu_resources m
   ON m.app_id=v.app_id AND m.resource_kind='form' AND m.resource_id=v.id
  WHERE v.app_id=app_uuid AND v.table_id=table_uuid AND v.id=view_uuid
 ) THEN
  RAISE EXCEPTION 'record command fence view not found' USING ERRCODE='23503';
 END IF;
 IF NOT schema_is_ready THEN
  RAISE EXCEPTION 'record command fence schema is not ready' USING ERRCODE='55000';
 END IF;
 IF current_schema_version<>expected_schema THEN
  RAISE EXCEPTION 'record command fence schema version changed' USING ERRCODE='40001';
 END IF;

 physical_table := 't_'||replace(table_uuid::text,'-','');
 BEGIN
  EXECUTE format('SELECT record_version FROM %I.%I WHERE id=$1 FOR UPDATE','appdata',physical_table)
   INTO current_record USING record_uuid;
 EXCEPTION WHEN undefined_table OR undefined_column THEN
  RAISE EXCEPTION 'record command fence record not found' USING ERRCODE='23503';
 END;
 IF current_record IS NULL THEN
  RAISE EXCEPTION 'record command fence record not found' USING ERRCODE='23503';
 END IF;
 IF current_record<>expected_record THEN
  RAISE EXCEPTION 'record command fence record version changed' USING ERRCODE='40001';
 END IF;

 SELECT command_id,expected_record_version,fence_epoch
 INTO existing_command,existing_version,existing_epoch
 FROM applications.record_command_fences
 WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid
 FOR UPDATE;
 IF FOUND THEN
  IF existing_command=command_uuid AND existing_version=expected_record AND existing_epoch>0 THEN
   RETURN existing_epoch;
  END IF;
  RAISE EXCEPTION 'record already has a pending command fence' USING ERRCODE='55000';
 END IF;

 PERFORM 1 FROM applications.record_command_fences
 WHERE command_id=command_uuid
 FOR UPDATE;
 IF FOUND THEN
  RAISE EXCEPTION 'command already owns a different record fence' USING ERRCODE='55000';
 END IF;

 BEGIN
  SELECT pg_catalog.nextval('applications.record_command_fence_epoch_seq'::regclass)
  INTO next_epoch;
 EXCEPTION WHEN SQLSTATE '2200H' THEN
  RAISE EXCEPTION 'record command fence epoch exhausted' USING ERRCODE='23514';
 END;

 INSERT INTO applications.record_command_fences
  (app_id,table_id,record_id,command_id,expected_record_version,state,fence_epoch)
 VALUES
  (app_uuid,table_uuid,record_uuid,command_uuid,expected_record,'pending',next_epoch);
 RETURN next_epoch;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION applications.release_record_command_fence(
 app_uuid uuid,
 table_uuid uuid,
 record_uuid uuid,
 command_uuid uuid,
 requested_epoch bigint,
 expected_record bigint
) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 max_safe_integer constant bigint := 9007199254740991;
 zero_uuid constant uuid := '00000000-0000-0000-0000-000000000000';
 current_record bigint;
 existing_command uuid;
 existing_epoch bigint;
 existing_version bigint;
 physical_table text;
 deleted_rows bigint;
BEGIN
 IF app_uuid IS NULL OR app_uuid=zero_uuid
 OR table_uuid IS NULL OR table_uuid=zero_uuid
 OR record_uuid IS NULL OR record_uuid=zero_uuid
 OR command_uuid IS NULL OR command_uuid=zero_uuid
 OR requested_epoch IS NULL OR requested_epoch<1 OR requested_epoch>max_safe_integer
 OR expected_record IS NULL OR expected_record<1 OR expected_record>max_safe_integer THEN
  RAISE EXCEPTION 'invalid record command fence release input' USING ERRCODE='23514';
 END IF;

 PERFORM 1 FROM applications.logical_tables
 WHERE app_id=app_uuid AND id=table_uuid
 FOR UPDATE;
 IF NOT FOUND THEN
  RETURN false;
 END IF;

 physical_table := 't_'||replace(table_uuid::text,'-','');
 BEGIN
  EXECUTE format('SELECT record_version FROM %I.%I WHERE id=$1 FOR UPDATE','appdata',physical_table)
   INTO current_record USING record_uuid;
 EXCEPTION WHEN undefined_table OR undefined_column THEN
  RETURN false;
 END;
 IF current_record IS NULL OR current_record<>expected_record THEN
  RETURN false;
 END IF;

 SELECT command_id,expected_record_version,fence_epoch
 INTO existing_command,existing_version,existing_epoch
 FROM applications.record_command_fences
 WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid
 FOR UPDATE;
 IF NOT FOUND OR existing_epoch<1
 OR existing_command<>command_uuid
 OR existing_epoch<>requested_epoch
 OR existing_version<>expected_record THEN
  RETURN false;
 END IF;

 DELETE FROM applications.record_command_fences
 WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid
 AND command_id=command_uuid AND fence_epoch=existing_epoch
 AND expected_record_version=expected_record;
 GET DIAGNOSTICS deleted_rows=ROW_COUNT;
 RETURN deleted_rows=1;
END $$;
-- +goose StatementEnd

REVOKE ALL ON SEQUENCE applications.record_command_fence_epoch_seq
 FROM PUBLIC;
REVOKE ALL ON FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint)
 FROM PUBLIC;
REVOKE ALL ON FUNCTION applications.release_record_command_fence(uuid,uuid,uuid,uuid,bigint,bigint)
 FROM PUBLIC;

-- New clusters apply migrations before the separately installed runtime roles.
-- Revoke from each restricted role when it already exists; never create roles here.
-- +goose StatementBegin
DO $$
DECLARE principal text;
BEGIN
 FOREACH principal IN ARRAY ARRAY['auth_app','auth_reader','auth_maintenance','auth_backup'] LOOP
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=principal) THEN
   EXECUTE format('REVOKE ALL ON SEQUENCE applications.record_command_fence_epoch_seq FROM %I',principal);
   EXECUTE format('REVOKE ALL ON FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM %I',principal);
   EXECUTE format('REVOKE ALL ON FUNCTION applications.release_record_command_fence(uuid,uuid,uuid,uuid,bigint,bigint) FROM %I',principal);
  END IF;
 END LOOP;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION applications.block_active_workflow_command_schema_change() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.schema_version IS DISTINCT FROM OLD.schema_version
 AND EXISTS (
  SELECT 1 FROM applications.record_command_fences f
  WHERE f.app_id=OLD.app_id AND f.table_id=OLD.id AND f.state='pending'
 ) THEN
  RAISE EXCEPTION 'pending workflow command blocks schema change'
   USING ERRCODE='55000',CONSTRAINT='active_workflow_command_blocks_schema_change';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER active_workflow_command_blocks_schema_change
 BEFORE UPDATE OF schema_version ON applications.logical_tables
 FOR EACH ROW EXECUTE FUNCTION applications.block_active_workflow_command_schema_change();

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE last_epoch bigint; epoch_called boolean;
BEGIN
 SELECT last_value,is_called INTO last_epoch,epoch_called
 FROM applications.record_command_fence_epoch_seq;
 IF epoch_called OR EXISTS (
  SELECT 1 FROM applications.record_command_fences WHERE fence_epoch>0
 ) THEN
  RAISE EXCEPTION 'cannot roll back used record command fence epochs' USING ERRCODE='55000';
 END IF;
END $$;
-- +goose StatementEnd

DROP TRIGGER active_workflow_command_blocks_schema_change ON applications.logical_tables;
DROP FUNCTION applications.block_active_workflow_command_schema_change();
DROP FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint);
DROP FUNCTION applications.release_record_command_fence(uuid,uuid,uuid,uuid,bigint,bigint);
DROP INDEX applications.uq_record_command_fences_record;
DROP INDEX applications.uq_record_command_fences_command;
ALTER TABLE applications.record_command_fences DROP COLUMN fence_epoch;
DROP SEQUENCE applications.record_command_fence_epoch_seq;
