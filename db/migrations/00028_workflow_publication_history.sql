-- +goose Up
-- V030-065: retain original immutable publication/deployment history after
-- configuration cleanup. No DELETE grant or product deletion entry point.
LOCK TABLE applications.apps,applications.workflow_definitions,applications.workflow_versions,
 applications.workflow_publications,applications.workflow_engine_receipts,
 applications.workflow_deployments IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications.workflow_publications p
  LEFT JOIN applications.workflow_definitions d ON d.app_id=p.app_id AND d.id=p.flow_id AND d.view_id=p.view_id
  LEFT JOIN applications.workflow_versions v ON v.app_id=p.app_id AND v.flow_id=p.flow_id AND v.version=p.version AND v.version_id=p.version_id
  WHERE d.id IS NULL OR v.version_id IS NULL)
 OR EXISTS(SELECT 1 FROM applications.workflow_engine_receipts r
  LEFT JOIN applications.workflow_versions v ON v.app_id=r.app_id AND v.flow_id=r.flow_id AND v.version=r.version AND v.version_id=r.version_id
  WHERE v.version_id IS NULL)
 OR EXISTS(SELECT 1 FROM applications.workflow_deployments r
  LEFT JOIN applications.workflow_versions v ON v.app_id=r.app_id AND v.flow_id=r.flow_id AND v.version=r.version
  WHERE v.version_id IS NULL) THEN
  RAISE EXCEPTION 'publication history has invalid original catalog binding' USING ERRCODE='23503';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE applications.workflow_publications ADD CONSTRAINT fk_publication_history_app
 FOREIGN KEY(app_id) REFERENCES applications.apps(id) ON DELETE RESTRICT;
ALTER TABLE applications.workflow_engine_receipts ADD CONSTRAINT fk_engine_receipt_history_app
 FOREIGN KEY(app_id) REFERENCES applications.apps(id) ON DELETE RESTRICT;
ALTER TABLE applications.workflow_deployments ADD CONSTRAINT fk_deployment_history_app
 FOREIGN KEY(app_id) REFERENCES applications.apps(id) ON DELETE RESTRICT;

-- +goose StatementBegin
CREATE FUNCTION applications.bind_publication_history_insert() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE original_view uuid; original_version uuid;
BEGIN
 -- Row locking needs UPDATE on immutable versions, which runtime must not have.
 -- VOLATILE trigger statements acquire fresh RC snapshots after the shared lock.
 IF pg_catalog.current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'publication history writes require read committed' USING ERRCODE='25001';
 END IF;
 PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(
  'weaveos:publication-history:' || NEW.app_id::text || ':' || NEW.flow_id::text,0));
 SELECT view_id INTO original_view FROM applications.workflow_definitions
 WHERE app_id=NEW.app_id AND id=NEW.flow_id;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'publication history definition missing' USING ERRCODE='23503';
 END IF;
 SELECT version_id INTO original_version FROM applications.workflow_versions
 WHERE app_id=NEW.app_id AND flow_id=NEW.flow_id AND version=NEW.version;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'publication history version missing' USING ERRCODE='23503';
 END IF;
 IF TG_TABLE_NAME='workflow_publications' THEN
  IF NEW.view_id IS DISTINCT FROM original_view OR NEW.version_id IS DISTINCT FROM original_version THEN
   RAISE EXCEPTION 'publication history scope mismatch' USING ERRCODE='23503';
  END IF;
 ELSIF TG_TABLE_NAME='workflow_engine_receipts' THEN
  IF NEW.version_id IS DISTINCT FROM original_version THEN
   RAISE EXCEPTION 'engine receipt version mismatch' USING ERRCODE='23503';
  END IF;
 ELSIF TG_TABLE_NAME<>'workflow_deployments' THEN
  RAISE EXCEPTION 'unsupported publication history relation' USING ERRCODE='23503';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.bind_publication_history_insert() FROM PUBLIC;
CREATE TRIGGER bind_publication_history_insert BEFORE INSERT ON applications.workflow_publications
 FOR EACH ROW EXECUTE FUNCTION applications.bind_publication_history_insert();
CREATE TRIGGER bind_engine_receipt_history_insert BEFORE INSERT ON applications.workflow_engine_receipts
 FOR EACH ROW EXECUTE FUNCTION applications.bind_publication_history_insert();
CREATE TRIGGER bind_deployment_history_insert BEFORE INSERT ON applications.workflow_deployments
 FOR EACH ROW EXECUTE FUNCTION applications.bind_publication_history_insert();

-- +goose StatementBegin
CREATE FUNCTION applications.guard_workflow_catalog_cleanup() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE target_flow uuid;
BEGIN
 IF TG_TABLE_NAME='workflow_definitions' THEN target_flow:=OLD.id;
 ELSIF TG_TABLE_NAME='workflow_versions' THEN target_flow:=OLD.flow_id;
 ELSE RAISE EXCEPTION 'unsupported workflow cleanup relation' USING ERRCODE='55000';
 END IF;
 IF pg_catalog.current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'workflow catalog cleanup requires read committed' USING ERRCODE='25001';
 END IF;
 PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(
  'weaveos:publication-history:' || OLD.app_id::text || ':' || target_flow::text,0));
 IF EXISTS(SELECT 1 FROM applications.workflow_instances i
   WHERE i.app_id=OLD.app_id AND i.flow_id=target_flow AND i.state IN ('starting','active'))
 OR EXISTS(SELECT 1 FROM applications.workflow_publications p
   WHERE p.app_id=OLD.app_id AND p.flow_id=target_flow AND p.status IN ('pending','unknown'))
 OR EXISTS(SELECT 1 FROM applications.workflow_commands c
   WHERE c.state='pending' AND c.command_json->>'AppID'=OLD.app_id::text
   AND (c.command_json->>'FlowID'=target_flow::text OR EXISTS(
    SELECT 1 FROM applications.workflow_instances i WHERE i.app_id=OLD.app_id AND i.flow_id=target_flow
    AND i.id::text=c.command_json->>'InstanceID'))) THEN
  RAISE EXCEPTION 'workflow has undrained accepted work; cleanup refused' USING ERRCODE='55000';
 END IF;
 RETURN OLD;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.guard_workflow_catalog_cleanup() FROM PUBLIC;
CREATE TRIGGER guard_workflow_definition_cleanup BEFORE DELETE ON applications.workflow_definitions
 FOR EACH ROW EXECUTE FUNCTION applications.guard_workflow_catalog_cleanup();
CREATE TRIGGER guard_workflow_version_cleanup BEFORE DELETE ON applications.workflow_versions
 FOR EACH ROW EXECUTE FUNCTION applications.guard_workflow_catalog_cleanup();

ALTER TABLE applications.workflow_publications
 DROP CONSTRAINT workflow_publications_app_id_flow_id_view_id_fkey,
 DROP CONSTRAINT workflow_publications_app_id_flow_id_version_version_id_fkey;
ALTER TABLE applications.workflow_engine_receipts
 DROP CONSTRAINT workflow_engine_receipts_app_id_flow_id_version_version_id_fkey;
ALTER TABLE applications.workflow_deployments
 DROP CONSTRAINT workflow_deployments_app_id_flow_id_version_fkey;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 LOCK TABLE applications.workflow_definitions,applications.workflow_versions,
 applications.workflow_publications,applications.workflow_engine_receipts,
 applications.workflow_deployments IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM applications.workflow_publications)
 OR EXISTS(SELECT 1 FROM applications.workflow_engine_receipts)
 OR EXISTS(SELECT 1 FROM applications.workflow_deployments) THEN
  RAISE EXCEPTION 'independent publication history exists; Down refused' USING ERRCODE='55000';
 END IF;
 ALTER TABLE applications.workflow_publications
  ADD CONSTRAINT workflow_publications_app_id_flow_id_view_id_fkey FOREIGN KEY(app_id,flow_id,view_id) REFERENCES applications.workflow_definitions(app_id,id,view_id) ON DELETE RESTRICT,
  ADD CONSTRAINT workflow_publications_app_id_flow_id_version_version_id_fkey FOREIGN KEY(app_id,flow_id,version,version_id) REFERENCES applications.workflow_versions(app_id,flow_id,version,version_id) ON DELETE RESTRICT,
  DROP CONSTRAINT fk_publication_history_app;
 ALTER TABLE applications.workflow_engine_receipts
  ADD CONSTRAINT workflow_engine_receipts_app_id_flow_id_version_version_id_fkey FOREIGN KEY(app_id,flow_id,version,version_id) REFERENCES applications.workflow_versions(app_id,flow_id,version,version_id) ON DELETE RESTRICT,
  DROP CONSTRAINT fk_engine_receipt_history_app;
 ALTER TABLE applications.workflow_deployments
  ADD CONSTRAINT workflow_deployments_app_id_flow_id_version_fkey FOREIGN KEY(app_id,flow_id,version) REFERENCES applications.workflow_versions(app_id,flow_id,version) ON DELETE RESTRICT,
  DROP CONSTRAINT fk_deployment_history_app;
 DROP TRIGGER bind_publication_history_insert ON applications.workflow_publications;
 DROP TRIGGER bind_engine_receipt_history_insert ON applications.workflow_engine_receipts;
 DROP TRIGGER bind_deployment_history_insert ON applications.workflow_deployments;
 DROP TRIGGER guard_workflow_definition_cleanup ON applications.workflow_definitions;
 DROP TRIGGER guard_workflow_version_cleanup ON applications.workflow_versions;
 DROP FUNCTION applications.bind_publication_history_insert();
 DROP FUNCTION applications.guard_workflow_catalog_cleanup();
END $$;
-- +goose StatementEnd
