-- +goose Up
-- B5a: compatible 15-column audit expansion. Install cold before hot.
ALTER TABLE archive.authentication_events DROP CONSTRAINT ck_archive_events_type;
ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_type CHECK (event_type IN ('register','login','logout','invitation_created','password_reset','session_invalid','account_status_changed','bootstrap_created','personnel_changed','application_changed'));
ALTER TABLE archive.authentication_events DROP CONSTRAINT ck_archive_events_summary;
ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_summary CHECK (COALESCE((
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
-- No destructive Down: retain new application events on artifact rollback.

