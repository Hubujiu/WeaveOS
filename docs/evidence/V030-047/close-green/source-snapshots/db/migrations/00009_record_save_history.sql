-- +goose Up
-- Accepted V015 ADR §11.4: canonical save deltas, separate from auth audit.
-- Published hot1–8 are unchanged. No automatic purge/retention or cold values.
CREATE TABLE applications.record_change_events(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), app_id uuid NOT NULL, table_id uuid NOT NULL,
 record_id uuid NOT NULL, source_view_id uuid NOT NULL, actor_user_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 record_version_before bigint NOT NULL CHECK(record_version_before BETWEEN 0 AND 9007199254740991),
 record_version_after bigint NOT NULL CHECK(record_version_after BETWEEN 1 AND 9007199254740991),
 origin text NOT NULL CHECK(origin IN ('ordinary','task_save')), opaque_task_ref text,
 occurred_at timestamptz NOT NULL,
 CHECK(record_version_after>record_version_before),
 UNIQUE(id,app_id,table_id), UNIQUE(actor_user_id,operation_id,record_id),
 FOREIGN KEY(app_id,table_id,source_view_id) REFERENCES applications.form_views(app_id,table_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(actor_user_id,operation_id) REFERENCES applications.operations(actor_user_id,operation_id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX ix_record_history_cursor ON applications.record_change_events(app_id,table_id,record_id,occurred_at DESC,id DESC);
CREATE TABLE applications.record_change_values(
 event_id uuid NOT NULL, app_id uuid NOT NULL, table_id uuid NOT NULL, field_id uuid NOT NULL,
 field_kind text NOT NULL CHECK(field_kind IN ('text','multiline','number','money','date','datetime','single_select','multi_select','boolean','member','department')),
 old_value jsonb NOT NULL CHECK(jsonb_typeof(old_value) IN ('string','boolean','array','null')),
 new_value jsonb NOT NULL CHECK(jsonb_typeof(new_value) IN ('string','boolean','array','null')),
 CHECK(old_value<>new_value), PRIMARY KEY(event_id,field_id),
 FOREIGN KEY(event_id,app_id,table_id) REFERENCES applications.record_change_events(id,app_id,table_id) ON DELETE RESTRICT,
 FOREIGN KEY(app_id,table_id,field_id) REFERENCES applications.fields(app_id,table_id,id) ON DELETE RESTRICT
);
CREATE TABLE applications.field_option_tombstones(
 app_id uuid NOT NULL, table_id uuid NOT NULL, field_id uuid NOT NULL, option_id uuid NOT NULL,
 label text NOT NULL, removed boolean NOT NULL,
 PRIMARY KEY(app_id,table_id,field_id,option_id),
 FOREIGN KEY(app_id,table_id,field_id) REFERENCES applications.fields(app_id,table_id,id) ON DELETE RESTRICT
);
INSERT INTO applications.field_option_tombstones(app_id,table_id,field_id,option_id,label,removed)
 SELECT f.app_id,f.table_id,f.id,(o->>'id')::uuid,o->>'label',f.removed
 FROM applications.fields f CROSS JOIN LATERAL jsonb_array_elements(COALESCE(f.definition->'config'->'options','[]'::jsonb)) o
 WHERE f.definition->>'kind' IN ('single_select','multi_select');
-- +goose StatementBegin
CREATE FUNCTION applications.retain_option_labels() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 UPDATE applications.field_option_tombstones SET removed=true WHERE app_id=NEW.app_id AND table_id=NEW.table_id AND field_id=NEW.id;
 IF NOT NEW.removed AND NEW.definition->>'kind' IN ('single_select','multi_select') THEN
  INSERT INTO applications.field_option_tombstones(app_id,table_id,field_id,option_id,label,removed)
   SELECT NEW.app_id,NEW.table_id,NEW.id,(o->>'id')::uuid,o->>'label',false
   FROM jsonb_array_elements(NEW.definition->'config'->'options') o
   ON CONFLICT(app_id,table_id,field_id,option_id) DO UPDATE SET label=EXCLUDED.label,removed=false;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER retain_option_labels AFTER INSERT OR UPDATE OF definition,removed ON applications.fields FOR EACH ROW EXECUTE FUNCTION applications.retain_option_labels();
-- +goose StatementBegin
CREATE FUNCTION applications.grant_dependency_changed() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE target uuid;
BEGIN
 FOR target IN SELECT id FROM applications.logical_tables
  WHERE id=CASE WHEN TG_OP='DELETE' THEN OLD.table_id ELSE NEW.table_id END
    OR TG_OP='UPDATE' AND id=OLD.table_id ORDER BY id FOR UPDATE LOOP
  UPDATE applications.logical_tables SET dependency_revision=dependency_revision+1 WHERE id=target;
 END LOOP;
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER grant_dependency_changed AFTER INSERT OR UPDATE OR DELETE ON applications.grant_fields FOR EACH ROW EXECUTE FUNCTION applications.grant_dependency_changed();
REVOKE ALL ON FUNCTION applications.retain_option_labels(),applications.grant_dependency_changed() FROM PUBLIC;
REVOKE ALL ON applications.record_change_events,applications.record_change_values,applications.field_option_tombstones FROM PUBLIC;
-- Roles are applied separately from the reviewed source; no destructive Down.
