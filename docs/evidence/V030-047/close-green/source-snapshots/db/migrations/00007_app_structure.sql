-- +goose Up
-- V030-013 accepted finite contract. Earlier migration bytes remain immutable.
CREATE SCHEMA appdata;
ALTER TABLE applications.apps ADD COLUMN structure_version bigint NOT NULL DEFAULT 0 CHECK(structure_version BETWEEN 0 AND 9007199254740991);
CREATE TABLE applications.directories (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT,
 name varchar(100) NOT NULL CHECK(btrim(name)<>''), parent_id uuid, position integer NOT NULL CHECK(position>=0),
 UNIQUE(app_id,id), FOREIGN KEY(app_id,parent_id) REFERENCES applications.directories(app_id,id) ON DELETE RESTRICT,
 CHECK(parent_id IS NULL OR parent_id<>id)
);
CREATE TABLE applications.logical_tables (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT,
 name varchar(100) NOT NULL CHECK(btrim(name)<>''), directory_id uuid, position integer NOT NULL CHECK(position>=0),
 schema_version bigint NOT NULL DEFAULT 0 CHECK(schema_version BETWEEN 0 AND 9007199254740991),
 data_revision bigint NOT NULL DEFAULT 0 CHECK(data_revision BETWEEN 0 AND 9007199254740991),
 dependency_revision bigint NOT NULL DEFAULT 0 CHECK(dependency_revision BETWEEN 0 AND 9007199254740991),
 schema_ready boolean NOT NULL DEFAULT false, fields_json jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(fields_json)='array'),
 UNIQUE(app_id,id), FOREIGN KEY(app_id,directory_id) REFERENCES applications.directories(app_id,id) ON DELETE RESTRICT
);
CREATE TABLE applications.fields (
 id uuid PRIMARY KEY, app_id uuid NOT NULL, table_id uuid NOT NULL, definition jsonb NOT NULL CHECK(jsonb_typeof(definition)='object'),
 removed boolean NOT NULL DEFAULT false, UNIQUE(app_id,table_id,id),
 FOREIGN KEY(app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT
);
CREATE TABLE applications.form_views (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), app_id uuid NOT NULL, table_id uuid NOT NULL,
 name varchar(100) NOT NULL CHECK(btrim(name)<>''), directory_id uuid, position integer NOT NULL CHECK(position>=0),
 view_version bigint NOT NULL DEFAULT 0 CHECK(view_version BETWEEN 0 AND 9007199254740991),
 layout jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(layout)='array'), UNIQUE(app_id,id),
 FOREIGN KEY(app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(app_id,directory_id) REFERENCES applications.directories(app_id,id) ON DELETE RESTRICT
);
CREATE INDEX ix_form_views_table ON applications.form_views(app_id,table_id,id);
CREATE TABLE applications.table_field_dependencies (
 app_id uuid NOT NULL,table_id uuid NOT NULL,field_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('enabled_flow','in_flight')),resource_id uuid NOT NULL,
 PRIMARY KEY(app_id,table_id,field_id,kind,resource_id),
 FOREIGN KEY(app_id,table_id,field_id) REFERENCES applications.fields(app_id,table_id,id) ON DELETE RESTRICT
);
-- +goose StatementBegin
CREATE FUNCTION applications.directory_acyclic() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
DECLARE cyclic boolean;
BEGIN
 PERFORM 1 FROM applications.apps WHERE id=NEW.app_id FOR UPDATE;
 WITH RECURSIVE parents(id,path) AS (
  SELECT NEW.parent_id,ARRAY[NEW.id] UNION ALL
  SELECT d.parent_id,p.path||p.id FROM parents p JOIN applications.directories d ON d.id=p.id AND d.app_id=NEW.app_id
  WHERE p.id IS NOT NULL AND NOT p.id=ANY(p.path)
 ) SELECT EXISTS(SELECT 1 FROM parents WHERE id=ANY(path)) INTO cyclic;
 IF cyclic THEN RAISE EXCEPTION 'directory cycle' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER directory_acyclic BEFORE INSERT OR UPDATE ON applications.directories FOR EACH ROW EXECUTE FUNCTION applications.directory_acyclic();
ALTER TABLE applications.menu_resources DROP CONSTRAINT menu_resources_check;
ALTER TABLE applications.menu_resources ADD CONSTRAINT ck_menu_resource_kinds CHECK((resource_kind='application' AND resource_id=app_id) OR resource_kind IN ('directory','form'));
-- +goose StatementBegin
CREATE FUNCTION applications.validate_menu_resource() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.resource_kind='directory' AND NOT EXISTS(SELECT 1 FROM applications.directories WHERE app_id=NEW.app_id AND id=NEW.resource_id)
 OR NEW.resource_kind='form' AND NOT EXISTS(SELECT 1 FROM applications.form_views WHERE app_id=NEW.app_id AND id=NEW.resource_id) THEN
 RAISE EXCEPTION 'unregistered resource' USING ERRCODE='23503'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER real_menu_resource BEFORE INSERT OR UPDATE ON applications.menu_resources FOR EACH ROW EXECUTE FUNCTION applications.validate_menu_resource();
ALTER TABLE applications.operations DROP CONSTRAINT operations_operation_kind_check;
ALTER TABLE applications.operations ADD CONSTRAINT ck_operation_kind CHECK(operation_kind IN ('application.create','group.create','group.update','members.replace','grants.replace','directory.create','directory.update','table.create','table.update','form.create','form.update','definition.save'));
-- Dedicated value normalization in PG for typed DDL; independently tested against the same decimal oracle as Go.
-- +goose StatementBegin
CREATE FUNCTION applications.round_decimal(value numeric, places integer, mode text) RETURNS numeric LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog AS $$
DECLARE factor numeric:=power(10::numeric,places); scaled numeric; base numeric; remainder numeric;
BEGIN
 IF places NOT BETWEEN -18 AND 18 OR mode NOT IN ('HALF_UP','HALF_EVEN','TOWARD_ZERO','FLOOR','CEILING') THEN RAISE EXCEPTION 'invalid decimal policy' USING ERRCODE='23514'; END IF;
 scaled:=value*factor;base:=trunc(scaled);remainder:=abs(scaled-base);
 IF mode='HALF_UP' THEN RETURN round(value,places); ELSIF mode='FLOOR' THEN RETURN floor(scaled)/factor; ELSIF mode='CEILING' THEN RETURN ceil(scaled)/factor;
 ELSIF mode='HALF_EVEN' AND (remainder>0.5 OR remainder=0.5 AND mod(abs(base),2)=1) THEN base:=base+sign(value); END IF;
 RETURN base/factor;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.data_revision_changed() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 UPDATE applications.logical_tables SET data_revision=data_revision+1 WHERE id=TG_ARGV[0]::uuid;
 RETURN NULL;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION applications.dependency_revision_changed() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 UPDATE applications.logical_tables SET dependency_revision=dependency_revision+1 WHERE id=COALESCE(NEW.table_id,OLD.table_id);
 RETURN COALESCE(NEW,OLD);
END $$;
-- +goose StatementEnd
CREATE TRIGGER dependency_revision_changed AFTER INSERT OR UPDATE OR DELETE ON applications.table_field_dependencies FOR EACH ROW EXECUTE FUNCTION applications.dependency_revision_changed();
-- +goose StatementBegin
CREATE FUNCTION applications.view_dependency_changed() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='UPDATE' AND (NEW.table_id<>OLD.table_id OR NEW.app_id<>OLD.app_id) THEN RAISE EXCEPTION 'view table is immutable' USING ERRCODE='23514';END IF;
 IF TG_OP<>'UPDATE' OR NEW.layout IS DISTINCT FROM OLD.layout THEN
  UPDATE applications.logical_tables SET dependency_revision=dependency_revision+1 WHERE id=COALESCE(NEW.table_id,OLD.table_id);
 END IF;
 RETURN COALESCE(NEW,OLD);
END $$;
-- +goose StatementEnd
CREATE TRIGGER view_dependency_changed BEFORE INSERT OR UPDATE OR DELETE ON applications.form_views FOR EACH ROW EXECUTE FUNCTION applications.view_dependency_changed();
-- No arbitrary SQL argument. Only migration-owned typed primitives with canonical IDs.
-- +goose StatementBegin
CREATE FUNCTION applications.apply_schema_change(actor_uuid uuid,app_uuid uuid,table_uuid uuid,operation text,before_field jsonb,after_field jsonb) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE qualified text; field_uuid uuid; column_name text; typ text; default_sql text; expr text; precision_value integer; scale_value integer; places integer; mode text; old_typ text; required_value boolean;
BEGIN
 IF NOT EXISTS(SELECT 1 FROM auth.users u JOIN applications.apps a ON a.id=app_uuid WHERE u.id=actor_uuid AND u.status='active' AND (u.is_bootstrap_admin OR a.owner_user_id=u.id)) THEN RAISE EXCEPTION 'definition denied' USING ERRCODE='42501'; END IF;
 PERFORM 1 FROM applications.logical_tables WHERE id=table_uuid AND app_id=app_uuid FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'table owner mismatch' USING ERRCODE='23503'; END IF;
 qualified:=format('appdata.%I','t_'||replace(table_uuid::text,'-',''));
 IF operation='lock_table' THEN EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE',qualified);RETURN;END IF;
 IF operation='create_table' THEN
  EXECUTE format('CREATE TABLE %s(id uuid PRIMARY KEY,created_by uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),record_version bigint NOT NULL DEFAULT 1 CHECK(record_version BETWEEN 1 AND 9007199254740991))',qualified);
  EXECUTE format('GRANT SELECT ON %s TO auth_app,auth_backup',qualified);
  EXECUTE format('CREATE TRIGGER data_revision_changed AFTER INSERT OR UPDATE OR DELETE ON %s FOR EACH STATEMENT EXECUTE FUNCTION applications.data_revision_changed(%L)',qualified,table_uuid::text);
  RETURN;
 END IF;
 field_uuid:=COALESCE(after_field->>'ID',before_field->>'ID')::uuid;
 IF field_uuid IS NULL THEN RAISE EXCEPTION 'field ID required' USING ERRCODE='23514';END IF;
 IF EXISTS(SELECT 1 FROM applications.fields f WHERE f.id=field_uuid AND (f.app_id<>app_uuid OR f.table_id<>table_uuid OR f.removed AND operation='add_column')) THEN RAISE EXCEPTION 'field identity mismatch' USING ERRCODE='23503';END IF;
 column_name:='f_'||replace(field_uuid::text,'-','');
 typ:=after_field->>'Type';old_typ:=before_field->>'Type';required_value:=COALESCE((after_field->>'Required')::boolean,false);
 IF operation<>'drop_column' THEN
  IF typ='numeric' THEN
   precision_value:=(after_field->>'Precision')::integer;scale_value:=(after_field->>'Scale')::integer;
   IF precision_value NOT BETWEEN 1 AND 38 OR scale_value NOT BETWEEN 0 AND 18 OR scale_value>precision_value THEN RAISE EXCEPTION 'invalid decimal type' USING ERRCODE='23514'; END IF;
   typ:=format('numeric(%s,%s)',precision_value,scale_value);
  ELSIF typ NOT IN ('text','date','timestamptz','boolean','uuid','uuid[]') THEN RAISE EXCEPTION 'invalid physical type' USING ERRCODE='23514';END IF;
  IF after_field->'Default' IS NOT NULL AND after_field->'Default'<>'null'::jsonb THEN
   IF after_field->>'Type'='boolean' THEN default_sql:=CASE WHEN (after_field->'Default'->>'Boolean')::boolean THEN 'true' ELSE 'false' END;
   ELSIF after_field->>'Type'='uuid[]' THEN SELECT format('ARRAY[%s]::uuid[]',string_agg(format('%L::uuid',x),',')) INTO default_sql FROM jsonb_array_elements_text(after_field->'Default'->'UUIDs') x;
   ELSE default_sql:=format('%L::%s',after_field->'Default'->>'Text',typ);END IF;
  END IF;
 END IF;
 CASE operation
 WHEN 'add_column' THEN EXECUTE format('ALTER TABLE %s ADD COLUMN %I %s%s%s',qualified,column_name,typ,CASE WHEN default_sql IS NULL THEN '' ELSE ' DEFAULT '||default_sql END,CASE WHEN required_value THEN ' NOT NULL' ELSE '' END);
 WHEN 'drop_column' THEN EXECUTE format('ALTER TABLE %s DROP COLUMN %I RESTRICT',qualified,column_name);
 WHEN 'alter_type' THEN
  EXECUTE format('ALTER TABLE %s ALTER COLUMN %I DROP DEFAULT',qualified,column_name);
  expr:=format('%I::%s',column_name,typ);
  IF after_field->>'Type'='numeric' THEN
   places:=(after_field->>'RoundingPlaces')::integer;mode:=after_field->>'RoundingMode';
   IF places>scale_value THEN RAISE EXCEPTION 'storage rounding invalid' USING ERRCODE='23514';END IF;
   expr:=format('applications.round_decimal(%I::numeric,%s,%L)::%s',column_name,places,mode,typ);
  ELSIF after_field->>'Type'='timestamptz' THEN
   mode:=after_field->>'TimePrecision';IF mode NOT IN ('minute','second','millisecond') THEN RAISE EXCEPTION 'time precision invalid' USING ERRCODE='23514';END IF;
   expr:=format('date_trunc(%L,%I::timestamptz,''UTC'')',CASE WHEN mode='millisecond' THEN 'milliseconds' ELSE mode END,column_name);
  ELSIF old_typ='uuid' AND after_field->>'Type'='uuid[]' THEN expr:=format('CASE WHEN %I IS NULL THEN NULL ELSE ARRAY[%I] END',column_name,column_name);
  ELSIF old_typ='uuid[]' AND after_field->>'Type'='uuid' THEN expr:=format('(%I)[1]',column_name);
  ELSIF old_typ='timestamptz' AND after_field->>'Type'='text' THEN expr:=format('to_char(%I AT TIME ZONE ''UTC'',%L)',column_name,CASE WHEN before_field->>'TimePrecision'='millisecond' THEN 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"' ELSE 'YYYY-MM-DD"T"HH24:MI:SS"Z"' END);
  END IF;
  EXECUTE format('ALTER TABLE %s ALTER COLUMN %I TYPE %s USING %s',qualified,column_name,typ,expr);
 WHEN 'alter_default' THEN EXECUTE format('ALTER TABLE %s ALTER COLUMN %I %s',qualified,column_name,CASE WHEN default_sql IS NULL THEN 'DROP DEFAULT' ELSE 'SET DEFAULT '||default_sql END);
 WHEN 'alter_required' THEN EXECUTE format('ALTER TABLE %s ALTER COLUMN %I %s NOT NULL',qualified,column_name,CASE WHEN required_value THEN 'SET' ELSE 'DROP' END);
 ELSE RAISE EXCEPTION 'invalid DDL operation' USING ERRCODE='23514';
 END CASE;
END $$;
-- +goose StatementEnd
REVOKE ALL ON SCHEMA appdata FROM PUBLIC;
-- +goose StatementBegin
CREATE FUNCTION applications.apply_option_mapping(actor_uuid uuid,app_uuid uuid,table_uuid uuid,field_uuid uuid,old_kind text,next_config jsonb,mappings jsonb) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE qualified text;column_name text;mapping jsonb;target uuid;source uuid;expr text;
BEGIN
 IF NOT EXISTS(SELECT 1 FROM auth.users u JOIN applications.apps a ON a.id=app_uuid WHERE u.id=actor_uuid AND u.status='active' AND (u.is_bootstrap_admin OR a.owner_user_id=u.id)) THEN RAISE EXCEPTION 'definition denied' USING ERRCODE='42501';END IF;
 PERFORM 1 FROM applications.logical_tables WHERE id=table_uuid AND app_id=app_uuid FOR UPDATE;
 IF NOT FOUND OR NOT EXISTS(SELECT 1 FROM applications.fields WHERE id=field_uuid AND app_id=app_uuid AND table_id=table_uuid AND NOT removed AND definition->>'kind'=old_kind) THEN RAISE EXCEPTION 'field owner mismatch' USING ERRCODE='23503';END IF;
 IF old_kind NOT IN ('single_select','multi_select') OR jsonb_typeof(mappings)<>'array' OR jsonb_typeof(next_config->'options')<>'array' THEN RAISE EXCEPTION 'invalid option mapping' USING ERRCODE='23514';END IF;
 FOR mapping IN SELECT * FROM jsonb_array_elements(mappings) LOOP
  source:=(mapping->>'fromOptionId')::uuid;target:=(mapping->>'toOptionId')::uuid;
  IF source IS NULL OR (mapping->>'fieldId')::uuid<>field_uuid OR target IS NOT NULL AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(next_config->'options') o WHERE (o->>'id')::uuid=target) THEN RAISE EXCEPTION 'invalid option identity' USING ERRCODE='23514';END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(mappings) m GROUP BY m->>'fromOptionId' HAVING count(*)>1) THEN RAISE EXCEPTION 'duplicate mapping' USING ERRCODE='23514';END IF;
 qualified:=format('appdata.%I','t_'||replace(table_uuid::text,'-',''));column_name:='f_'||replace(field_uuid::text,'-','');
 EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE',qualified);
 IF old_kind='single_select' THEN
  EXECUTE format('UPDATE %s SET %I=(SELECT (m->>''toOptionId'')::uuid FROM jsonb_array_elements($1) m WHERE (m->>''fromOptionId'')::uuid=%I) WHERE EXISTS(SELECT 1 FROM jsonb_array_elements($1) m WHERE (m->>''fromOptionId'')::uuid=%I)',qualified,column_name,column_name,column_name) USING mappings;
 ELSE
  expr:=format('NULLIF(ARRAY(SELECT (o->>''id'')::uuid FROM jsonb_array_elements($2->''options'') WITH ORDINALITY AS options(o,ordering) WHERE (o->>''id'')::uuid=ANY(ARRAY(SELECT CASE WHEN EXISTS(SELECT 1 FROM jsonb_array_elements($1) m WHERE (m->>''fromOptionId'')::uuid=selected.option_id) THEN (SELECT (m->>''toOptionId'')::uuid FROM jsonb_array_elements($1) m WHERE (m->>''fromOptionId'')::uuid=selected.option_id) ELSE selected.option_id END FROM unnest(%I) AS selected(option_id))) ORDER BY ordering),ARRAY[]::uuid[])',column_name);
  EXECUTE format('UPDATE %s SET %I=%s WHERE %I IS NOT NULL AND %I IS DISTINCT FROM %s',qualified,column_name,expr,column_name,column_name,expr) USING mappings,next_config;
 END IF;
END $$;
-- +goose StatementEnd
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA applications FROM PUBLIC;
-- Hot audit keeps old expression intact and adds an independent safe structural envelope.
-- +goose StatementBegin
DO $$ DECLARE old_expression text;
BEGIN
 SELECT pg_get_expr(conbin,conrelid) INTO old_expression FROM pg_constraint WHERE conrelid='auth.authentication_events'::regclass AND conname='ck_auth_events_type';
 ALTER TABLE auth.authentication_events DROP CONSTRAINT ck_auth_events_type;
 EXECUTE format('ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_type CHECK((%s) OR event_type=''application_structure_changed'')',old_expression);
 SELECT pg_get_expr(conbin,conrelid) INTO old_expression FROM pg_constraint WHERE conrelid='auth.authentication_events'::regclass AND conname='ck_auth_events_summary';
 ALTER TABLE auth.authentication_events DROP CONSTRAINT ck_auth_events_summary;
 EXECUTE format('ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_summary CHECK((event_type<>''application_structure_changed'' AND (%s)) OR COALESCE((event_type=''application_structure_changed'' AND actor_user_id IS NOT NULL AND object_id IS NOT NULL AND object_type IN (''directory'',''table'',''form'') AND jsonb_typeof(change_summary)=''object'' AND change_summary ?& ARRAY[''appId'',''operationId'',''structureVersion'',''schemaVersion'',''viewVersion'',''changeCount''] AND change_summary-ARRAY[''appId'',''operationId'',''structureVersion'',''schemaVersion'',''viewVersion'',''changeCount'']=''{}''::jsonb),false))',old_expression);
END $$;
-- +goose StatementEnd
-- No destructive Down. Restore via approved backup/recovery; never drop durable records.
