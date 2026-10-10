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
  'workflow.task.return','workflow.manual.start','workflow.delete','workflow.round.resubmit','workflow.round.review'
 ));


ALTER TABLE applications.operations
 DROP CONSTRAINT ck_operation_result,
 ADD CONSTRAINT ck_operation_result CHECK(
  (result_json IS NULL AND http_status IS NULL AND location IS NULL) OR
  (result_json IS NOT NULL AND jsonb_typeof(result_json)='object' AND http_status IS NOT NULL
   AND (http_status IN (200,201,204) OR
        (http_status=202 AND operation_kind IN ('workflow.task.agree','workflow.task.reject','workflow.instance.withdraw','workflow.task.return','workflow.manual.start','workflow.delete','workflow.round.resubmit','workflow.round.review')))
   AND location IS NOT NULL));


ALTER TABLE applications.operations ADD CONSTRAINT ck_workflow_round_result CHECK(
 result_json IS NULL OR operation_kind NOT IN ('workflow.round.resubmit','workflow.round.review') OR COALESCE(
 operation_id<>'00000000-0000-0000-0000-000000000000'
 AND result_json ?& ARRAY['operationId','flowId','previousInstanceId','instanceId','roundKind','roundNumber','status']
 AND result_json-ARRAY['operationId','flowId','previousInstanceId','instanceId','roundKind','roundNumber','status']='{}'::jsonb
 AND jsonb_typeof(result_json->'operationId')='string' AND result_json->>'operationId'=operation_id::text
 AND result_json->>'status'='accepted'
 AND ((operation_kind='workflow.round.resubmit' AND result_json->>'roundKind'='resubmit') OR (operation_kind='workflow.round.review' AND result_json->>'roundKind'='review'))
 AND jsonb_typeof(result_json->'roundNumber')='number'
 AND result_json->>'roundNumber' ~ '^[1-9][0-9]{0,15}$'
 AND (result_json->>'roundNumber')::numeric<=9007199254740991
 AND result_json->>'previousInstanceId'<>result_json->>'instanceId'
 AND jsonb_typeof(result_json->'flowId')='string' AND (result_json->>'flowId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' AND result_json->>'flowId'<>'00000000-0000-0000-0000-000000000000'
 AND jsonb_typeof(result_json->'previousInstanceId')='string' AND (result_json->>'previousInstanceId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' AND result_json->>'previousInstanceId'<>'00000000-0000-0000-0000-000000000000'
 AND jsonb_typeof(result_json->'instanceId')='string' AND (result_json->>'instanceId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' AND result_json->>'instanceId'<>'00000000-0000-0000-0000-000000000000'
 AND http_status=202 AND location='/api/v1/application-operations/'||operation_id::text,false));

-- +goose Down
LOCK TABLE applications.operations IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications.operations WHERE operation_kind IN ('workflow.round.resubmit','workflow.round.review')) THEN
  RAISE EXCEPTION USING ERRCODE='55000',MESSAGE='durable workflow round acceptance blocks Down';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE applications.operations DROP CONSTRAINT ck_workflow_round_result;
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
  'workflow.task.return','workflow.manual.start','workflow.delete'
 ));


ALTER TABLE applications.operations
 DROP CONSTRAINT ck_operation_result,
 ADD CONSTRAINT ck_operation_result CHECK(
  (result_json IS NULL AND http_status IS NULL AND location IS NULL) OR
  (result_json IS NOT NULL AND jsonb_typeof(result_json)='object' AND http_status IS NOT NULL
   AND (http_status IN (200,201,204) OR
        (http_status=202 AND operation_kind IN ('workflow.task.agree','workflow.task.reject','workflow.instance.withdraw','workflow.task.return','workflow.manual.start','workflow.delete')))
   AND location IS NOT NULL));


