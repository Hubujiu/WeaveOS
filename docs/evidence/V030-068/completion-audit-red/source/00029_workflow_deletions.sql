-- +goose Up
-- Durable deletion identity, separate from runnable catalog and old journals.
CREATE TABLE applications.workflow_deletions (
 flow_id uuid PRIMARY KEY CHECK(flow_id<>'00000000-0000-0000-0000-000000000000'),
 app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT CHECK(app_id<>'00000000-0000-0000-0000-000000000000'),
 table_id uuid NOT NULL CHECK(table_id<>'00000000-0000-0000-0000-000000000000'),
 view_id uuid NOT NULL CHECK(view_id<>'00000000-0000-0000-0000-000000000000'),
 actor_user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT CHECK(actor_user_id<>'00000000-0000-0000-0000-000000000000'),
 operation_id uuid NOT NULL CHECK(operation_id<>'00000000-0000-0000-0000-000000000000'),
 flow_name varchar(100) NOT NULL CHECK(btrim(flow_name)<>''),
 expected_revision bigint NOT NULL CHECK(expected_revision BETWEEN 1 AND 9007199254740991),
 fingerprint bytea NOT NULL CHECK(octet_length(fingerprint)=32),
 status varchar(16) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','unknown','deleted')),
 reason varchar(40) CHECK(reason IN ('waiting_work','engine_unavailable','engine_rejected','receipt_invalid','cleanup_retry')),
 engine_deleted_versions bigint CHECK(engine_deleted_versions BETWEEN 0 AND 9007199254740991),
 engine_deleted_at timestamptz,
 lease_token uuid CHECK(lease_token<>'00000000-0000-0000-0000-000000000000'),
 lease_until timestamptz,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 completed_at timestamptz,
 UNIQUE(actor_user_id,operation_id),
 UNIQUE(app_id,flow_id),
 FOREIGN KEY(app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT,
 CHECK((engine_deleted_versions IS NULL)=(engine_deleted_at IS NULL)),
 CHECK((lease_token IS NULL)=(lease_until IS NULL)),
 CHECK((status='deleted' AND engine_deleted_at IS NOT NULL AND completed_at IS NOT NULL AND lease_token IS NULL AND reason IS NULL)
   OR (status IN ('pending','unknown') AND completed_at IS NULL))
);
CREATE INDEX ix_workflow_deletions_due ON applications.workflow_deletions(next_attempt_at,created_at,flow_id)
 WHERE status IN ('pending','unknown');
CREATE INDEX ix_workflow_deletions_scope ON applications.workflow_deletions(app_id,table_id,flow_id);
REVOKE ALL ON applications.workflow_deletions FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION applications.guard_workflow_deletion_identity() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE original record;
BEGIN
 IF TG_OP='INSERT' THEN
  SELECT app_id,table_id,view_id,name,revision,state INTO original
   FROM applications.workflow_definitions WHERE id=NEW.flow_id FOR UPDATE;
  IF NOT FOUND OR ROW(NEW.app_id,NEW.table_id,NEW.view_id,NEW.flow_name,NEW.expected_revision+1)
   IS DISTINCT FROM ROW(original.app_id,original.table_id,original.view_id,original.name,original.revision) THEN
   -- Unclosed definitions are reported separately from mismatched identity.
   IF FOUND AND original.state<>'closing' AND NEW.app_id=original.app_id
    AND NEW.table_id=original.table_id AND NEW.view_id=original.view_id THEN
    RAISE EXCEPTION 'deletion acceptance must close original catalog first' USING ERRCODE='55000';
   END IF;
   RAISE EXCEPTION 'deletion identity must bind original catalog' USING ERRCODE='23503';
  END IF;
  IF original.state<>'closing' THEN
   RAISE EXCEPTION 'deletion acceptance must close original catalog first' USING ERRCODE='55000';
  END IF;
  IF NEW.status<>'pending' OR NEW.engine_deleted_at IS NOT NULL OR NEW.engine_deleted_versions IS NOT NULL
   OR NEW.completed_at IS NOT NULL OR NEW.lease_token IS NOT NULL OR NEW.attempts<>0 OR NEW.reason IS NOT NULL THEN
   RAISE EXCEPTION 'deletion must begin as an unconfirmed intent' USING ERRCODE='23514';
  END IF;
 ELSE
  IF OLD.status='deleted' OR
   ROW(NEW.flow_id,NEW.app_id,NEW.table_id,NEW.view_id,NEW.actor_user_id,NEW.operation_id,NEW.flow_name,NEW.expected_revision,NEW.fingerprint,NEW.created_at)
   IS DISTINCT FROM ROW(OLD.flow_id,OLD.app_id,OLD.table_id,OLD.view_id,OLD.actor_user_id,OLD.operation_id,OLD.flow_name,OLD.expected_revision,OLD.fingerprint,OLD.created_at)
   OR (OLD.engine_deleted_at IS NOT NULL AND ROW(NEW.engine_deleted_at,NEW.engine_deleted_versions) IS DISTINCT FROM ROW(OLD.engine_deleted_at,OLD.engine_deleted_versions)) THEN
   RAISE EXCEPTION 'accepted deletion identity and confirmed receipt are immutable' USING ERRCODE='55000';
  END IF;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.guard_workflow_deletion_identity() FROM PUBLIC;
CREATE TRIGGER guard_workflow_deletion_identity BEFORE INSERT OR UPDATE ON applications.workflow_deletions
 FOR EACH ROW EXECUTE FUNCTION applications.guard_workflow_deletion_identity();

-- +goose StatementBegin
CREATE FUNCTION applications.guard_deleted_workflow_configuration() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE target_flow uuid;
BEGIN
 IF TG_TABLE_NAME='workflow_definitions' THEN target_flow:=NEW.id;
 ELSE
  target_flow:=NEW.flow_id;
  -- The immutable version FK only takes KEY SHARE and does not serialize
  -- against a concurrent non-key state update. Lock the definition first.
  PERFORM 1 FROM applications.workflow_definitions WHERE id=target_flow FOR UPDATE;
 END IF;
 IF EXISTS(SELECT 1 FROM applications.workflow_deletions WHERE flow_id=target_flow) THEN
  IF TG_TABLE_NAME='workflow_definitions' AND TG_OP='UPDATE' THEN
   IF OLD.state='closing' AND NEW.state='disabled' AND NEW.revision=OLD.revision+1
    AND (to_jsonb(NEW)-ARRAY['state','revision','updated_at'])=(to_jsonb(OLD)-ARRAY['state','revision','updated_at']) THEN
    RETURN NEW;
   END IF;
  END IF;
  RAISE EXCEPTION 'workflow deletion identity prevents configuration changes' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.guard_deleted_workflow_configuration() FROM PUBLIC;
CREATE TRIGGER guard_deleted_workflow_configuration BEFORE INSERT OR UPDATE ON applications.workflow_definitions
 FOR EACH ROW EXECUTE FUNCTION applications.guard_deleted_workflow_configuration();
CREATE TRIGGER guard_deleted_workflow_version BEFORE INSERT ON applications.workflow_versions
 FOR EACH ROW EXECUTE FUNCTION applications.guard_deleted_workflow_configuration();

-- +goose StatementBegin
CREATE FUNCTION applications.workflow_deletion_has_work(target_app uuid,target_flow uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY INVOKER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM applications.workflow_instances i WHERE i.app_id=target_app AND i.flow_id=target_flow AND i.state IN ('starting','active'))
 OR EXISTS(SELECT 1 FROM applications.workflow_publications p WHERE p.app_id=target_app AND p.flow_id=target_flow AND p.status IN ('pending','unknown'))
 OR EXISTS(SELECT 1 FROM applications.workflow_commands c WHERE c.state='pending' AND c.command_json->>'AppID'=target_app::text
  AND (c.command_json->>'FlowID'=target_flow::text OR EXISTS(SELECT 1 FROM applications.workflow_instances i WHERE i.app_id=target_app AND i.flow_id=target_flow AND i.id::text=c.command_json->>'InstanceID')))
 OR EXISTS(SELECT 1 FROM applications.workflow_tasks t JOIN applications.workflow_instances i ON i.app_id=t.app_id AND i.id=t.instance_id WHERE i.app_id=target_app AND i.flow_id=target_flow AND t.closed_command_id IS NULL);
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.workflow_deletion_has_work(uuid,uuid) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION applications.complete_workflow_deletion(target_app uuid,target_flow uuid,original_operation uuid,original_lease uuid) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE d applications.workflow_deletions%ROWTYPE; affected integer;
BEGIN
 IF pg_catalog.current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'workflow deletion requires read committed' USING ERRCODE='25001';
 END IF;
 SELECT * INTO d FROM applications.workflow_deletions WHERE app_id=target_app AND flow_id=target_flow AND operation_id=original_operation;
 IF NOT FOUND THEN RAISE EXCEPTION 'original deletion identity missing' USING ERRCODE='23503'; END IF;
 -- Never take the durable queue row before the ordinary application gates.
 PERFORM 1 FROM applications.apps WHERE id=target_app FOR UPDATE;
 PERFORM 1 FROM applications.logical_tables WHERE app_id=target_app AND id=d.table_id FOR UPDATE;
 PERFORM 1 FROM applications.workflow_definitions WHERE app_id=target_app AND id=target_flow FOR UPDATE;
 SELECT * INTO d FROM applications.workflow_deletions WHERE app_id=target_app AND flow_id=target_flow AND operation_id=original_operation FOR UPDATE;
 IF NOT FOUND OR d.status='deleted' OR d.lease_token IS DISTINCT FROM original_lease OR d.lease_token IS NULL OR d.lease_until<=clock_timestamp() THEN RETURN false; END IF;
 IF d.engine_deleted_at IS NULL OR applications.workflow_deletion_has_work(target_app,target_flow) THEN
  RAISE EXCEPTION 'workflow deletion lacks confirmation or has undrained work' USING ERRCODE='55000';
 END IF;
 DELETE FROM applications.workflow_tasks t USING applications.workflow_instances i
  WHERE i.app_id=target_app AND i.flow_id=target_flow AND i.state IN ('completed','rejected','withdrawn','no_effect')
  AND t.app_id=i.app_id AND t.instance_id=i.id AND t.closed_command_id IS NOT NULL;
 DELETE FROM applications.workflow_instances WHERE app_id=target_app AND flow_id=target_flow AND state IN ('completed','rejected','withdrawn','no_effect');
 DELETE FROM applications.workflow_versions WHERE app_id=target_app AND flow_id=target_flow;
 DELETE FROM applications.workflow_definitions WHERE app_id=target_app AND id=target_flow;
 UPDATE applications.workflow_deletions SET status='deleted',reason=NULL,lease_token=NULL,lease_until=NULL,completed_at=clock_timestamp(),updated_at=clock_timestamp()
  WHERE app_id=target_app AND flow_id=target_flow AND operation_id=original_operation AND lease_token=original_lease AND lease_until>clock_timestamp();
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'deletion lease expired before completion' USING ERRCODE='40001'; END IF;
 INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary)
 VALUES('application_structure_changed','success',d.actor_user_id,'WORKFLOW_DELETION_COMPLETED',original_lease::text,'form',d.view_id,
  jsonb_build_object('appId',target_app,'flowId',target_flow,'operationId',original_operation,'status','deleted','action','workflow.delete.completed'));
 RETURN true;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.complete_workflow_deletion(uuid,uuid,uuid,uuid) FROM PUBLIC;

-- +goose Down
LOCK TABLE applications.workflow_deletions IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications.workflow_deletions) THEN
  RAISE EXCEPTION 'cannot erase durable workflow deletion identity' USING ERRCODE='55000';
 END IF;
END $$;
-- +goose StatementEnd
DROP FUNCTION applications.complete_workflow_deletion(uuid,uuid,uuid,uuid);
DROP FUNCTION applications.workflow_deletion_has_work(uuid,uuid);
DROP TRIGGER guard_deleted_workflow_version ON applications.workflow_versions;
DROP TRIGGER guard_deleted_workflow_configuration ON applications.workflow_definitions;
DROP FUNCTION applications.guard_deleted_workflow_configuration();
DROP TABLE applications.workflow_deletions;
DROP FUNCTION applications.guard_workflow_deletion_identity();
