-- +goose Up
LOCK TABLE applications.workflow_definitions, applications.workflow_instances IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications.workflow_instances GROUP BY app_id,flow_id,record_id,created_at HAVING count(*)>1) THEN
  RAISE EXCEPTION USING ERRCODE='55000',MESSAGE='ambiguous legacy workflow round order; explicit reconciliation required';
 END IF;
END $$;
-- +goose StatementEnd
CREATE TABLE applications.workflow_rounds (
 instance_id uuid PRIMARY KEY,
 app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT,
 table_id uuid NOT NULL,view_id uuid NOT NULL,record_id uuid NOT NULL,flow_id uuid NOT NULL,
 initiator_id uuid NOT NULL,
 definition_version bigint NOT NULL CHECK(definition_version BETWEEN 1 AND 9007199254740991),
 round_number bigint NOT NULL CHECK(round_number BETWEEN 1 AND 9007199254740991),
 previous_instance_id uuid REFERENCES applications.workflow_rounds(instance_id) ON DELETE RESTRICT,
 round_kind varchar(16) NOT NULL CHECK(round_kind IN ('legacy','initial','resubmit','review')),
 created_at timestamptz NOT NULL,
 UNIQUE(app_id,flow_id,record_id,round_number),
 CHECK((round_kind IN ('legacy','initial') AND previous_instance_id IS NULL) OR
       (round_kind IN ('resubmit','review') AND previous_instance_id IS NOT NULL AND previous_instance_id<>instance_id))
);
CREATE INDEX ix_workflow_round_record ON applications.workflow_rounds(app_id,table_id,record_id,flow_id,round_number DESC);
INSERT INTO applications.workflow_rounds(instance_id,app_id,table_id,view_id,record_id,flow_id,initiator_id,definition_version,round_number,round_kind,created_at)
 SELECT id,app_id,table_id,view_id,record_id,flow_id,initiator_id,definition_version,
 row_number() OVER(PARTITION BY app_id,flow_id,record_id ORDER BY created_at), 'legacy',created_at
 FROM applications.workflow_instances;
ALTER TABLE applications.workflow_instances
 ADD COLUMN previous_instance_id uuid,
 ADD COLUMN round_kind varchar(16) NOT NULL DEFAULT 'initial',
 ADD CONSTRAINT ck_workflow_instance_round_kind CHECK(
  (round_kind='initial' AND previous_instance_id IS NULL) OR
  (round_kind IN ('resubmit','review') AND previous_instance_id IS NOT NULL AND previous_instance_id<>id));
REVOKE ALL ON applications.workflow_rounds FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION applications.capture_workflow_round() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE previous applications.workflow_rounds%ROWTYPE; next_number bigint; parent_state text;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION USING ERRCODE='40001',MESSAGE='workflow round allocation requires read committed';
 END IF;
 PERFORM 1 FROM applications.workflow_definitions WHERE app_id=NEW.app_id AND id=NEW.flow_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION USING ERRCODE='23503',MESSAGE='workflow round definition missing'; END IF;
 SELECT * INTO previous FROM applications.workflow_rounds
  WHERE app_id=NEW.app_id AND flow_id=NEW.flow_id AND record_id=NEW.record_id
  ORDER BY round_number DESC LIMIT 1;
 next_number:=COALESCE(previous.round_number,0)+1;
 IF next_number>9007199254740991 THEN RAISE EXCEPTION USING ERRCODE='40001',MESSAGE='workflow round limit reached'; END IF;
 IF NEW.round_kind IN ('resubmit','review') THEN
  IF previous.instance_id IS NULL OR previous.instance_id<>NEW.previous_instance_id OR previous.table_id<>NEW.table_id THEN
   RAISE EXCEPTION USING ERRCODE='40001',MESSAGE='workflow parent is not the latest round';
  END IF;
  SELECT state INTO parent_state FROM applications.workflow_instances WHERE id=previous.instance_id FOR SHARE;
  IF parent_state IS NULL OR parent_state NOT IN ('completed','rejected','withdrawn') OR
     (NEW.round_kind='resubmit' AND (parent_state NOT IN ('rejected','withdrawn') OR previous.initiator_id<>NEW.initiator_id)) THEN
   RAISE EXCEPTION USING ERRCODE='40001',MESSAGE='workflow parent is not eligible';
  END IF;
 END IF;
 INSERT INTO applications.workflow_rounds(instance_id,app_id,table_id,view_id,record_id,flow_id,initiator_id,definition_version,round_number,previous_instance_id,round_kind,created_at)
 VALUES(NEW.id,NEW.app_id,NEW.table_id,NEW.view_id,NEW.record_id,NEW.flow_id,NEW.initiator_id,NEW.definition_version,next_number,NEW.previous_instance_id,NEW.round_kind,NEW.created_at);
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.capture_workflow_round() FROM PUBLIC;
CREATE TRIGGER capture_workflow_round AFTER INSERT ON applications.workflow_instances
 FOR EACH ROW EXECUTE FUNCTION applications.capture_workflow_round();

-- +goose StatementBegin
CREATE FUNCTION applications.guard_workflow_round() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP<>'INSERT' THEN
  RAISE EXCEPTION USING ERRCODE='55000',MESSAGE='workflow round history is immutable';
 END IF;
 IF NEW.round_kind='legacy' OR NOT EXISTS(
  SELECT 1 FROM applications.workflow_instances i WHERE i.id=NEW.instance_id
   AND (i.app_id,i.table_id,i.view_id,i.record_id,i.flow_id,i.initiator_id,i.definition_version,i.created_at,i.round_kind)
    IS NOT DISTINCT FROM (NEW.app_id,NEW.table_id,NEW.view_id,NEW.record_id,NEW.flow_id,NEW.initiator_id,NEW.definition_version,NEW.created_at,NEW.round_kind)
   AND i.previous_instance_id IS NOT DISTINCT FROM NEW.previous_instance_id
 ) OR NEW.round_number<>(SELECT COALESCE(max(round_number),0)+1 FROM applications.workflow_rounds WHERE app_id=NEW.app_id AND flow_id=NEW.flow_id AND record_id=NEW.record_id) THEN
  RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='workflow round identity mismatch';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.guard_workflow_round() FROM PUBLIC;
CREATE TRIGGER guard_workflow_round BEFORE INSERT OR UPDATE OR DELETE ON applications.workflow_rounds
 FOR EACH ROW EXECUTE FUNCTION applications.guard_workflow_round();
CREATE TRIGGER guard_workflow_round_truncate BEFORE TRUNCATE ON applications.workflow_rounds
 FOR EACH STATEMENT EXECUTE FUNCTION applications.guard_workflow_round();
-- +goose StatementBegin
CREATE FUNCTION applications.guard_workflow_instance_round() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
BEGIN
 IF (NEW.id,NEW.app_id,NEW.table_id,NEW.view_id,NEW.record_id,NEW.flow_id,NEW.initiator_id,NEW.definition_version,NEW.previous_instance_id,NEW.round_kind)
 IS DISTINCT FROM (OLD.id,OLD.app_id,OLD.table_id,OLD.view_id,OLD.record_id,OLD.flow_id,OLD.initiator_id,OLD.definition_version,OLD.previous_instance_id,OLD.round_kind) THEN
  RAISE EXCEPTION USING ERRCODE='55000',MESSAGE='workflow instance round binding is immutable';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.guard_workflow_instance_round() FROM PUBLIC;
CREATE TRIGGER guard_workflow_instance_round BEFORE UPDATE ON applications.workflow_instances
 FOR EACH ROW EXECUTE FUNCTION applications.guard_workflow_instance_round();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 LOCK TABLE applications.workflow_definitions,applications.workflow_instances,applications.workflow_rounds IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM applications.workflow_rounds) OR EXISTS(SELECT 1 FROM applications.workflow_instances) THEN
  RAISE EXCEPTION USING ERRCODE='55000',MESSAGE='workflow round history blocks Down';
 END IF;
 DROP TRIGGER guard_workflow_instance_round ON applications.workflow_instances;
 DROP FUNCTION applications.guard_workflow_instance_round();
 DROP TRIGGER capture_workflow_round ON applications.workflow_instances;
 DROP FUNCTION applications.capture_workflow_round();
 DROP TABLE applications.workflow_rounds;
 DROP FUNCTION applications.guard_workflow_round();
 ALTER TABLE applications.workflow_instances DROP CONSTRAINT ck_workflow_instance_round_kind,DROP COLUMN previous_instance_id,DROP COLUMN round_kind;
END $$;
-- +goose StatementEnd
