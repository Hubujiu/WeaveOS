-- +goose Up
ALTER TABLE applications.apps ADD COLUMN deleted_at timestamptz DEFAULT NULL;
ALTER TABLE applications.directories ADD COLUMN deleted_at timestamptz DEFAULT NULL;
ALTER TABLE applications.logical_tables ADD COLUMN deleted_at timestamptz DEFAULT NULL;
ALTER TABLE applications.form_views ADD COLUMN deleted_at timestamptz DEFAULT NULL;
CREATE TABLE applications.structure_deletions (
 app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT,
 resource_kind text NOT NULL CHECK(resource_kind IN ('application','directory','table','form')),
 resource_id uuid NOT NULL,actor_user_id uuid NOT NULL,operation_id uuid NOT NULL,
 before_structure_version bigint NOT NULL CHECK(before_structure_version BETWEEN 0 AND 9007199254740990),
 after_structure_version bigint NOT NULL CHECK(after_structure_version=before_structure_version+1),
 resource_version bigint NOT NULL CHECK(resource_version BETWEEN 0 AND 9007199254740991),
 deleted_at timestamptz NOT NULL,
 PRIMARY KEY(app_id,resource_kind,resource_id),UNIQUE(actor_user_id,operation_id),
 CHECK(resource_kind<>'application' OR (resource_id=app_id AND resource_version>0)),
 CHECK(resource_kind<>'directory' OR resource_version=0)
);
CREATE INDEX ix_structure_deletions_app ON applications.structure_deletions(app_id,deleted_at,resource_kind,resource_id);
REVOKE ALL ON applications.structure_deletions FROM PUBLIC;
-- +goose StatementBegin
CREATE FUNCTION applications.protect_structure_identity() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE app_uuid uuid;directory_uuid uuid;
BEGIN
 IF TG_OP='INSERT' AND NEW.deleted_at IS NOT NULL THEN RAISE EXCEPTION 'new resource must be active' USING ERRCODE='23514';END IF;
 IF TG_OP='UPDATE' THEN
  IF OLD.deleted_at IS NOT NULL THEN RAISE EXCEPTION 'retained resource is immutable' USING ERRCODE='P0002';END IF;
  IF NEW.id<>OLD.id THEN RAISE EXCEPTION 'resource identity immutable' USING ERRCODE='23514';END IF;
 END IF;
 IF TG_TABLE_NAME='apps' THEN RETURN NEW;END IF;
 app_uuid:=NEW.app_id;
 IF TG_OP='UPDATE' AND NEW.app_id<>OLD.app_id THEN RAISE EXCEPTION 'application identity immutable' USING ERRCODE='23514';END IF;
 PERFORM 1 FROM applications.apps WHERE id=app_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 IF TG_TABLE_NAME='directories' THEN directory_uuid:=NEW.parent_id;ELSE directory_uuid:=NEW.directory_id;END IF;
 IF directory_uuid IS NOT NULL THEN
  PERFORM 1 FROM applications.directories WHERE app_id=app_uuid AND id=directory_uuid AND deleted_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'directory missing' USING ERRCODE='23514';END IF;
 END IF;
 IF TG_TABLE_NAME='form_views' THEN
  IF TG_OP='UPDATE' AND NEW.table_id<>OLD.table_id THEN RAISE EXCEPTION 'table identity immutable' USING ERRCODE='23514';END IF;
  PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=NEW.table_id AND deleted_at IS NULL FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.protect_structure_identity() FROM PUBLIC;
CREATE TRIGGER structure_identity BEFORE INSERT OR UPDATE ON applications.apps FOR EACH ROW EXECUTE FUNCTION applications.protect_structure_identity();
CREATE TRIGGER structure_identity BEFORE INSERT OR UPDATE ON applications.directories FOR EACH ROW EXECUTE FUNCTION applications.protect_structure_identity();
CREATE TRIGGER structure_identity BEFORE INSERT OR UPDATE ON applications.logical_tables FOR EACH ROW EXECUTE FUNCTION applications.protect_structure_identity();
CREATE TRIGGER structure_identity BEFORE INSERT OR UPDATE ON applications.form_views FOR EACH ROW EXECUTE FUNCTION applications.protect_structure_identity();
-- +goose StatementBegin
CREATE FUNCTION applications.delete_structure_resource(app_uuid uuid,resource_kind text,resource_uuid uuid,actor_uuid uuid,operation_uuid uuid,expected_structure bigint,expected_resource bigint) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE owner_uuid uuid;root boolean;sv bigint;pv bigint;rv bigint;table_uuid uuid;deps text[]:='{}';has_rows boolean;at_time timestamptz;
BEGIN
 IF app_uuid IS NULL OR resource_uuid IS NULL OR actor_uuid IS NULL OR operation_uuid IS NULL
 OR '00000000-0000-0000-0000-000000000000'::uuid=ANY(ARRAY[app_uuid,resource_uuid,actor_uuid,operation_uuid])
 OR resource_kind IS NULL OR resource_kind NOT IN ('application','directory','table','form')
 OR expected_structure IS NULL OR expected_structure NOT BETWEEN 0 AND 9007199254740990
 OR expected_resource IS NULL OR expected_resource NOT BETWEEN 0 AND 9007199254740991
 OR resource_kind='application' AND (resource_uuid<>app_uuid OR expected_resource=0)
 OR resource_kind='directory' AND expected_resource<>0 THEN RAISE EXCEPTION 'invalid deletion input' USING ERRCODE='23514';END IF;
 PERFORM personnel.lock_query_revisions();
 SELECT u.is_bootstrap_admin INTO root FROM auth.users u WHERE u.id=actor_uuid AND u.status='active' FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'actor unavailable' USING ERRCODE='42501';END IF;
 SELECT a.owner_user_id,a.structure_version,a.policy_revision INTO owner_uuid,sv,pv FROM applications.apps a WHERE a.id=app_uuid AND a.deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 IF NOT root AND owner_uuid<>actor_uuid THEN RAISE EXCEPTION 'manager required' USING ERRCODE='42501';END IF;
 IF NOT EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='app.'||app_uuid::text||'.access' AND enabled) THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 -- App gate precedes table gate. Retained resources are never eligible again.
 IF resource_kind='directory' THEN
  PERFORM 1 FROM applications.directories WHERE app_id=app_uuid AND id=resource_uuid AND deleted_at IS NULL FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'directory missing' USING ERRCODE='P0002';END IF;rv:=0;
 ELSIF resource_kind='table' THEN
  SELECT id,schema_version INTO table_uuid,rv FROM applications.logical_tables WHERE app_id=app_uuid AND id=resource_uuid AND deleted_at IS NULL FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
 ELSIF resource_kind='form' THEN
  SELECT v.table_id INTO table_uuid FROM applications.form_views v WHERE v.app_id=app_uuid AND v.id=resource_uuid AND v.deleted_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'view missing' USING ERRCODE='P0002';END IF;
  PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid AND deleted_at IS NULL FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
  SELECT view_version INTO rv FROM applications.form_views WHERE app_id=app_uuid AND id=resource_uuid AND deleted_at IS NULL FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'view missing' USING ERRCODE='P0002';END IF;
 ELSE rv:=pv;END IF;
 IF sv<>expected_structure THEN RAISE EXCEPTION 'structure conflict' USING ERRCODE='W0038',DETAIL=sv::text;END IF;
 IF rv<>expected_resource THEN
  IF resource_kind='table' THEN RAISE EXCEPTION 'schema conflict' USING ERRCODE='W0002',DETAIL=rv::text;
  ELSIF resource_kind='form' THEN RAISE EXCEPTION 'view conflict' USING ERRCODE='W0039',DETAIL=rv::text;
  ELSE RAISE EXCEPTION 'policy conflict' USING ERRCODE='W0040';END IF;
 END IF;
 IF resource_kind IN ('application','directory') THEN
  IF EXISTS(SELECT 1 FROM applications.directories WHERE app_id=app_uuid AND deleted_at IS NULL AND (resource_kind='application' OR parent_id=resource_uuid)) THEN deps:=array_append(deps,'directories');END IF;
  IF EXISTS(SELECT 1 FROM applications.logical_tables WHERE app_id=app_uuid AND deleted_at IS NULL AND (resource_kind='application' OR directory_id=resource_uuid)) THEN deps:=array_append(deps,'tables');END IF;
  IF EXISTS(SELECT 1 FROM applications.form_views WHERE app_id=app_uuid AND deleted_at IS NULL AND (resource_kind='application' OR directory_id=resource_uuid)) THEN deps:=array_append(deps,'forms');END IF;
 END IF;
 IF resource_kind='application' THEN
  IF EXISTS(SELECT 1 FROM applications.permission_groups WHERE app_id=app_uuid) THEN deps:=array_append(deps,'permission_groups');END IF;
 END IF;
 IF resource_kind='table' AND EXISTS(SELECT 1 FROM applications.form_views WHERE app_id=app_uuid AND table_id=table_uuid AND deleted_at IS NULL) THEN deps:=array_append(deps,'forms');END IF;
 IF resource_kind<>'directory' AND EXISTS(SELECT 1 FROM applications.workflow_definitions WHERE app_id=app_uuid AND (resource_kind='application' OR resource_kind='table' AND table_id=table_uuid OR resource_kind='form' AND view_id=resource_uuid)) THEN deps:=array_append(deps,'workflows');END IF;
 IF resource_kind IN ('table','form') THEN
  IF EXISTS(SELECT 1 FROM applications.record_drafts WHERE app_id=app_uuid AND table_id=table_uuid AND (resource_kind='table' OR view_id=resource_uuid)) THEN deps:=array_append(deps,'drafts');END IF;
  IF EXISTS(SELECT 1 FROM applications.record_command_fences f LEFT JOIN applications.workflow_commands c ON c.command_id=f.command_id WHERE f.app_id=app_uuid AND f.table_id=table_uuid AND f.state='pending' AND (resource_kind='table' OR c.command_json->>'ViewID'=resource_uuid::text OR c.command_json->>'ViewID' IS NULL)) THEN deps:=array_append(deps,'pending_commands');END IF;
 END IF;
 IF resource_kind IN ('directory','form') AND EXISTS(SELECT 1 FROM applications.grants g WHERE g.app_id=app_uuid AND g.resource_kind=delete_structure_resource.resource_kind AND g.resource_id=resource_uuid) THEN deps:=array_append(deps,'grants');END IF;
 IF resource_kind='table' AND EXISTS(SELECT 1 FROM applications.logical_tables WHERE id=table_uuid AND schema_ready)
 AND to_regclass(format('appdata.%I','t_'||replace(table_uuid::text,'-',''))) IS NULL THEN RAISE EXCEPTION 'typed storage unavailable' USING ERRCODE='55000';END IF;
 IF resource_kind='table' AND to_regclass(format('appdata.%I','t_'||replace(table_uuid::text,'-',''))) IS NOT NULL THEN
  EXECUTE format('LOCK TABLE appdata.%I IN SHARE ROW EXCLUSIVE MODE','t_'||replace(table_uuid::text,'-',''));
  EXECUTE format('SELECT EXISTS(SELECT 1 FROM appdata.%I)','t_'||replace(table_uuid::text,'-','')) INTO has_rows;
  IF has_rows THEN deps:=array_append(deps,'records');END IF;
 END IF;
 IF cardinality(deps)>0 THEN RAISE EXCEPTION 'resource not empty' USING ERRCODE='W0037',DETAIL=to_json(deps)::text;END IF;
 at_time:=clock_timestamp();
 IF resource_kind='application' THEN
  UPDATE applications.apps SET deleted_at=at_time,policy_revision=policy_revision+1,structure_version=structure_version+1 WHERE id=app_uuid;
  UPDATE personnel.permission_catalog SET enabled=false WHERE code='app.'||app_uuid::text||'.access';
 ELSIF resource_kind='directory' THEN UPDATE applications.directories SET deleted_at=at_time WHERE app_id=app_uuid AND id=resource_uuid;
 ELSIF resource_kind='table' THEN UPDATE applications.logical_tables SET deleted_at=at_time WHERE app_id=app_uuid AND id=resource_uuid;
 ELSE
  UPDATE applications.form_views SET deleted_at=at_time WHERE app_id=app_uuid AND id=resource_uuid;
  UPDATE applications.logical_tables SET dependency_revision=dependency_revision+1 WHERE app_id=app_uuid AND id=table_uuid;
 END IF;
 IF resource_kind<>'application' THEN UPDATE applications.apps SET structure_version=structure_version+1 WHERE id=app_uuid;END IF;
 INSERT INTO applications.structure_deletions(app_id,resource_kind,resource_id,actor_user_id,operation_id,before_structure_version,after_structure_version,resource_version,deleted_at)
 VALUES(app_uuid,resource_kind,resource_uuid,actor_uuid,operation_uuid,sv,sv+1,rv,at_time);
 RETURN jsonb_build_object('operationId',operation_uuid,'appId',app_uuid,'resourceKind',resource_kind,'id',resource_uuid,'structureVersion',sv+1,'deleted',true);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.delete_structure_resource(uuid,text,uuid,uuid,uuid,bigint,bigint) FROM PUBLIC;

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
  'workflow.task.return','workflow.manual.start','workflow.delete','workflow.round.resubmit','workflow.round.review','application.template.import','preset.create','preset.update','preset.discard','record.delete','record.restore','application.delete','directory.delete','table.delete','form.delete'
 ));

ALTER TABLE applications.operations ADD CONSTRAINT ck_structure_deletion_result CHECK(
 result_json IS NULL OR operation_kind NOT IN ('application.delete','directory.delete','table.delete','form.delete') OR COALESCE(
 http_status=200 AND location=''
 AND result_json ?& ARRAY['operationId','appId','resourceKind','id','structureVersion','deleted']
 AND result_json-ARRAY['operationId','appId','resourceKind','id','structureVersion','deleted']='{}'::jsonb
 AND jsonb_typeof(result_json->'operationId')='string' AND result_json->>'operationId'=operation_id::text
 AND jsonb_typeof(result_json->'appId')='string' AND result_json->>'appId'=app_id::text
 AND jsonb_typeof(result_json->'resourceKind')='string' AND result_json->>'resourceKind'=split_part(operation_kind,'.',1)
 AND jsonb_typeof(result_json->'id')='string' AND result_json->>'id'~'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
 AND result_json->>'id'<>'00000000-0000-0000-0000-000000000000'
 AND (operation_kind<>'application.delete' OR result_json->>'id'=app_id::text)
 AND jsonb_typeof(result_json->'structureVersion')='number' AND result_json->>'structureVersion'~'^[1-9][0-9]{0,15}$' AND (result_json->>'structureVersion')::numeric BETWEEN 1 AND 9007199254740991
 AND result_json->'deleted'='true'::jsonb,false));
ALTER FUNCTION applications.apply_schema_change(uuid,uuid,uuid,text,jsonb,jsonb) RENAME TO apply_schema_change_before_structure_deletion;
REVOKE ALL ON FUNCTION applications.apply_schema_change_before_structure_deletion(uuid,uuid,uuid,text,jsonb,jsonb) FROM PUBLIC;
-- +goose StatementBegin
DO $$ DECLARE principal text;BEGIN
 FOREACH principal IN ARRAY ARRAY['auth_app','auth_reader','auth_maintenance','auth_backup'] LOOP
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=principal) THEN EXECUTE format('REVOKE ALL ON FUNCTION applications.apply_schema_change_before_structure_deletion(uuid,uuid,uuid,text,jsonb,jsonb) FROM %I',principal);END IF;END LOOP;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.apply_schema_change(actor_uuid uuid,app_uuid uuid,table_uuid uuid,operation text,before_field jsonb,after_field jsonb) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.apps WHERE id=app_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
 PERFORM applications.apply_schema_change_before_structure_deletion(actor_uuid,app_uuid,table_uuid,operation,before_field,after_field);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.apply_schema_change(uuid,uuid,uuid,text,jsonb,jsonb) FROM PUBLIC;
ALTER FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) RENAME TO apply_record_change_before_structure_deletion;
REVOKE ALL ON FUNCTION applications.apply_record_change_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) FROM PUBLIC;
-- +goose StatementBegin
DO $$ DECLARE principal text;BEGIN
 FOREACH principal IN ARRAY ARRAY['auth_app','auth_reader','auth_maintenance','auth_backup'] LOOP
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=principal) THEN EXECUTE format('REVOKE ALL ON FUNCTION applications.apply_record_change_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) FROM %I',principal);END IF;END LOOP;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.apply_record_change(app_uuid uuid,view_uuid uuid,table_uuid uuid,record_uuid uuid,actor_uuid uuid,operation text,expected_schema bigint,expected_record bigint,field_values jsonb) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.apps WHERE id=app_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM applications.form_views WHERE app_id=app_uuid AND table_id=table_uuid AND id=view_uuid AND deleted_at IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'view missing' USING ERRCODE='P0002';END IF;
 RETURN applications.apply_record_change_before_structure_deletion(app_uuid,view_uuid,table_uuid,record_uuid,actor_uuid,operation,expected_schema,expected_record,field_values);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) FROM PUBLIC;
ALTER FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) RENAME TO acquire_record_command_fence_before_structure_deletion;
REVOKE ALL ON FUNCTION applications.acquire_record_command_fence_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM PUBLIC;
-- +goose StatementBegin
DO $$ DECLARE principal text;BEGIN
 FOREACH principal IN ARRAY ARRAY['auth_app','auth_reader','auth_maintenance','auth_backup'] LOOP
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=principal) THEN EXECUTE format('REVOKE ALL ON FUNCTION applications.acquire_record_command_fence_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM %I',principal);END IF;END LOOP;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.acquire_record_command_fence(app_uuid uuid,table_uuid uuid,view_uuid uuid,record_uuid uuid,command_uuid uuid,expected_schema bigint,expected_record bigint) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF app_uuid IS NULL OR table_uuid IS NULL OR view_uuid IS NULL OR record_uuid IS NULL OR command_uuid IS NULL
 OR '00000000-0000-0000-0000-000000000000'::uuid=ANY(ARRAY[app_uuid,table_uuid,view_uuid,record_uuid,command_uuid])
 OR expected_schema IS NULL OR expected_schema NOT BETWEEN 0 AND 9007199254740991
 OR expected_record IS NULL OR expected_record NOT BETWEEN 1 AND 9007199254740991 THEN RAISE EXCEPTION 'invalid record command fence input' USING ERRCODE='23514';END IF;
 PERFORM 1 FROM applications.apps WHERE id=app_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN IF EXISTS(SELECT 1 FROM applications.apps WHERE id=app_uuid AND deleted_at IS NOT NULL) THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';ELSE RAISE EXCEPTION 'record command fence application not found' USING ERRCODE='23503';END IF;END IF;
 PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN IF EXISTS(SELECT 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid AND deleted_at IS NOT NULL) THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';ELSE RAISE EXCEPTION 'record command fence table not found' USING ERRCODE='23503';END IF;END IF;
 PERFORM 1 FROM applications.form_views WHERE app_id=app_uuid AND table_id=table_uuid AND id=view_uuid AND deleted_at IS NULL;
 IF NOT FOUND THEN IF EXISTS(SELECT 1 FROM applications.form_views WHERE app_id=app_uuid AND table_id=table_uuid AND id=view_uuid AND deleted_at IS NOT NULL) THEN RAISE EXCEPTION 'view missing' USING ERRCODE='P0002';ELSE RAISE EXCEPTION 'record command fence view not found' USING ERRCODE='23503';END IF;END IF;
 RETURN applications.acquire_record_command_fence_before_structure_deletion(app_uuid,table_uuid,view_uuid,record_uuid,command_uuid,expected_schema,expected_record);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM PUBLIC;
ALTER FUNCTION applications.change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) RENAME TO change_record_lifecycle_before_structure_deletion;
REVOKE ALL ON FUNCTION applications.change_record_lifecycle_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) FROM PUBLIC;
-- +goose StatementBegin
DO $$ DECLARE principal text;BEGIN
 FOREACH principal IN ARRAY ARRAY['auth_app','auth_reader','auth_maintenance','auth_backup'] LOOP
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=principal) THEN EXECUTE format('REVOKE ALL ON FUNCTION applications.change_record_lifecycle_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) FROM %I',principal);END IF;END LOOP;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.change_record_lifecycle(app_uuid uuid,view_uuid uuid,table_uuid uuid,record_uuid uuid,actor_uuid uuid,operation_uuid uuid,expected_schema bigint,expected_record bigint,target_deleted boolean) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.apps WHERE id=app_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM applications.form_views WHERE app_id=app_uuid AND table_id=table_uuid AND id=view_uuid AND deleted_at IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'view missing' USING ERRCODE='P0002';END IF;
 RETURN applications.change_record_lifecycle_before_structure_deletion(app_uuid,view_uuid,table_uuid,record_uuid,actor_uuid,operation_uuid,expected_schema,expected_record,target_deleted);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION applications.protect_active_structure_reference() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE table_uuid uuid;view_uuid uuid;directory_uuid uuid;
BEGIN
 PERFORM 1 FROM applications.apps WHERE id=NEW.app_id AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 IF TG_TABLE_NAME IN ('workflow_definitions','record_drafts') THEN table_uuid:=NEW.table_id;view_uuid:=NEW.view_id;
 ELSIF TG_TABLE_NAME IN ('fields','table_field_dependencies','record_command_fences') THEN table_uuid:=NEW.table_id;
 ELSIF TG_TABLE_NAME='table_presets' THEN view_uuid:=NEW.view_id;
 ELSIF TG_TABLE_NAME IN ('grants','menu_resources') THEN
  IF NEW.resource_kind='form' THEN view_uuid:=NEW.resource_id;
  ELSIF NEW.resource_kind='directory' THEN directory_uuid:=NEW.resource_id;END IF;
 END IF;
 IF view_uuid IS NOT NULL AND table_uuid IS NULL THEN
  SELECT table_id INTO table_uuid FROM applications.form_views WHERE app_id=NEW.app_id AND id=view_uuid AND deleted_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'view missing' USING ERRCODE='P0002';END IF;
 END IF;
 IF table_uuid IS NOT NULL THEN
  PERFORM 1 FROM applications.logical_tables WHERE app_id=NEW.app_id AND id=table_uuid AND deleted_at IS NULL FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
 END IF;
 IF view_uuid IS NOT NULL THEN
  PERFORM 1 FROM applications.form_views WHERE app_id=NEW.app_id AND table_id=table_uuid AND id=view_uuid AND deleted_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'view missing' USING ERRCODE='P0002';END IF;
 END IF;
 IF directory_uuid IS NOT NULL THEN
  PERFORM 1 FROM applications.directories WHERE app_id=NEW.app_id AND id=directory_uuid AND deleted_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'directory missing' USING ERRCODE='P0002';END IF;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.protect_active_structure_reference() FROM PUBLIC;
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.permission_groups FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.group_members FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.grants FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.menu_resources FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.workflow_definitions FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.record_drafts FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.table_presets FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.fields FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.table_field_dependencies FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
CREATE TRIGGER active_structure_reference BEFORE INSERT OR UPDATE ON applications.record_command_fences FOR EACH ROW EXECUTE FUNCTION applications.protect_active_structure_reference();
ALTER FUNCTION applications.apply_option_mapping(uuid,uuid,uuid,uuid,text,jsonb,jsonb) RENAME TO apply_option_mapping_before_structure_deletion;
REVOKE ALL ON FUNCTION applications.apply_option_mapping_before_structure_deletion(uuid,uuid,uuid,uuid,text,jsonb,jsonb) FROM PUBLIC;
-- +goose StatementBegin
DO $$ DECLARE principal text;BEGIN
 FOREACH principal IN ARRAY ARRAY['auth_app','auth_reader','auth_maintenance','auth_backup'] LOOP
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=principal) THEN EXECUTE format('REVOKE ALL ON FUNCTION applications.apply_option_mapping_before_structure_deletion(uuid,uuid,uuid,uuid,text,jsonb,jsonb) FROM %I',principal);END IF;END LOOP;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.apply_option_mapping(actor_uuid uuid,app_uuid uuid,table_uuid uuid,field_uuid uuid,old_kind text,next_config jsonb,mappings jsonb) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.apps WHERE id=app_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'application missing' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM applications.logical_tables WHERE app_id=app_uuid AND id=table_uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'table missing' USING ERRCODE='P0002';END IF;
 PERFORM applications.apply_option_mapping_before_structure_deletion(actor_uuid,app_uuid,table_uuid,field_uuid,old_kind,next_config,mappings);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.apply_option_mapping(uuid,uuid,uuid,uuid,text,jsonb,jsonb) FROM PUBLIC;

-- +goose Down
LOCK TABLE applications.apps,applications.directories,applications.logical_tables,applications.form_views,applications.structure_deletions,applications.operations IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications.structure_deletions) OR EXISTS(SELECT 1 FROM applications.apps WHERE deleted_at IS NOT NULL) OR EXISTS(SELECT 1 FROM applications.directories WHERE deleted_at IS NOT NULL) OR EXISTS(SELECT 1 FROM applications.logical_tables WHERE deleted_at IS NOT NULL) OR EXISTS(SELECT 1 FROM applications.form_views WHERE deleted_at IS NOT NULL) OR EXISTS(SELECT 1 FROM applications.operations WHERE operation_kind IN ('application.delete','directory.delete','table.delete','form.delete')) THEN RAISE EXCEPTION 'structure deletion history requires retention' USING ERRCODE='55000';END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE applications.operations DROP CONSTRAINT ck_structure_deletion_result;
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
DROP FUNCTION applications.apply_schema_change(uuid,uuid,uuid,text,jsonb,jsonb);
ALTER FUNCTION applications.apply_schema_change_before_structure_deletion(uuid,uuid,uuid,text,jsonb,jsonb) RENAME TO apply_schema_change;
-- +goose StatementBegin
DO $$ BEGIN IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='auth_app') THEN GRANT EXECUTE ON FUNCTION applications.apply_schema_change(uuid,uuid,uuid,text,jsonb,jsonb) TO auth_app;END IF;END $$;
-- +goose StatementEnd
DROP FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb);
ALTER FUNCTION applications.apply_record_change_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) RENAME TO apply_record_change;
-- +goose StatementBegin
DO $$ BEGIN IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='auth_app') THEN GRANT EXECUTE ON FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) TO auth_app;END IF;END $$;
-- +goose StatementEnd
DROP FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint);
ALTER FUNCTION applications.acquire_record_command_fence_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,bigint,bigint) RENAME TO acquire_record_command_fence;
-- +goose StatementBegin
DO $$ BEGIN IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='auth_app') THEN GRANT EXECUTE ON FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) TO auth_app;END IF;END $$;
-- +goose StatementEnd
DROP FUNCTION applications.change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean);
ALTER FUNCTION applications.change_record_lifecycle_before_structure_deletion(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) RENAME TO change_record_lifecycle;
-- +goose StatementBegin
DO $$ BEGIN IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='auth_app') THEN GRANT EXECUTE ON FUNCTION applications.change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean) TO auth_app;END IF;END $$;
-- +goose StatementEnd

DROP TRIGGER active_structure_reference ON applications.permission_groups;
DROP TRIGGER active_structure_reference ON applications.group_members;
DROP TRIGGER active_structure_reference ON applications.grants;
DROP TRIGGER active_structure_reference ON applications.menu_resources;
DROP TRIGGER active_structure_reference ON applications.workflow_definitions;
DROP TRIGGER active_structure_reference ON applications.record_drafts;
DROP TRIGGER active_structure_reference ON applications.table_presets;
DROP TRIGGER active_structure_reference ON applications.fields;
DROP TRIGGER active_structure_reference ON applications.table_field_dependencies;
DROP TRIGGER active_structure_reference ON applications.record_command_fences;
DROP FUNCTION applications.protect_active_structure_reference();
DROP FUNCTION applications.apply_option_mapping(uuid,uuid,uuid,uuid,text,jsonb,jsonb);
ALTER FUNCTION applications.apply_option_mapping_before_structure_deletion(uuid,uuid,uuid,uuid,text,jsonb,jsonb) RENAME TO apply_option_mapping;
-- +goose StatementBegin
DO $$ BEGIN IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='auth_app') THEN GRANT EXECUTE ON FUNCTION applications.apply_option_mapping(uuid,uuid,uuid,uuid,text,jsonb,jsonb) TO auth_app;END IF;END $$;
-- +goose StatementEnd

DROP TRIGGER structure_identity ON applications.apps;
DROP TRIGGER structure_identity ON applications.directories;
DROP TRIGGER structure_identity ON applications.logical_tables;
DROP TRIGGER structure_identity ON applications.form_views;
DROP FUNCTION applications.protect_structure_identity();
GRANT UPDATE ON applications.directories,applications.logical_tables,applications.form_views TO auth_app;
DROP FUNCTION applications.delete_structure_resource(uuid,text,uuid,uuid,uuid,bigint,bigint);
DROP TABLE applications.structure_deletions;
ALTER TABLE applications.form_views DROP COLUMN deleted_at;
ALTER TABLE applications.logical_tables DROP COLUMN deleted_at;
ALTER TABLE applications.directories DROP COLUMN deleted_at;
ALTER TABLE applications.apps DROP COLUMN deleted_at;
