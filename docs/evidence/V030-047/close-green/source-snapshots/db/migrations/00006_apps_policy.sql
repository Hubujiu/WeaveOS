-- +goose Up
-- B5a frozen schema. Compatible expansion; old migrations remain immutable.
CREATE SCHEMA applications;
CREATE TABLE applications.apps (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 name varchar(100) NOT NULL CHECK (btrim(name) <> ''),
 owner_user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 policy_revision bigint NOT NULL DEFAULT 1 CHECK (policy_revision BETWEEN 1 AND 9007199254740991),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE applications.permission_groups (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT,
 name varchar(100) NOT NULL CHECK (btrim(name) <> ''),
 enabled boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(app_id,id)
);
CREATE TABLE applications.group_members (
 app_id uuid NOT NULL,
 group_id uuid NOT NULL,
 user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 PRIMARY KEY(app_id,group_id,user_id),
 FOREIGN KEY(app_id,group_id) REFERENCES applications.permission_groups(app_id,id) ON DELETE RESTRICT
);
CREATE INDEX ix_app_group_members_user ON applications.group_members(user_id,app_id,group_id);
CREATE TABLE applications.menu_resources (
 app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT,
 resource_kind varchar(24) NOT NULL,
 resource_id uuid NOT NULL,
 PRIMARY KEY(app_id,resource_kind,resource_id),
 CHECK(resource_kind='application' AND resource_id=app_id)
);
CREATE TABLE applications.grants (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 app_id uuid NOT NULL,
 group_id uuid NOT NULL,
 resource_kind varchar(24) NOT NULL,
 resource_id uuid NOT NULL,
 action varchar(32) NOT NULL CHECK(action='menu.enter'),
 row_scope varchar(8) NOT NULL CHECK(row_scope='all'),
 UNIQUE(app_id,id),
 UNIQUE(app_id,group_id,resource_kind,resource_id,action,row_scope),
 FOREIGN KEY(app_id,group_id) REFERENCES applications.permission_groups(app_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(app_id,resource_kind,resource_id) REFERENCES applications.menu_resources(app_id,resource_kind,resource_id) ON DELETE RESTRICT
);
CREATE TABLE applications.grant_fields (
 app_id uuid NOT NULL,
 grant_id uuid NOT NULL,
 field_id uuid NOT NULL,
 PRIMARY KEY(app_id,grant_id,field_id),
 FOREIGN KEY(app_id,grant_id) REFERENCES applications.grants(app_id,id) ON DELETE RESTRICT,
 -- No reviewed field registry exists in B5a; every persisted grant has no fields.
 CONSTRAINT ck_b5_no_grant_fields CHECK(false)
);
CREATE TABLE applications.operations (
 actor_user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 operation_id uuid NOT NULL,
 app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 operation_kind varchar(40) NOT NULL CHECK(operation_kind IN ('application.create','group.create','group.update','members.replace','grants.replace')),
 fingerprint bytea NOT NULL CHECK(octet_length(fingerprint)=32),
 result_json jsonb,
 http_status integer,
 location text,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(actor_user_id,operation_id),
 CHECK((result_json IS NULL AND http_status IS NULL AND location IS NULL) OR
       (result_json IS NOT NULL AND jsonb_typeof(result_json)='object' AND http_status IS NOT NULL AND http_status IN (200,201) AND location IS NOT NULL))
);
-- +goose StatementBegin
CREATE FUNCTION applications.owner_immutable() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id THEN
  RAISE EXCEPTION 'application owner is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.owner_immutable() FROM PUBLIC;
CREATE TRIGGER immutable_app_owner BEFORE UPDATE OF owner_user_id ON applications.apps
FOR EACH ROW EXECUTE FUNCTION applications.owner_immutable();
-- +goose StatementBegin
CREATE FUNCTION applications.operation_complete() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM applications.operations o WHERE o.actor_user_id=NEW.actor_user_id AND o.operation_id=NEW.operation_id AND o.result_json IS NULL) THEN
  RAISE EXCEPTION 'operation result must commit atomically' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.operation_complete() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER complete_operation AFTER INSERT OR UPDATE ON applications.operations
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION applications.operation_complete();

ALTER TABLE personnel.permission_catalog DROP CONSTRAINT ck_permission_catalog_category;
ALTER TABLE personnel.permission_catalog ADD CONSTRAINT ck_permission_catalog_category CHECK (
 (category='system' AND code IN ('personnel.manage','applications.create') AND app_id IS NULL)
 OR (category='application' AND app_id IS NOT NULL AND btrim(app_id)<>'')
);
INSERT INTO personnel.permission_catalog(code,name,category,app_id) VALUES('applications.create','应用管理','system',NULL);

-- This function only registers exact app-owned facts. The caller has already
-- locked query revisions, current auth dependencies and the application policy.
-- +goose StatementBegin
CREATE FUNCTION applications.register_catalog_entry(app_uuid uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE app_name varchar; catalog_code varchar;
BEGIN
 SELECT a.name INTO STRICT app_name FROM applications.apps a WHERE a.id=app_uuid;
 catalog_code:='app.'||app_uuid::text||'.access';
 INSERT INTO personnel.permission_catalog(code,name,category,app_id)
 VALUES(catalog_code,app_name,'application',app_uuid::text)
 ON CONFLICT(code) DO NOTHING;
 IF NOT EXISTS(SELECT 1 FROM personnel.permission_catalog c WHERE c.code=catalog_code AND c.name=app_name AND c.category='application' AND c.app_id=app_uuid::text AND c.enabled) THEN
  RAISE EXCEPTION 'conflicting application catalog registration' USING ERRCODE='23514';
 END IF;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.register_catalog_entry(uuid) FROM PUBLIC;
REVOKE ALL ON SCHEMA applications FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA applications FROM PUBLIC;
ALTER TABLE auth.authentication_events DROP CONSTRAINT ck_auth_events_type;
ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_type CHECK(event_type IN ('register','login','logout','invitation_created','password_reset','session_invalid','account_status_changed','bootstrap_created','personnel_changed','application_changed'));
ALTER TABLE auth.authentication_events DROP CONSTRAINT ck_auth_events_summary;
ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_summary CHECK(COALESCE((
 (event_type='personnel_changed' AND object_type IS NOT NULL AND object_id IS NOT NULL AND change_summary IS NOT NULL
  AND object_type IN ('department','member','identity','template') AND jsonb_typeof(change_summary)='object')
 OR
 (event_type='application_changed' AND actor_user_id IS NOT NULL AND object_id IS NOT NULL
  AND change_summary IS NOT NULL AND jsonb_typeof(change_summary)='object'
  AND change_summary ?& ARRAY['appId','operationId','beforePolicyRevision','afterPolicyRevision','changeCounts']
  AND change_summary-ARRAY['appId','operationId','beforePolicyRevision','afterPolicyRevision','changeCounts']='{}'::jsonb
  AND jsonb_typeof(change_summary->'appId')='string'
  AND (change_summary->>'appId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  AND jsonb_typeof(change_summary->'operationId')='string'
  AND (change_summary->>'operationId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  AND jsonb_typeof(change_summary->'beforePolicyRevision')='number'
  AND (change_summary->>'beforePolicyRevision') ~ '^[0-9]+$'
  AND jsonb_typeof(change_summary->'afterPolicyRevision')='number'
  AND (change_summary->>'afterPolicyRevision') ~ '^[0-9]+$'
  AND (change_summary->>'afterPolicyRevision')::numeric=(change_summary->>'beforePolicyRevision')::numeric+1
  AND jsonb_typeof(change_summary->'changeCounts')='object'
  AND (change_summary->'changeCounts')-ARRAY['applications','groups','members','grants']='{}'::jsonb
  AND (NOT (change_summary->'changeCounts') ? 'applications' OR (jsonb_typeof(change_summary->'changeCounts'->'applications')='number' AND (change_summary->'changeCounts'->>'applications') ~ '^[0-9]+$'))
  AND (NOT (change_summary->'changeCounts') ? 'groups' OR (jsonb_typeof(change_summary->'changeCounts'->'groups')='number' AND (change_summary->'changeCounts'->>'groups') ~ '^[0-9]+$'))
  AND (NOT (change_summary->'changeCounts') ? 'members' OR (jsonb_typeof(change_summary->'changeCounts'->'members')='number' AND (change_summary->'changeCounts'->>'members') ~ '^[0-9]+$'))
  AND (NOT (change_summary->'changeCounts') ? 'grants' OR (jsonb_typeof(change_summary->'changeCounts'->'grants')='number' AND (change_summary->'changeCounts'->>'grants') ~ '^[0-9]+$'))
  AND ((reason_code='APPLICATION_CREATED' AND object_type='application' AND object_id::text=change_summary->>'appId'
        AND change_summary->>'beforePolicyRevision'='0' AND change_summary->>'afterPolicyRevision'='1')
    OR (reason_code IN ('GROUP_CREATED','GROUP_UPDATED','GROUP_MEMBERS_REPLACED','GROUP_GRANTS_REPLACED')
        AND object_type='permission_group' AND (change_summary->>'beforePolicyRevision')::numeric>=1)))
 OR
 (event_type NOT IN ('personnel_changed','application_changed') AND object_type IS NULL AND object_id IS NULL AND change_summary IS NULL)
),false));
-- No destructive Down: new apps, policies, operations and audit are durable.
