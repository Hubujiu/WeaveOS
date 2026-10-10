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
 IF sv<>expected_structure THEN RAISE EXCEPTION 'structure conflict' USING ERRCODE='W0038';END IF;
 IF rv<>expected_resource THEN
  IF resource_kind='table' THEN RAISE EXCEPTION 'schema conflict' USING ERRCODE='W0002';
  ELSIF resource_kind='form' THEN RAISE EXCEPTION 'view conflict' USING ERRCODE='W0039';
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
 IF resource_kind='table' AND to_regclass(format('appdata.%I','t_'||replace(table_uuid::text,'-',''))) IS NOT NULL THEN
  EXECUTE format('LOCK TABLE appdata.%I IN SHARE ROW EXCLUSIVE MODE','t_'||replace(table_uuid::text,'-',''));
  EXECUTE format('SELECT EXISTS(SELECT 1 FROM appdata.%I)','t_'||replace(table_uuid::text,'-','')) INTO has_rows;
  IF has_rows THEN deps:=array_append(deps,'records');END IF;
 END IF;
 IF cardinality(deps)>0 THEN RAISE EXCEPTION 'resource not empty' USING ERRCODE='W0037',DETAIL=to_json(deps)::text;END IF;
 at_time:=clock_timestamp();
 IF resource_kind='application' THEN
  UPDATE applications.apps SET deleted_at=at_time,policy_revision=policy_revision+1 WHERE id=app_uuid;
  UPDATE personnel.permission_catalog SET enabled=false WHERE code='app.'||app_uuid::text||'.access';
 ELSIF resource_kind='directory' THEN UPDATE applications.directories SET deleted_at=at_time WHERE app_id=app_uuid AND id=resource_uuid;
 ELSIF resource_kind='table' THEN UPDATE applications.logical_tables SET deleted_at=at_time WHERE app_id=app_uuid AND id=resource_uuid;
 ELSE
  UPDATE applications.form_views SET deleted_at=at_time WHERE app_id=app_uuid AND id=resource_uuid;
  UPDATE applications.logical_tables SET dependency_revision=dependency_revision+1 WHERE app_id=app_uuid AND id=table_uuid;
 END IF;
 UPDATE applications.apps SET structure_version=structure_version+1 WHERE id=app_uuid;
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
DROP FUNCTION applications.delete_structure_resource(uuid,text,uuid,uuid,uuid,bigint,bigint);
DROP TABLE applications.structure_deletions;
ALTER TABLE applications.form_views DROP COLUMN deleted_at;
ALTER TABLE applications.logical_tables DROP COLUMN deleted_at;
ALTER TABLE applications.directories DROP COLUMN deleted_at;
ALTER TABLE applications.apps DROP COLUMN deleted_at;
