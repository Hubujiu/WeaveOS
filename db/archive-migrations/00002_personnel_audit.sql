-- +goose Up
-- Q25: apply cold compatibility before hot, retaining the original PK-only storage.
ALTER TABLE archive.authentication_events
  ADD COLUMN object_type varchar(24),
  ADD COLUMN object_id uuid,
  ADD COLUMN change_summary jsonb;
ALTER TABLE archive.authentication_events DROP CONSTRAINT authentication_events_event_type_check;
ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_type CHECK (event_type IN ('register','login','logout','invitation_created','password_reset','session_invalid','account_status_changed','bootstrap_created','personnel_changed'));
ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_summary CHECK (
  (event_type = 'personnel_changed' AND object_type IS NOT NULL AND object_id IS NOT NULL AND change_summary IS NOT NULL
    AND object_type IN ('department','member','identity','template') AND jsonb_typeof(change_summary) = 'object')
  OR (event_type <> 'personnel_changed' AND object_type IS NULL AND object_id IS NULL AND change_summary IS NULL)
);
-- No destructive Down: rollback the application, preserving new audit records.

