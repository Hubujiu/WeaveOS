-- +goose Up
-- Frozen new-reference rule: INSERT includes omitted constant defaults.
-- The caller protects live sources before app/table/row gates; this function
-- performs same-transaction validation without acquiring reversed source locks.
-- +goose StatementBegin
CREATE FUNCTION applications.validate_effective_record_references(app_uuid uuid,table_uuid uuid,record_uuid uuid,
 operation text,field_values jsonb) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE entry record; previous jsonb; actual_status text; registry_status text; qualified text;
BEGIN
 qualified:=format('appdata.%I','t_'||replace(table_uuid::text,'-',''));
 FOR entry IN SELECT f.id,f.definition,f.definition->>'kind' AS kind,
   CASE WHEN field_values ? f.id::text THEN field_values->f.id::text ELSE f.definition->'default' END AS value
  FROM applications.fields f WHERE f.app_id=app_uuid AND f.table_id=table_uuid AND NOT f.removed
   AND f.definition->>'kind' IN ('member','department') AND (operation='insert' OR field_values ? f.id::text)
  ORDER BY f.definition->>'kind',f.id LOOP
  IF NOT EXISTS(SELECT 1 FROM applications.reference_source_revision WHERE singleton) THEN
   RAISE EXCEPTION 'reference sources unavailable' USING ERRCODE='W0004';
  END IF;
  IF to_regclass('applications.'||CASE WHEN entry.kind='member' THEN 'member_sources' ELSE 'department_sources' END) IS NULL THEN
   RAISE EXCEPTION 'reference registry unavailable' USING ERRCODE='W0004';
  END IF;
  IF entry.value IS NULL OR NOT applications.canonical_record_value(entry.definition,entry.value) THEN
   RAISE EXCEPTION 'invalid reference value' USING ERRCODE='23514';
  END IF;
  IF entry.value='null'::jsonb THEN CONTINUE;END IF;
  IF operation='update' THEN
   EXECUTE format('SELECT to_jsonb(%I) FROM %s WHERE id=$1','f_'||replace(entry.id::text,'-',''),qualified)
    INTO previous USING record_uuid;
   IF previous=entry.value THEN CONTINUE;END IF;
  END IF;
  IF entry.kind='member' THEN
   SELECT u.status,s.status INTO actual_status,registry_status FROM auth.users u
    LEFT JOIN applications.member_sources s ON s.id=u.id WHERE u.id=(entry.value#>>'{}')::uuid;
   IF actual_status IS DISTINCT FROM 'active' THEN RAISE EXCEPTION 'inactive new member reference' USING ERRCODE='23514';END IF;
  ELSE
   SELECT 'active',s.status INTO actual_status,registry_status FROM personnel.departments d
    LEFT JOIN applications.department_sources s ON s.id=d.id WHERE d.id=(entry.value#>>'{}')::uuid;
   IF actual_status IS DISTINCT FROM 'active' THEN RAISE EXCEPTION 'deleted new department reference' USING ERRCODE='23514';END IF;
  END IF;
  IF registry_status IS DISTINCT FROM 'active' THEN RAISE EXCEPTION 'reference registry inconsistent' USING ERRCODE='W0004';END IF;
 END LOOP;
EXCEPTION WHEN undefined_table OR undefined_column THEN
 RAISE EXCEPTION 'reference sources unavailable' USING ERRCODE='W0004';
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.validate_effective_record_references(uuid,uuid,uuid,text,jsonb) FROM PUBLIC;
-- Existing capability signature/privileges remain finite and unchanged.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION applications.apply_record_change(app_uuid uuid,view_uuid uuid,table_uuid uuid,record_uuid uuid,actor_uuid uuid,
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
  PERFORM applications.validate_effective_record_references(app_uuid,table_uuid,record_uuid,operation,field_values);
  EXECUTE format('INSERT INTO %s(id,created_by%s) SELECT $1,$2%s FROM jsonb_populate_record(NULL::%s,$3) AS canonical RETURNING jsonb_build_object(''id'',id,''createdBy'',created_by,''recordVersion'',record_version,''createdAt'',created_at,''updatedAt'',updated_at)',qualified,columns_sql,select_sql,qualified) INTO header USING record_uuid,actor_uuid,physical_values;
 ELSE
  IF expected_record IS NULL OR expected_record NOT BETWEEN 1 AND 9007199254740991 THEN RAISE EXCEPTION 'invalid record version' USING ERRCODE='23514';END IF;
  EXECUTE format('SELECT jsonb_build_object(''id'',id,''createdBy'',created_by,''recordVersion'',record_version,''createdAt'',created_at,''updatedAt'',updated_at) FROM %s WHERE id=$1 FOR UPDATE',qualified) INTO header USING record_uuid;
  IF header IS NULL THEN RAISE EXCEPTION 'record missing' USING ERRCODE='P0002';END IF;
  IF (header->>'recordVersion')::bigint<>expected_record THEN RAISE EXCEPTION 'record conflict' USING ERRCODE='W0003';END IF;
  PERFORM applications.validate_effective_record_references(app_uuid,table_uuid,record_uuid,operation,field_values);
  IF assign_sql<>'' THEN
   EXECUTE format('UPDATE %s AS stored SET %s,record_version=stored.record_version+1,updated_at=clock_timestamp() FROM jsonb_populate_record(NULL::%s,$2) AS canonical WHERE stored.id=$1 AND stored.record_version=$3 RETURNING jsonb_build_object(''id'',stored.id,''createdBy'',stored.created_by,''recordVersion'',stored.record_version,''createdAt'',stored.created_at,''updatedAt'',stored.updated_at)',qualified,assign_sql,qualified) INTO header USING record_uuid,physical_values,expected_record;
   IF header IS NULL THEN RAISE EXCEPTION 'record conflict' USING ERRCODE='W0003';END IF;
  END IF;
 END IF;
 RETURN header;
END $$;
-- +goose StatementEnd
-- No destructive Down or new runtime source-write privilege.
