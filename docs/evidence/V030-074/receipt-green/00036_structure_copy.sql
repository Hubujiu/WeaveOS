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
  'workflow.task.return','workflow.manual.start','workflow.delete','workflow.round.resubmit','workflow.round.review','application.template.import','preset.create','preset.update','preset.discard','record.delete','record.restore','application.delete','directory.delete','table.delete','form.delete','structure.copy'
 ));

ALTER TABLE applications.operations ADD CONSTRAINT ck_structure_copy_result CHECK(
 result_json IS NULL OR operation_kind<>'structure.copy' OR COALESCE(
 operation_id<>'00000000-0000-0000-0000-000000000000'
 AND app_id<>'00000000-0000-0000-0000-000000000000'
 AND jsonb_typeof(result_json)='object'
 AND result_json ?& ARRAY['operationId','appId','resourceKind','id','structureVersion','policyRevision','status']
 AND result_json-ARRAY['operationId','appId','resourceKind','id','structureVersion','policyRevision','status']='{}'::jsonb
 AND jsonb_typeof(result_json->'operationId')='string' AND result_json->>'operationId'=operation_id::text
 AND jsonb_typeof(result_json->'appId')='string' AND result_json->>'appId'=app_id::text
 AND jsonb_typeof(result_json->'resourceKind')='string' AND result_json->>'resourceKind' IN ('application','directory','table','form')
 AND jsonb_typeof(result_json->'id')='string'
 AND (result_json->>'id') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
 AND result_json->>'id'<>'00000000-0000-0000-0000-000000000000'
 AND (result_json->>'resourceKind'<>'application' OR result_json->>'id'=app_id::text)
 AND CASE WHEN jsonb_typeof(result_json->'structureVersion')='number' AND (result_json->>'structureVersion') ~ '^[0-9]+$' THEN (result_json->>'structureVersion')::numeric BETWEEN 1 AND 9007199254740991 ELSE false END
 AND CASE WHEN jsonb_typeof(result_json->'policyRevision')='number' AND (result_json->>'policyRevision') ~ '^[0-9]+$' THEN (result_json->>'policyRevision')::numeric BETWEEN 1 AND 9007199254740991 ELSE false END
 AND result_json->'status'='"copied"'::jsonb
 AND http_status=201
 AND location='/api/v1/applications/'||app_id::text||CASE result_json->>'resourceKind'
  WHEN 'application' THEN ''
  WHEN 'directory' THEN '/directories/'||(result_json->>'id')
  WHEN 'table' THEN '/tables/'||(result_json->>'id')
  WHEN 'form' THEN '/forms/'||(result_json->>'id') END,false));

-- +goose Down
-- No-behavior placeholder until independent real Down RED.
SELECT 1;
