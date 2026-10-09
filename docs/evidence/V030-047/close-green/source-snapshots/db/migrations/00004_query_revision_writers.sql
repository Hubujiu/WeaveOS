-- +goose Up
-- Q36: revisions are conservative write signals, never query-change verdicts.
-- Application writers call this before authorization/account/dependency locks.
-- +goose StatementBegin
CREATE FUNCTION personnel.lock_query_revisions() RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM revision FROM personnel.query_revisions WHERE scope='people' FOR UPDATE;
 PERFORM revision FROM personnel.query_revisions WHERE scope='configuration' FOR UPDATE;
 PERFORM revision FROM personnel.query_revisions WHERE scope='activity' FOR UPDATE;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION personnel.lock_query_revisions() FROM PUBLIC;

-- Direct owner-controlled configuration writes also lock before tuple locks.
-- Application entrypoints additionally lock before their authorization reads.
-- +goose StatementBegin
CREATE FUNCTION personnel.query_revision_statement_lock() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN PERFORM personnel.lock_query_revisions(); RETURN NULL; END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION personnel.query_revision_statement_lock() FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION personnel.query_revision_changed() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE before_value jsonb; after_value jsonb; target_scope text:=TG_ARGV[0];
BEGIN
 IF TG_OP <> 'INSERT' THEN before_value:=to_jsonb(OLD); END IF;
 IF TG_OP <> 'DELETE' THEN after_value:=to_jsonb(NEW); END IF;
 IF TG_TABLE_SCHEMA='auth' AND TG_TABLE_NAME='users' THEN
  before_value:=jsonb_build_array(before_value->'id',before_value->'account',before_value->'status',before_value->'is_bootstrap_admin',before_value->'created_at');
  after_value:=jsonb_build_array(after_value->'id',after_value->'account',after_value->'status',after_value->'is_bootstrap_admin',after_value->'created_at');
 ELSIF TG_TABLE_NAME='member_configuration' THEN
  IF TG_OP='INSERT' AND NEW.version=0 THEN RETURN NEW; END IF;
  IF TG_OP='DELETE' AND OLD.version=0 THEN RETURN OLD; END IF;
  before_value:=before_value-'updated_at'; after_value:=after_value-'updated_at';
 ELSE
  before_value:=before_value-ARRAY['version','updated_at','created_at'];
  after_value:=after_value-ARRAY['version','updated_at','created_at'];
 END IF;
 IF before_value IS DISTINCT FROM after_value THEN
  PERFORM personnel.lock_query_revisions();
  UPDATE personnel.query_revisions SET revision=revision+1 WHERE scope=target_scope;
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION personnel.query_revision_changed() FROM PUBLIC;

CREATE TRIGGER query_users_lock BEFORE INSERT OR DELETE OR UPDATE OF id,account,status,is_bootstrap_admin,created_at ON auth.users
FOR EACH STATEMENT EXECUTE FUNCTION personnel.query_revision_statement_lock();
CREATE TRIGGER query_users_revision AFTER INSERT OR UPDATE OR DELETE ON auth.users
FOR EACH ROW EXECUTE FUNCTION personnel.query_revision_changed('people');
CREATE TRIGGER query_member_configuration_revision BEFORE INSERT OR UPDATE OR DELETE ON personnel.member_configuration
FOR EACH ROW EXECUTE FUNCTION personnel.query_revision_changed('people');

-- +goose StatementBegin
DO $$ DECLARE item text; target_scope text; BEGIN
 FOREACH item IN ARRAY ARRAY['departments','identities','permission_templates','permission_catalog','identity_templates','identity_permissions','template_permissions','department_members','member_identities'] LOOP
  target_scope:=CASE WHEN item IN ('department_members','member_identities') THEN 'people' ELSE 'configuration' END;
  EXECUTE format('CREATE TRIGGER query_revision_lock BEFORE INSERT OR UPDATE OR DELETE ON personnel.%I FOR EACH STATEMENT EXECUTE FUNCTION personnel.query_revision_statement_lock()',item);
  EXECUTE format('CREATE TRIGGER query_revision_bump AFTER INSERT OR UPDATE OR DELETE ON personnel.%I FOR EACH ROW EXECUTE FUNCTION personnel.query_revision_changed(%L)',item,target_scope);
 END LOOP;
END $$;
-- +goose StatementEnd

-- Private login/password/session audit changes are excluded. Hot archive/delete
-- of a visible personnel or invitation event participates in the same revision.
-- +goose StatementBegin
CREATE FUNCTION personnel.query_activity_revision() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE before_value jsonb; after_value jsonb;
BEGIN
 IF TG_OP<>'INSERT' AND OLD.event_type IN ('personnel_changed','invitation_created') THEN
  before_value:=jsonb_build_array(OLD.id,OLD.occurred_at,OLD.event_type,OLD.actor_user_id,OLD.reason_code,OLD.object_type,OLD.object_id,OLD.change_summary,OLD.outcome);
 END IF;
 IF TG_OP<>'DELETE' AND NEW.event_type IN ('personnel_changed','invitation_created') THEN
  after_value:=jsonb_build_array(NEW.id,NEW.occurred_at,NEW.event_type,NEW.actor_user_id,NEW.reason_code,NEW.object_type,NEW.object_id,NEW.change_summary,NEW.outcome);
 END IF;
 IF before_value IS DISTINCT FROM after_value THEN
  PERFORM personnel.lock_query_revisions();
  UPDATE personnel.query_revisions SET revision=revision+1 WHERE scope='activity';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION personnel.query_activity_revision() FROM PUBLIC;
CREATE TRIGGER query_activity_revision BEFORE INSERT OR UPDATE OR DELETE ON auth.authentication_events
FOR EACH ROW EXECUTE FUNCTION personnel.query_activity_revision();

-- +goose Down
DROP TRIGGER query_activity_revision ON auth.authentication_events;
DROP FUNCTION personnel.query_activity_revision();
DROP TRIGGER query_users_revision ON auth.users;
DROP TRIGGER query_users_lock ON auth.users;
DROP TRIGGER query_member_configuration_revision ON personnel.member_configuration;
-- +goose StatementBegin
DO $$ DECLARE item text; BEGIN
 FOREACH item IN ARRAY ARRAY['departments','identities','permission_templates','permission_catalog','identity_templates','identity_permissions','template_permissions','department_members','member_identities'] LOOP
  EXECUTE format('DROP TRIGGER query_revision_bump ON personnel.%I',item);
  EXECUTE format('DROP TRIGGER query_revision_lock ON personnel.%I',item);
 END LOOP;
END $$;
-- +goose StatementEnd
DROP FUNCTION personnel.query_revision_changed();
DROP FUNCTION personnel.query_revision_statement_lock();
DROP FUNCTION personnel.lock_query_revisions();
