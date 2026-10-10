-- +goose Up
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
  'workflow.task.return','workflow.manual.start','workflow.delete','workflow.round.resubmit','workflow.round.review','application.template.import'
 ));

ALTER TABLE applications.operations ADD CONSTRAINT ck_template_import_result CHECK(
 result_json IS NULL OR operation_kind<>'application.template.import' OR COALESCE(
 operation_id<>'00000000-0000-0000-0000-000000000000'
 AND app_id<>'00000000-0000-0000-0000-000000000000'
 AND result_json ?& ARRAY['operationId','appId','policyRevision','structureVersion','status']
 AND result_json-ARRAY['operationId','appId','policyRevision','structureVersion','status']='{}'::jsonb
 AND jsonb_typeof(result_json->'operationId')='string' AND result_json->>'operationId'=operation_id::text
 AND jsonb_typeof(result_json->'appId')='string' AND result_json->>'appId'=app_id::text
 AND result_json->'policyRevision'='1'::jsonb
 AND result_json->'structureVersion'='1'::jsonb
 AND result_json->>'status'='imported'
 AND http_status=201 AND location='/api/v1/applications/'||app_id::text,false));

-- +goose Down
LOCK TABLE applications.operations IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications.operations WHERE operation_kind='application.template.import') THEN
  RAISE EXCEPTION USING ERRCODE='55000',MESSAGE='durable template import receipt blocks Down';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE applications.operations DROP CONSTRAINT ck_template_import_result;
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
  'workflow.task.return','workflow.manual.start','workflow.delete','workflow.round.resubmit','workflow.round.review'
 ));
