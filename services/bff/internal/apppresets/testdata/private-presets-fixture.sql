-- V071 isolated test fixture ONLY. Not a production migration or a V070 dependency.
-- Frozen Notion data contract; final numbered migration must be rebased onto
-- actually integrated hot32 and revalidated independently before delivery.
CREATE TABLE applications.table_presets (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 owner_user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 app_id uuid NOT NULL,
 view_id uuid NOT NULL,
 name text COLLATE "C" NOT NULL,
 slot smallint NOT NULL,
 definition_json text NOT NULL,
 field_kinds jsonb NOT NULL,
 version bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT fk_application_presets_view FOREIGN KEY(app_id,view_id) REFERENCES applications.form_views(app_id,id) ON DELETE RESTRICT,
 CONSTRAINT uq_application_presets_name UNIQUE(owner_user_id,app_id,view_id,name),
 CONSTRAINT uq_application_presets_slot UNIQUE(owner_user_id,app_id,view_id,slot),
 CONSTRAINT ck_application_presets_slot CHECK(slot BETWEEN 1 AND 20),
 CONSTRAINT ck_application_presets_name CHECK(char_length(name) BETWEEN 1 AND 100 AND name=btrim(name)),
 CONSTRAINT ck_application_presets_version CHECK(version BETWEEN 1 AND 9007199254740991),
 CONSTRAINT ck_application_presets_definition CHECK(COALESCE(
  octet_length(definition_json)<=32768 AND jsonb_typeof(definition_json::jsonb)='object'
  AND definition_json::jsonb ?& ARRAY['name','filter','sort','hiddenColumnIds','columnOrder','columnWidths']
  AND definition_json::jsonb-ARRAY['name','filter','sort','hiddenColumnIds','columnOrder','columnWidths']='{}'::jsonb
  AND jsonb_typeof(definition_json::jsonb->'name')='string'
  AND definition_json::jsonb->>'name'=name
  AND jsonb_typeof(definition_json::jsonb->'filter') IN ('null','object')
  AND jsonb_typeof(definition_json::jsonb->'sort') IN ('null','object')
  AND jsonb_typeof(definition_json::jsonb->'hiddenColumnIds')='array'
  AND jsonb_typeof(definition_json::jsonb->'columnOrder')='array'
  AND jsonb_typeof(definition_json::jsonb->'columnWidths')='object',false)),
 CONSTRAINT ck_application_presets_kinds CHECK(jsonb_typeof(field_kinds)='object' AND octet_length(field_kinds::text)<=16384 AND jsonb_array_length(jsonb_path_query_array(field_kinds,'$.keyvalue()'))<=200)
);
CREATE INDEX ix_application_presets_private_updated ON applications.table_presets(owner_user_id,app_id,view_id,updated_at DESC,id ASC);
REVOKE ALL ON applications.table_presets FROM PUBLIC;

-- Preserve the actual fixture baseline's finite kind constraint. Production
-- migration will carry a literal reviewed old+new list, never dynamic SQL.
DO $$ DECLARE prior text; BEGIN
 SELECT pg_get_constraintdef(oid) INTO STRICT prior FROM pg_constraint WHERE conrelid='applications.operations'::regclass AND conname='ck_operation_kind';
 ALTER TABLE applications.operations DROP CONSTRAINT ck_operation_kind;
 EXECUTE 'ALTER TABLE applications.operations ADD CONSTRAINT ck_operation_kind CHECK (('||substring(prior FROM 7)||') OR operation_kind IN (''preset.create'',''preset.update'',''preset.discard''))';
END $$;
ALTER TABLE applications.operations ADD CONSTRAINT ck_application_preset_result CHECK(
 result_json IS NULL OR operation_kind NOT IN ('preset.create','preset.update','preset.discard') OR COALESCE(
 operation_id<>'00000000-0000-0000-0000-000000000000'
 AND result_json ?& ARRAY['operationId','id','version']
 AND result_json-ARRAY['operationId','id','version']='{}'::jsonb
 AND jsonb_typeof(result_json->'operationId')='string' AND result_json->>'operationId'=operation_id::text
 AND jsonb_typeof(result_json->'id')='string' AND result_json->>'id'~'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' AND result_json->>'id'<>'00000000-0000-0000-0000-000000000000'
 AND jsonb_typeof(result_json->'version')='number' AND result_json->>'version'~'^[1-9][0-9]{0,15}$' AND (result_json->>'version')::numeric<=9007199254740991
 AND ((operation_kind='preset.create' AND http_status=201 AND result_json->>'version'='1' AND location~('^/api/v1/applications/'||app_id::text||'/forms/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/table-presets/'||(result_json->>'id')||'$'))
  OR (operation_kind='preset.update' AND http_status=200 AND location='')
  OR (operation_kind='preset.discard' AND http_status=204 AND location='')),false));
