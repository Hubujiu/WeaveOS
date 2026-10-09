-- +goose Up
-- V030-064: extend the one immutable event journal, never duplicate BPMN/graph.
-- Publication receipts and their catalog FKs are intentionally unchanged.
LOCK TABLE applications.workflow_definitions, applications.workflow_versions,
 applications.workflow_instances, applications.workflow_commands,
 applications.workflow_execution_events, applications.workflow_tasks IN ACCESS EXCLUSIVE MODE;
ALTER TABLE applications.workflow_execution_events
 ADD COLUMN table_id uuid,
 ADD COLUMN view_id uuid,
 ADD COLUMN record_id uuid,
 ADD COLUMN flow_id uuid,
 ADD COLUMN version_id uuid,
 ADD COLUMN definition_version bigint,
 ADD COLUMN task_id uuid,
 ADD COLUMN task_epoch bigint,
 ADD COLUMN node_id uuid,
 ADD COLUMN target_node_id uuid,
 ADD COLUMN flow_name varchar(100),
 ADD COLUMN flow_name_source text;

-- +goose StatementBegin
CREATE FUNCTION applications.capture_workflow_event_journal() RETURNS trigger
LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$
DECLARE
 c jsonb; r jsonb; ledger_state text; i record; t record;
 wanted_task uuid; wanted_node uuid; wanted_target uuid; wanted_epoch bigint;
 wanted_source text;
BEGIN
 SELECT command_json,receipt_json,state INTO STRICT c,r,ledger_state
 FROM applications.workflow_commands WHERE command_id=NEW.command_id;
 SELECT wi.*, wv.version_id AS original_version_id, wd.name AS original_name INTO STRICT i
 FROM applications.workflow_instances wi
 JOIN applications.workflow_definitions wd ON wd.app_id=wi.app_id AND wd.id=wi.flow_id
 JOIN applications.workflow_versions wv ON wv.app_id=wi.app_id AND wv.flow_id=wi.flow_id AND wv.version=wi.definition_version
 WHERE wi.app_id=NEW.app_id AND wi.id=NEW.instance_id;
 IF (c->>'ProtocolVersion')::integer IS DISTINCT FROM 2
 OR (c->>'CommandID')::uuid IS DISTINCT FROM NEW.command_id
 OR (c->>'AppID')::uuid IS DISTINCT FROM NEW.app_id
 OR (c->>'InstanceID')::uuid IS DISTINCT FROM NEW.instance_id
 OR (c->>'ActorID')::uuid IS DISTINCT FROM NEW.actor_id
 OR c->>'Action' IS DISTINCT FROM NEW.action
 OR (c->>'SchemaVersion')::bigint IS DISTINCT FROM NEW.schema_version
 OR (c->>'RecordVersion')::bigint IS DISTINCT FROM NEW.record_version
 OR (c->>'TableID')::uuid IS DISTINCT FROM i.table_id
 OR (c->>'ViewID')::uuid IS DISTINCT FROM i.view_id
 OR (c->>'RecordID')::uuid IS DISTINCT FROM i.record_id
 OR (c->>'FlowID')::uuid IS DISTINCT FROM i.flow_id
 OR (c->>'VersionID')::uuid IS DISTINCT FROM i.original_version_id
 OR (c->>'DefinitionVersion')::bigint IS DISTINCT FROM i.definition_version
 OR NEW.sequence IS DISTINCT FROM (c->>'ExpectedSequence')::bigint + (CASE WHEN NEW.outcome='success' THEN 1 ELSE 0 END) THEN
  RAISE EXCEPTION 'workflow journal identity mismatch' USING ERRCODE='23514';
 END IF;
 -- INSERT is inside the original projection callback, before terminal ledger
 -- update. Backfill, however, must only accept already confirmed old events.
 IF TG_OP='UPDATE' OR ledger_state<>'pending' THEN
  IF ledger_state IS DISTINCT FROM NEW.outcome OR r IS NULL
  OR (r->>'CommandID')::uuid IS DISTINCT FROM NEW.command_id
  OR r->>'Outcome' IS DISTINCT FROM NEW.outcome
  OR (r->>'Sequence')::bigint IS DISTINCT FROM NEW.sequence
  OR (r->>'ProofID')::uuid IS DISTINCT FROM NEW.proof_id THEN
   RAISE EXCEPTION 'workflow journal terminal receipt mismatch' USING ERRCODE='23514';
  END IF;
 END IF;
 wanted_task:=nullif(c->>'TaskID','')::uuid;
 wanted_epoch:=(c->>'TaskEpoch')::bigint;
 wanted_target:=nullif(c->>'TargetNodeID','')::uuid;
 IF wanted_task IS NOT NULL THEN
  SELECT * INTO t FROM applications.workflow_tasks WHERE id=wanted_task;
  IF FOUND THEN
   IF (t.app_id,t.instance_id,t.assignee_id,t.activation_epoch) IS DISTINCT FROM (NEW.app_id,NEW.instance_id,NEW.actor_id,wanted_epoch) THEN
    RAISE EXCEPTION 'workflow journal original task mismatch' USING ERRCODE='23514';
   END IF;
   wanted_node:=t.node_id;
  ELSIF NEW.outcome<>'no_effect' THEN
   RAISE EXCEPTION 'workflow journal original task missing' USING ERRCODE='23514';
  END IF;
 END IF;
 wanted_source:=CASE WHEN TG_OP='UPDATE' THEN 'legacy_last_known' ELSE 'captured' END;
 -- Explicit conflicting caller values are rejected, not silently repaired.
 IF (NEW.table_id IS NOT NULL AND NEW.table_id IS DISTINCT FROM i.table_id)
 OR (NEW.view_id IS NOT NULL AND NEW.view_id IS DISTINCT FROM i.view_id)
 OR (NEW.record_id IS NOT NULL AND NEW.record_id IS DISTINCT FROM i.record_id)
 OR (NEW.flow_id IS NOT NULL AND NEW.flow_id IS DISTINCT FROM i.flow_id)
 OR (NEW.version_id IS NOT NULL AND NEW.version_id IS DISTINCT FROM i.original_version_id)
 OR (NEW.definition_version IS NOT NULL AND NEW.definition_version IS DISTINCT FROM i.definition_version)
 OR (NEW.task_id IS NOT NULL AND NEW.task_id IS DISTINCT FROM wanted_task)
 OR (NEW.task_epoch IS NOT NULL AND NEW.task_epoch IS DISTINCT FROM wanted_epoch)
 OR (NEW.node_id IS NOT NULL AND NEW.node_id IS DISTINCT FROM wanted_node)
 OR (NEW.target_node_id IS NOT NULL AND NEW.target_node_id IS DISTINCT FROM wanted_target)
 OR (NEW.flow_name IS NOT NULL AND NEW.flow_name IS DISTINCT FROM i.original_name)
 OR (NEW.flow_name_source IS NOT NULL AND NEW.flow_name_source IS DISTINCT FROM wanted_source) THEN
  RAISE EXCEPTION 'workflow journal supplied metadata mismatch' USING ERRCODE='23514';
 END IF;
 NEW.table_id:=i.table_id; NEW.view_id:=i.view_id; NEW.record_id:=i.record_id;
 NEW.flow_id:=i.flow_id; NEW.version_id:=i.original_version_id;
 NEW.definition_version:=i.definition_version; NEW.task_id:=wanted_task;
 NEW.task_epoch:=wanted_epoch; NEW.node_id:=wanted_node; NEW.target_node_id:=wanted_target;
 NEW.flow_name:=i.original_name; NEW.flow_name_source:=wanted_source;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION applications.capture_workflow_event_journal() FROM PUBLIC;
-- Temporary migration-only trigger validates every old event and populates only
-- new columns. Original bytes, hashes, times and identities remain untouched.
CREATE TRIGGER backfill_workflow_event_journal BEFORE UPDATE ON applications.workflow_execution_events
 FOR EACH ROW EXECUTE FUNCTION applications.capture_workflow_event_journal();
UPDATE applications.workflow_execution_events SET table_id=NULL;
DROP TRIGGER backfill_workflow_event_journal ON applications.workflow_execution_events;
CREATE TRIGGER capture_workflow_event_journal BEFORE INSERT ON applications.workflow_execution_events
 FOR EACH ROW EXECUTE FUNCTION applications.capture_workflow_event_journal();
ALTER TABLE applications.workflow_execution_events
 ALTER COLUMN table_id SET NOT NULL, ALTER COLUMN view_id SET NOT NULL,
 ALTER COLUMN record_id SET NOT NULL, ALTER COLUMN flow_id SET NOT NULL,
 ALTER COLUMN version_id SET NOT NULL, ALTER COLUMN definition_version SET NOT NULL,
 ALTER COLUMN task_epoch SET NOT NULL, ALTER COLUMN flow_name SET NOT NULL,
 ALTER COLUMN flow_name_source SET NOT NULL,
 ADD CONSTRAINT ck_workflow_journal_scope CHECK (
  table_id<>'00000000-0000-0000-0000-000000000000' AND view_id<>'00000000-0000-0000-0000-000000000000'
  AND record_id<>'00000000-0000-0000-0000-000000000000' AND flow_id<>'00000000-0000-0000-0000-000000000000'
  AND version_id<>'00000000-0000-0000-0000-000000000000' AND definition_version BETWEEN 1 AND 9007199254740991),
 ADD CONSTRAINT ck_workflow_journal_task CHECK (
  task_epoch BETWEEN 0 AND 9007199254740991 AND (task_id IS NULL)=(task_epoch=0)
  AND (task_id IS NULL OR task_id<>'00000000-0000-0000-0000-000000000000')
  AND (node_id IS NULL OR (task_id IS NOT NULL AND node_id<>'00000000-0000-0000-0000-000000000000'))
  AND (task_id IS NULL OR outcome='no_effect' OR node_id IS NOT NULL)),
 ADD CONSTRAINT ck_workflow_journal_target CHECK ((action='return')=(target_node_id IS NOT NULL)
  AND (target_node_id IS NULL OR target_node_id<>'00000000-0000-0000-0000-000000000000')),
 ADD CONSTRAINT ck_workflow_journal_name CHECK (btrim(flow_name)<>'' AND flow_name_source IN ('captured','legacy_last_known')),
 ADD CONSTRAINT fk_workflow_journal_table FOREIGN KEY(app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT;
ALTER TABLE applications.workflow_execution_events DROP CONSTRAINT workflow_execution_events_app_id_instance_id_fkey;

CREATE INDEX ix_workflow_events_record_history ON applications.workflow_execution_events
 (app_id,table_id,record_id,created_at DESC,command_id DESC);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 LOCK TABLE applications.workflow_instances,applications.workflow_execution_events IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM applications.workflow_execution_events) THEN
  RAISE EXCEPTION 'independent workflow history exists; Down refused' USING ERRCODE='55000';
 END IF;
 DROP INDEX applications.ix_workflow_events_record_history;
 DROP TRIGGER capture_workflow_event_journal ON applications.workflow_execution_events;
 DROP FUNCTION applications.capture_workflow_event_journal();
 ALTER TABLE applications.workflow_execution_events
  ADD CONSTRAINT workflow_execution_events_app_id_instance_id_fkey FOREIGN KEY(app_id,instance_id) REFERENCES applications.workflow_instances(app_id,id) ON DELETE RESTRICT,
  DROP CONSTRAINT fk_workflow_journal_table, DROP CONSTRAINT ck_workflow_journal_scope,
  DROP CONSTRAINT ck_workflow_journal_task, DROP CONSTRAINT ck_workflow_journal_target,
  DROP CONSTRAINT ck_workflow_journal_name,
  DROP COLUMN table_id,DROP COLUMN view_id,DROP COLUMN record_id,DROP COLUMN flow_id,
  DROP COLUMN version_id,DROP COLUMN definition_version,DROP COLUMN task_id,
  DROP COLUMN task_epoch,DROP COLUMN node_id,DROP COLUMN target_node_id,
  DROP COLUMN flow_name,DROP COLUMN flow_name_source;
END $$;
-- +goose StatementEnd
