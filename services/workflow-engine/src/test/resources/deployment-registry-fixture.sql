CREATE TABLE wf_deployments (
 version_id uuid PRIMARY KEY,
 app_id uuid NOT NULL,
 flow_id uuid NOT NULL,
 version bigint NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991),
 bpmn_sha256 varchar(64) NOT NULL CHECK(bpmn_sha256 ~ '^[0-9a-f]{64}$'),
 status varchar(16) NOT NULL CHECK(status IN ('pending','confirmed')),
 engine_deployment_id varchar(200),
 process_definition_id varchar(255),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(app_id,flow_id,version),
 CHECK((status='pending' AND engine_deployment_id IS NULL AND process_definition_id IS NULL)
 OR(status='confirmed' AND engine_deployment_id IS NOT NULL AND process_definition_id IS NOT NULL))
);

-- V067 reviewed isolated identity schema; production backfill remains explicit migration 00002.
CREATE TABLE wf_flow_deletion_guards (
 app_id uuid NOT NULL,
 flow_id uuid NOT NULL,
 retired boolean NOT NULL DEFAULT false,
 operation_id uuid,
 deleted_versions bigint,
 deleted_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CONSTRAINT pk_wf_flow_deletion_guards PRIMARY KEY(app_id,flow_id),
 CONSTRAINT uq_wf_flow_deletion_operation UNIQUE(operation_id),
 CONSTRAINT ck_wf_flow_deletion_identity CHECK(app_id<>'00000000-0000-0000-0000-000000000000'::uuid AND flow_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 CONSTRAINT ck_wf_flow_deletion_operation CHECK(operation_id IS NULL OR operation_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 CONSTRAINT ck_wf_flow_deletion_result CHECK(
  (NOT retired AND operation_id IS NULL AND deleted_versions IS NULL AND deleted_at IS NULL)
  OR (retired AND operation_id IS NOT NULL AND deleted_versions IS NOT NULL AND deleted_versions BETWEEN 0 AND 9007199254740991 AND deleted_at IS NOT NULL))
);
-- +goose StatementBegin
CREATE FUNCTION wf_flow_deletion_guard_immutable() RETURNS trigger
 LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.retired THEN RAISE EXCEPTION 'flow identity must be established live' USING ERRCODE='23514'; END IF;
 ELSE
  IF OLD.retired OR NEW.app_id IS DISTINCT FROM OLD.app_id OR NEW.flow_id IS DISTINCT FROM OLD.flow_id OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
   RAISE EXCEPTION 'flow deletion identity is immutable' USING ERRCODE='55000';
  END IF;
 END IF;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION wf_flow_deletion_guard_immutable() FROM PUBLIC;
CREATE TRIGGER wf_flow_deletion_guard_immutable BEFORE INSERT OR UPDATE ON wf_flow_deletion_guards
 FOR EACH ROW EXECUTE FUNCTION wf_flow_deletion_guard_immutable();

