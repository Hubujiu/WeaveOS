-- +goose Up
CREATE TABLE applications.record_lifecycle (
 app_id uuid NOT NULL,table_id uuid NOT NULL,record_id uuid NOT NULL,
 deleted boolean NOT NULL,record_version bigint NOT NULL CHECK(record_version BETWEEN 1 AND 9007199254740991),
 changed_by uuid NOT NULL,changed_at timestamptz NOT NULL,
 PRIMARY KEY(app_id,table_id,record_id),
 FOREIGN KEY(app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT
);
CREATE TABLE applications.record_lifecycle_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 app_id uuid NOT NULL,table_id uuid NOT NULL,record_id uuid NOT NULL,
 view_id uuid NOT NULL,actor_user_id uuid NOT NULL,operation_id uuid NOT NULL,
 action text NOT NULL CHECK(action IN ('delete','restore')),
 before_record_version bigint NOT NULL CHECK(before_record_version BETWEEN 1 AND 9007199254740990),
 after_record_version bigint NOT NULL CHECK(after_record_version=before_record_version+1),
 occurred_at timestamptz NOT NULL,
 FOREIGN KEY(app_id,table_id,record_id) REFERENCES applications.record_lifecycle(app_id,table_id,record_id) ON DELETE RESTRICT,
 UNIQUE(actor_user_id,operation_id)
);
CREATE INDEX ix_record_lifecycle_events_record ON applications.record_lifecycle_events(app_id,table_id,record_id,id);
REVOKE ALL ON applications.record_lifecycle,applications.record_lifecycle_events FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION applications.change_record_lifecycle(app_uuid uuid,view_uuid uuid,table_uuid uuid,record_uuid uuid,actor_uuid uuid,operation_uuid uuid,expected_schema bigint,expected_record bigint,target_deleted boolean) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ready boolean;sv bigint;rv bigint;is_deleted boolean;qualified text;at_time timestamptz;
BEGIN
 IF app_uuid IS NULL OR view_uuid IS NULL OR table_uuid IS NULL OR record_uuid IS NULL OR actor_uuid IS NULL OR operation_uuid IS NULL
 OR '00000000-0000-0000-0000-000000000000'::uuid=ANY(ARRAY[app_uuid,view_uuid,table_uuid,record_uuid,actor_uuid,operation_uuid])
 OR expected_schema IS NULL OR expected_schema NOT BETWEEN 0 AND 9007199254740991 OR expected_record IS NULL OR expected_record NOT BETWEEN 1 AND 9007199254740990 OR target_deleted IS NULL THEN
 RAISE EXCEPTION 'invalid lifecycle input' USING ERRCODE='23514';END IF;
 SELECT t.schema_ready,t.schema_version INTO ready,sv FROM applications.logical_tables t
 JOIN applications.form_views v ON v.app_id=t.app_id AND v.table_id=t.id AND v.id=view_uuid
 JOIN applications.menu_resources m ON m.app_id=v.app_id AND m.resource_kind='form' AND m.resource_id=v.id
 WHERE t.app_id=app_uuid AND t.id=table_uuid FOR UPDATE OF t;
 IF NOT FOUND THEN RAISE EXCEPTION 'resource missing' USING ERRCODE='P0002';END IF;
 IF NOT ready THEN RAISE EXCEPTION 'schema not ready' USING ERRCODE='W0001';END IF;
 IF sv<>expected_schema THEN RAISE EXCEPTION 'schema conflict' USING ERRCODE='W0002';END IF;
 qualified:=format('appdata.%I','t_'||replace(table_uuid::text,'-',''));
 EXECUTE format('SELECT record_version FROM %s WHERE id=$1 FOR UPDATE',qualified) INTO rv USING record_uuid;
 IF rv IS NULL THEN RAISE EXCEPTION 'record missing' USING ERRCODE='P0002';END IF;
 IF rv<>expected_record THEN RAISE EXCEPTION 'record conflict' USING ERRCODE='W0003';END IF;
 SELECT deleted INTO is_deleted FROM applications.record_lifecycle WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid;
 IF COALESCE(is_deleted,false)=target_deleted THEN RAISE EXCEPTION 'lifecycle conflict' USING ERRCODE='W0034';END IF;
 IF EXISTS(SELECT 1 FROM applications.record_command_fences WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid AND state='pending') THEN
 RAISE EXCEPTION 'record fenced' USING ERRCODE='W0035';END IF;
 IF EXISTS(SELECT 1 FROM applications.workflow_instances WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid AND state IN ('starting','active')) THEN
 RAISE EXCEPTION 'record in flight' USING ERRCODE='W0036';END IF;
 at_time:=clock_timestamp();
 -- The existing physical-table trigger advances data_revision exactly once.
 EXECUTE format('UPDATE %s SET record_version=record_version+1,updated_at=$2 WHERE id=$1',qualified) USING record_uuid,at_time;
 INSERT INTO applications.record_lifecycle(app_id,table_id,record_id,deleted,record_version,changed_by,changed_at)
 VALUES(app_uuid,table_uuid,record_uuid,target_deleted,rv+1,actor_uuid,at_time)
 ON CONFLICT(app_id,table_id,record_id) DO UPDATE SET deleted=EXCLUDED.deleted,record_version=EXCLUDED.record_version,changed_by=EXCLUDED.changed_by,changed_at=EXCLUDED.changed_at;
 INSERT INTO applications.record_lifecycle_events(app_id,table_id,record_id,view_id,actor_user_id,operation_id,action,before_record_version,after_record_version,occurred_at)
 VALUES(app_uuid,table_uuid,record_uuid,view_uuid,actor_uuid,operation_uuid,CASE WHEN target_deleted THEN 'delete' ELSE 'restore' END,rv,rv+1,at_time);
 RETURN jsonb_build_object('operationId',operation_uuid,'id',record_uuid,'recordVersion',rv+1,'schemaVersion',sv,'deleted',target_deleted);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) FROM PUBLIC;

ALTER TABLE applications.operations
 DROP CONSTRAINT ck_operation_kind,
 ADD CONSTRAINT ck_operation_kind CHECK(operation_kind IN (
  'application.create',
  'group.create',
  'group.update',
  'members.replace',
  'grants.replace',
  'directory.create',
  'directory.update',
  'table.create',
  'table.update',
  'form.create',
  'form.update',
  'definition.save',
  'record.create',
  'record.edit',
  'draft.create',
  'draft.update',
  'draft.discard',
  'workflow.definition.save',
  'workflow.enable',
  'workflow.close',
  'workflow.task.agree',
  'workflow.task.reject',
  'workflow.instance.withdraw',
  'workflow.task.return','workflow.manual.start','workflow.delete','workflow.round.resubmit','workflow.round.review','application.template.import','preset.create','preset.update','preset.discard','record.delete','record.restore'
 ));

-- V072 preserves the previous reviewed DML implementation behind a private name.
ALTER FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) RENAME TO apply_record_change_before_lifecycle;
ALTER FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) RENAME TO acquire_record_command_fence_before_lifecycle;
REVOKE ALL ON FUNCTION applications.apply_record_change_before_lifecycle(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb),applications.acquire_record_command_fence_before_lifecycle(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM PUBLIC;
-- +goose StatementBegin
DO $$ DECLARE principal text;BEGIN
 FOREACH principal IN ARRAY ARRAY['auth_app','auth_reader','auth_maintenance','auth_backup'] LOOP
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=principal) THEN
 EXECUTE format('REVOKE ALL ON FUNCTION applications.apply_record_change_before_lifecycle(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb),applications.acquire_record_command_fence_before_lifecycle(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM %I',principal);
 END IF;END LOOP;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.apply_record_change(app_uuid uuid,view_uuid uuid,table_uuid uuid,record_uuid uuid,actor_uuid uuid,operation text,expected_schema bigint,expected_record bigint,field_values jsonb) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid FOR UPDATE;
 IF EXISTS(SELECT 1 FROM applications.record_lifecycle WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid AND deleted) THEN RAISE EXCEPTION 'record missing' USING ERRCODE='P0002';END IF;
 RETURN applications.apply_record_change_before_lifecycle(app_uuid,view_uuid,table_uuid,record_uuid,actor_uuid,operation,expected_schema,expected_record,field_values);
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.acquire_record_command_fence(app_uuid uuid,table_uuid uuid,view_uuid uuid,record_uuid uuid,command_uuid uuid,expected_schema bigint,expected_record bigint) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid FOR UPDATE;
 IF EXISTS(SELECT 1 FROM applications.record_lifecycle WHERE app_id=app_uuid AND table_id=table_uuid AND record_id=record_uuid AND deleted) THEN RAISE EXCEPTION 'record missing' USING ERRCODE='P0002';END IF;
 RETURN applications.acquire_record_command_fence_before_lifecycle(app_uuid,table_uuid,view_uuid,record_uuid,command_uuid,expected_schema,expected_record);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb),applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM PUBLIC;
-- +goose StatementBegin
CREATE FUNCTION applications.protect_deleted_record_start() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.state IN ('starting','active') THEN
 PERFORM 1 FROM applications.logical_tables WHERE app_id=NEW.app_id AND id=NEW.table_id FOR UPDATE;
 IF EXISTS(SELECT 1 FROM applications.record_lifecycle WHERE app_id=NEW.app_id AND table_id=NEW.table_id AND record_id=NEW.record_id AND deleted) THEN RAISE EXCEPTION 'record missing' USING ERRCODE='P0002';END IF;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.protect_deleted_record_start() FROM PUBLIC;
CREATE TRIGGER protect_deleted_record_start BEFORE INSERT OR UPDATE OF state,app_id,table_id,record_id ON applications.workflow_instances FOR EACH ROW EXECUTE FUNCTION applications.protect_deleted_record_start();

ALTER TABLE applications.operations ADD CONSTRAINT ck_record_lifecycle_result CHECK(
 result_json IS NULL OR operation_kind NOT IN ('record.delete','record.restore') OR COALESCE(
 http_status=200 AND location=''
 AND result_json ?& ARRAY['operationId','id','recordVersion','schemaVersion','deleted']
 AND result_json-ARRAY['operationId','id','recordVersion','schemaVersion','deleted']='{}'::jsonb
 AND jsonb_typeof(result_json->'operationId')='string' AND result_json->>'operationId'=operation_id::text
 AND jsonb_typeof(result_json->'id')='string' AND result_json->>'id'~'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
 AND result_json->>'id'<>'00000000-0000-0000-0000-000000000000'
 AND jsonb_typeof(result_json->'recordVersion')='number' AND result_json->>'recordVersion'~'^[1-9][0-9]{0,15}$' AND (result_json->>'recordVersion')::numeric BETWEEN 2 AND 9007199254740991
 AND jsonb_typeof(result_json->'schemaVersion')='number' AND result_json->>'schemaVersion'~'^[1-9][0-9]{0,15}$' AND (result_json->>'schemaVersion')::numeric BETWEEN 1 AND 9007199254740991
 AND jsonb_typeof(result_json->'deleted')='boolean' AND (result_json->>'deleted')::boolean=(operation_kind='record.delete'),false));

-- +goose Down
-- Declaration-only until independent real Down tests.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'lifecycle down not implemented' USING ERRCODE='55000';END $$;
-- +goose StatementEnd
