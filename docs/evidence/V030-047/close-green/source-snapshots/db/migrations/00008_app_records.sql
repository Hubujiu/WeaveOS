-- +goose Up
-- Frozen V015 ADR §8 and §10.1. Trusted BFF Session boundary, no DB actor key.
-- Additive hot8; prior migration bytes and value-history decisions stay intact.
ALTER TABLE applications.grants DROP CONSTRAINT grants_action_check;
ALTER TABLE applications.grants DROP CONSTRAINT grants_row_scope_check;
ALTER TABLE applications.grants ADD CONSTRAINT ck_record_grant_tuple CHECK(
 (action='menu.enter' AND row_scope='all') OR
 (resource_kind='form' AND ((action='data.create' AND row_scope='all') OR
 (action IN ('data.read','data.edit','data.history') AND row_scope IN ('all','own'))))
);
ALTER TABLE applications.grant_fields DROP CONSTRAINT ck_b5_no_grant_fields;
ALTER TABLE applications.grant_fields ADD COLUMN table_id uuid NOT NULL;
ALTER TABLE applications.grant_fields ADD CONSTRAINT fk_grant_field_registry
 FOREIGN KEY(app_id,table_id,field_id) REFERENCES applications.fields(app_id,table_id,id) ON DELETE RESTRICT;
ALTER TABLE applications.form_views ADD CONSTRAINT uq_form_table_binding UNIQUE(app_id,table_id,id);
-- +goose StatementBegin
CREATE FUNCTION applications.validate_data_grant_field() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.apps WHERE id=NEW.app_id FOR UPDATE;
 IF NOT EXISTS(SELECT 1 FROM applications.grants g
 JOIN applications.form_views v ON v.app_id=g.app_id AND v.id=g.resource_id AND g.resource_kind='form'
 JOIN applications.fields f ON f.app_id=v.app_id AND f.table_id=v.table_id AND f.id=NEW.field_id AND NOT f.removed
 WHERE g.app_id=NEW.app_id AND g.id=NEW.grant_id AND g.action IN ('data.create','data.read','data.edit','data.history') AND v.table_id=NEW.table_id)
 THEN RAISE EXCEPTION 'invalid grant field binding' USING ERRCODE='23503';END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER valid_data_grant_field BEFORE INSERT OR UPDATE ON applications.grant_fields FOR EACH ROW EXECUTE FUNCTION applications.validate_data_grant_field();
-- +goose StatementBegin
CREATE FUNCTION applications.protect_granted_field() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF (TG_OP='DELETE' OR NEW.removed) AND EXISTS(SELECT 1 FROM applications.grant_fields WHERE app_id=OLD.app_id AND table_id=OLD.table_id AND field_id=OLD.id)
 THEN RAISE EXCEPTION 'revoke field grants before removal' USING ERRCODE='23514';END IF;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER protect_granted_field BEFORE UPDATE OF removed OR DELETE ON applications.fields FOR EACH ROW EXECUTE FUNCTION applications.protect_granted_field();

CREATE TABLE applications.record_drafts(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),owner_user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 app_id uuid NOT NULL,table_id uuid NOT NULL,view_id uuid NOT NULL,target_record_id uuid,
 schema_version bigint NOT NULL CHECK(schema_version BETWEEN 0 AND 9007199254740991),
 base_record_version bigint CHECK(base_record_version BETWEEN 1 AND 9007199254740991),
 draft_version bigint NOT NULL DEFAULT 1 CHECK(draft_version BETWEEN 1 AND 9007199254740991),
 values_json jsonb NOT NULL CHECK(jsonb_typeof(values_json)='object'),
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK((target_record_id IS NULL)=(base_record_version IS NULL)),
 FOREIGN KEY(app_id,table_id,view_id) REFERENCES applications.form_views(app_id,table_id,id) ON DELETE RESTRICT
);
CREATE INDEX ix_record_draft_owner_cursor ON applications.record_drafts(owner_user_id,app_id,view_id,updated_at DESC,id DESC);

CREATE TABLE applications.record_write_audit(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),actor_user_id uuid NOT NULL,
 app_id uuid NOT NULL,table_id uuid NOT NULL,view_id uuid NOT NULL,record_id uuid NOT NULL,operation_id uuid NOT NULL,
 before_record_version bigint NOT NULL CHECK(before_record_version BETWEEN 0 AND 9007199254740991),
 after_record_version bigint NOT NULL CHECK(after_record_version BETWEEN 1 AND 9007199254740991 AND after_record_version>=before_record_version),
 changed_field_ids uuid[] NOT NULL,origin varchar(24) NOT NULL CHECK(origin='ordinary'),
 request_id varchar(64) NOT NULL,occurred_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(actor_user_id,operation_id),
 FOREIGN KEY(app_id,table_id,view_id) REFERENCES applications.form_views(app_id,table_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(actor_user_id,operation_id) REFERENCES applications.operations(actor_user_id,operation_id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX ix_record_audit_resource ON applications.record_write_audit(app_id,table_id,record_id,occurred_at DESC,id DESC);
CREATE TABLE applications.record_command_fences(
 app_id uuid NOT NULL,table_id uuid NOT NULL,record_id uuid NOT NULL,command_id uuid NOT NULL,
 expected_record_version bigint NOT NULL CHECK(expected_record_version BETWEEN 1 AND 9007199254740991),
 state varchar(16) NOT NULL CHECK(state='pending'),created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(app_id,table_id,record_id,command_id),
 FOREIGN KEY(app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT
);
-- Future command owner transitions are deliberately not invented here.
-- +goose StatementBegin
CREATE FUNCTION applications.record_fence_gate() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM applications.logical_tables WHERE app_id=COALESCE(NEW.app_id,OLD.app_id) AND id=COALESCE(NEW.table_id,OLD.table_id) FOR UPDATE;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER record_fence_gate BEFORE INSERT OR UPDATE OR DELETE ON applications.record_command_fences FOR EACH ROW EXECUTE FUNCTION applications.record_fence_gate();

ALTER TABLE applications.operations DROP CONSTRAINT ck_operation_kind;
ALTER TABLE applications.operations ADD CONSTRAINT ck_operation_kind CHECK(operation_kind IN (
 'application.create','group.create','group.update','members.replace','grants.replace',
 'directory.create','directory.update','table.create','table.update','form.create','form.update','definition.save',
 'record.create','record.edit','draft.create','draft.update','draft.discard'));
ALTER TABLE applications.operations DROP CONSTRAINT operations_check;
ALTER TABLE applications.operations ADD CONSTRAINT ck_operation_result CHECK(
 (result_json IS NULL AND http_status IS NULL AND location IS NULL) OR
 (result_json IS NOT NULL AND jsonb_typeof(result_json)='object' AND http_status IS NOT NULL AND http_status IN (200,201,204) AND location IS NOT NULL));
ALTER TABLE applications.operations ADD CONSTRAINT ck_record_minimum_result CHECK(
 result_json IS NULL OR operation_kind NOT IN ('record.create','record.edit','draft.create','draft.update','draft.discard') OR
 CASE WHEN operation_kind IN ('record.create','record.edit') THEN
 COALESCE(result_json ?& ARRAY['operationId','id','recordVersion','schemaVersion','createdAt','updatedAt']
 AND result_json-ARRAY['operationId','id','recordVersion','schemaVersion','createdAt','updatedAt']='{}'::jsonb
 AND result_json->>'operationId'=operation_id::text AND jsonb_typeof(result_json->'id')='string'
 AND jsonb_typeof(result_json->'recordVersion')='number' AND jsonb_typeof(result_json->'schemaVersion')='number'
 AND jsonb_typeof(result_json->'createdAt')='string' AND jsonb_typeof(result_json->'updatedAt')='string',false)
 ELSE COALESCE(result_json ?& ARRAY['operationId','id','draftVersion']
 AND result_json-ARRAY['operationId','id','draftVersion']='{}'::jsonb
 AND result_json->>'operationId'=operation_id::text AND jsonb_typeof(result_json->'id')='string'
 AND jsonb_typeof(result_json->'draftVersion')='number',false) END);

-- Real source hooks cover all SQL writers, with last labels retained on deletion.
CREATE TABLE applications.member_sources(id uuid PRIMARY KEY,label varchar(254) NOT NULL,status varchar(16) NOT NULL CHECK(status IN ('active','disabled','deleted')));
CREATE TABLE applications.department_sources(id uuid PRIMARY KEY,label varchar(100) NOT NULL,parent_id uuid,status varchar(16) NOT NULL CHECK(status IN ('active','deleted')));
CREATE TABLE applications.member_department_sources(member_id uuid NOT NULL REFERENCES applications.member_sources(id),department_id uuid NOT NULL REFERENCES applications.department_sources(id),PRIMARY KEY(member_id,department_id));
CREATE INDEX ix_source_department_members ON applications.member_department_sources(department_id,member_id);
CREATE TABLE applications.reference_source_revision(singleton boolean PRIMARY KEY CHECK(singleton),revision bigint NOT NULL CHECK(revision BETWEEN 0 AND 9007199254740991));
INSERT INTO applications.reference_source_revision VALUES(true,0);
INSERT INTO applications.member_sources SELECT id,account,status FROM auth.users;
INSERT INTO applications.department_sources SELECT id,name,parent_id,'active' FROM personnel.departments;
INSERT INTO applications.member_department_sources SELECT user_id,department_id FROM personnel.department_members;
-- +goose StatementBegin
CREATE FUNCTION applications.sync_reference_source() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_TABLE_SCHEMA='auth' THEN
  IF TG_OP='DELETE' THEN UPDATE applications.member_sources SET status='deleted' WHERE id=OLD.id;
  ELSE
   INSERT INTO applications.member_sources(id,label,status) VALUES(NEW.id,NEW.account,NEW.status)
   ON CONFLICT(id) DO UPDATE SET label=EXCLUDED.label,status=EXCLUDED.status WHERE applications.member_sources.status<>'deleted';
   IF NOT FOUND THEN RAISE EXCEPTION 'source ID cannot be reused' USING ERRCODE='23514';END IF;
  END IF;
 ELSIF TG_TABLE_NAME='departments' THEN
  IF TG_OP='DELETE' THEN UPDATE applications.department_sources SET status='deleted' WHERE id=OLD.id;
  ELSE
   INSERT INTO applications.department_sources(id,label,parent_id,status) VALUES(NEW.id,NEW.name,NEW.parent_id,'active')
   ON CONFLICT(id) DO UPDATE SET label=EXCLUDED.label,parent_id=EXCLUDED.parent_id WHERE applications.department_sources.status<>'deleted';
   IF NOT FOUND THEN RAISE EXCEPTION 'source ID cannot be reused' USING ERRCODE='23514';END IF;
  END IF;
 ELSE
  IF TG_OP IN ('DELETE','UPDATE') THEN DELETE FROM applications.member_department_sources WHERE member_id=OLD.user_id AND department_id=OLD.department_id;END IF;
  IF TG_OP IN ('INSERT','UPDATE') THEN INSERT INTO applications.member_department_sources VALUES(NEW.user_id,NEW.department_id) ON CONFLICT DO NOTHING;END IF;
 END IF;
 UPDATE applications.reference_source_revision SET revision=revision+1 WHERE singleton;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER member_reference_source AFTER INSERT OR UPDATE OF account,status OR DELETE ON auth.users FOR EACH ROW EXECUTE FUNCTION applications.sync_reference_source();
CREATE TRIGGER department_reference_source AFTER INSERT OR UPDATE OF name,parent_id OR DELETE ON personnel.departments FOR EACH ROW EXECUTE FUNCTION applications.sync_reference_source();
CREATE TRIGGER member_department_reference_source AFTER INSERT OR UPDATE OR DELETE ON personnel.department_members FOR EACH ROW EXECUTE FUNCTION applications.sync_reference_source();

-- Strict canonical-value validation is repeated inside the finite DB capability.
-- +goose StatementBegin
CREATE FUNCTION applications.canonical_record_value(definition jsonb,value jsonb) RETURNS boolean
LANGUAGE plpgsql STABLE SET search_path=pg_catalog AS $$
DECLARE kind text:=definition->>'kind';config jsonb:=definition->'config';s text:=value#>>'{}';expected text;option_values jsonb;
BEGIN
 IF value='null'::jsonb THEN RETURN NOT (definition->>'required')::boolean;END IF;
 CASE kind
 WHEN 'text','multiline' THEN RETURN jsonb_typeof(value)='string' AND (config->>'maxLength' IS NULL OR char_length(s)<=(config->>'maxLength')::integer);
 WHEN 'number','money' THEN
  IF jsonb_typeof(value)<>'string' OR s!~'^-?(0|[1-9][0-9]*)(\.[0-9]+)?$' THEN RETURN false;END IF;
  EXECUTE format('SELECT applications.round_decimal($1::numeric,$2,$3)::numeric(%s,%s)::text',(config->>'precision')::integer,(config->>'scale')::integer)
   INTO expected USING s,(config->>'roundingPlaces')::integer,config->>'roundingMode';
  RETURN s=expected;
 WHEN 'date' THEN RETURN jsonb_typeof(value)='string' AND s~'^[0-9]{4}-[0-9]{2}-[0-9]{2}$' AND to_char(s::date,'YYYY-MM-DD')=s;
 WHEN 'datetime' THEN
  IF jsonb_typeof(value)<>'string' THEN RETURN false;END IF;
  expected:=to_char(date_trunc(CASE config->>'precision' WHEN 'millisecond' THEN 'milliseconds' ELSE config->>'precision' END,s::timestamptz,'UTC') AT TIME ZONE 'UTC',
   CASE config->>'precision' WHEN 'millisecond' THEN 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"' ELSE 'YYYY-MM-DD"T"HH24:MI:SS"Z"' END);
  RETURN s=expected;
 WHEN 'boolean' THEN RETURN jsonb_typeof(value)='boolean';
 WHEN 'member','department' THEN RETURN jsonb_typeof(value)='string' AND s=(s::uuid)::text;
 WHEN 'single_select' THEN RETURN jsonb_typeof(value)='string' AND s=(s::uuid)::text AND EXISTS(SELECT 1 FROM jsonb_array_elements(config->'options') o WHERE o->>'id'=s);
 WHEN 'multi_select' THEN
  IF jsonb_typeof(value)<>'array' OR EXISTS(SELECT 1 FROM jsonb_array_elements(value) v WHERE jsonb_typeof(v)<>'string') THEN RETURN false;END IF;
  SELECT COALESCE(jsonb_agg(o->'id' ORDER BY ordering),'[]'::jsonb) INTO option_values FROM jsonb_array_elements(config->'options') WITH ORDINALITY AS options(o,ordering) WHERE value ? (o->>'id');
  RETURN value=option_values;
 ELSE RETURN false;
 END CASE;
EXCEPTION WHEN data_exception THEN RETURN false;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.apply_record_change(app_uuid uuid,view_uuid uuid,table_uuid uuid,record_uuid uuid,actor_uuid uuid,
 operation text,expected_schema bigint,expected_record bigint,field_values jsonb) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE qualified text;ready boolean;schema_version bigint;entry record;physical_values jsonb:='{}';columns_sql text:='';select_sql text:='';assign_sql text:='';column_name text;header jsonb;
BEGIN
 IF operation IS NULL OR operation NOT IN ('lock_header','insert','update') OR app_uuid IS NULL OR view_uuid IS NULL OR table_uuid IS NULL OR record_uuid IS NULL OR expected_schema IS NULL OR expected_schema NOT BETWEEN 0 AND 9007199254740991 THEN RAISE EXCEPTION 'invalid record capability' USING ERRCODE='23514';END IF;
 SELECT t.schema_ready,t.schema_version INTO ready,schema_version FROM applications.logical_tables t
 JOIN applications.form_views v ON v.app_id=t.app_id AND v.table_id=t.id AND v.id=view_uuid
 JOIN applications.menu_resources r ON r.app_id=v.app_id AND r.resource_kind='form' AND r.resource_id=v.id
 WHERE t.app_id=app_uuid AND t.id=table_uuid FOR UPDATE OF t;
 IF NOT FOUND THEN RAISE EXCEPTION 'unregistered table/view' USING ERRCODE='23503';END IF;
 IF NOT ready THEN RAISE EXCEPTION 'schema not ready' USING ERRCODE='W0001';END IF;
 IF schema_version<>expected_schema THEN RAISE EXCEPTION 'schema conflict' USING ERRCODE='W0002';END IF;
 qualified:=format('appdata.%I','t_'||replace(table_uuid::text,'-',''));
 IF operation='lock_header' THEN
  EXECUTE format('SELECT jsonb_build_object(''id'',id,''createdBy'',created_by,''recordVersion'',record_version,''createdAt'',created_at,''updatedAt'',updated_at) FROM %s WHERE id=$1 FOR UPDATE',qualified) INTO header USING record_uuid;
  IF header IS NULL THEN RAISE EXCEPTION 'record missing' USING ERRCODE='P0002';END IF;
  RETURN header;
 END IF;
 IF actor_uuid IS NULL OR field_values IS NULL OR jsonb_typeof(field_values)<>'object' THEN RAISE EXCEPTION 'invalid values' USING ERRCODE='23514';END IF;
 FOR entry IN SELECT v.key,v.value,f.definition FROM jsonb_each(field_values) v LEFT JOIN applications.fields f ON f.app_id=app_uuid AND f.table_id=table_uuid AND f.id::text=v.key AND NOT f.removed ORDER BY v.key LOOP
  IF entry.definition IS NULL OR NOT applications.canonical_record_value(entry.definition,entry.value) THEN RAISE EXCEPTION 'invalid canonical field' USING ERRCODE='23514';END IF;
  column_name:='f_'||replace(entry.key,'-','');
  physical_values:=physical_values||jsonb_build_object(column_name,entry.value);
  columns_sql:=columns_sql||format(',%I',column_name);
  select_sql:=select_sql||format(',canonical.%I',column_name);
  assign_sql:=assign_sql||CASE WHEN assign_sql='' THEN '' ELSE ',' END||format('%I=canonical.%I',column_name,column_name);
 END LOOP;
 IF operation='insert' THEN
  EXECUTE format('INSERT INTO %s(id,created_by%s) SELECT $1,$2%s FROM jsonb_populate_record(NULL::%s,$3) AS canonical RETURNING jsonb_build_object(''id'',id,''createdBy'',created_by,''recordVersion'',record_version,''createdAt'',created_at,''updatedAt'',updated_at)',qualified,columns_sql,select_sql,qualified) INTO header USING record_uuid,actor_uuid,physical_values;
 ELSE
  IF expected_record IS NULL OR expected_record NOT BETWEEN 1 AND 9007199254740991 THEN RAISE EXCEPTION 'invalid record version' USING ERRCODE='23514';END IF;
  EXECUTE format('SELECT jsonb_build_object(''id'',id,''createdBy'',created_by,''recordVersion'',record_version,''createdAt'',created_at,''updatedAt'',updated_at) FROM %s WHERE id=$1 FOR UPDATE',qualified) INTO header USING record_uuid;
  IF header IS NULL THEN RAISE EXCEPTION 'record missing' USING ERRCODE='P0002';END IF;
  IF (header->>'recordVersion')::bigint<>expected_record THEN RAISE EXCEPTION 'record conflict' USING ERRCODE='W0003';END IF;
  IF assign_sql<>'' THEN
   EXECUTE format('UPDATE %s AS stored SET %s,record_version=stored.record_version+1,updated_at=clock_timestamp() FROM jsonb_populate_record(NULL::%s,$2) AS canonical WHERE stored.id=$1 AND stored.record_version=$3 RETURNING jsonb_build_object(''id'',stored.id,''createdBy'',stored.created_by,''recordVersion'',stored.record_version,''createdAt'',stored.created_at,''updatedAt'',stored.updated_at)',qualified,assign_sql,qualified) INTO header USING record_uuid,physical_values,expected_record;
   IF header IS NULL THEN RAISE EXCEPTION 'record conflict' USING ERRCODE='W0003';END IF;
  END IF;
 END IF;
 RETURN header;
END $$;
-- +goose StatementEnd
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA applications FROM PUBLIC;
-- Role policy is separately applied from reviewed infra/runtime/roles.sql.
-- No destructive Down and no cold value-history migration in this frozen slice.
