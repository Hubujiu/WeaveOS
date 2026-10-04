-- +goose Up
-- V030-020 cold-first audit expansion. Keep the complete V030-013 legacy
-- personnel/application expression, plus the old structural shape, and add
-- only the three exact workflow management actions.
ALTER TABLE archive.authentication_events
    DROP CONSTRAINT ck_archive_events_summary;

ALTER TABLE archive.authentication_events
    ADD CONSTRAINT ck_archive_events_summary CHECK (
        (
            event_type <> 'application_structure_changed'
            AND COALESCE((
                (event_type = 'personnel_changed'
                    AND object_type IS NOT NULL AND object_id IS NOT NULL AND change_summary IS NOT NULL
                    AND object_type IN ('department','member','identity','template')
                    AND jsonb_typeof(change_summary) = 'object')
                OR
                (event_type = 'application_changed'
                    AND actor_user_id IS NOT NULL AND object_id IS NOT NULL AND change_summary IS NOT NULL
                    AND jsonb_typeof(change_summary) = 'object'
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
                    AND (NOT (change_summary->'changeCounts' ? 'applications') OR (jsonb_typeof(change_summary->'changeCounts'->'applications')='number' AND (change_summary->'changeCounts'->>'applications') ~ '^[0-9]+$'))
                    AND (NOT (change_summary->'changeCounts' ? 'groups') OR (jsonb_typeof(change_summary->'changeCounts'->'groups')='number' AND (change_summary->'changeCounts'->>'groups') ~ '^[0-9]+$'))
                    AND (NOT (change_summary->'changeCounts' ? 'members') OR (jsonb_typeof(change_summary->'changeCounts'->'members')='number' AND (change_summary->'changeCounts'->>'members') ~ '^[0-9]+$'))
                    AND (NOT (change_summary->'changeCounts' ? 'grants') OR (jsonb_typeof(change_summary->'changeCounts'->'grants')='number' AND (change_summary->'changeCounts'->>'grants') ~ '^[0-9]+$'))
                    AND ((reason_code='APPLICATION_CREATED' AND object_type='application' AND object_id::text=change_summary->>'appId'
                          AND change_summary->>'beforePolicyRevision'='0' AND change_summary->>'afterPolicyRevision'='1')
                      OR (reason_code IN ('GROUP_CREATED','GROUP_UPDATED','GROUP_MEMBERS_REPLACED','GROUP_GRANTS_REPLACED')
                          AND object_type='permission_group' AND (change_summary->>'beforePolicyRevision')::numeric>=1)))
                OR
                (event_type NOT IN ('personnel_changed','application_changed')
                    AND object_type IS NULL AND object_id IS NULL AND change_summary IS NULL)
            ), false)
        )
        OR COALESCE((
            event_type='application_structure_changed'
            AND actor_user_id IS NOT NULL AND object_id IS NOT NULL
            AND object_type IN ('directory','table','form')
            AND jsonb_typeof(change_summary)='object'
            AND change_summary ?& ARRAY['appId','operationId','structureVersion','schemaVersion','viewVersion','changeCount']
            AND change_summary-ARRAY['appId','operationId','structureVersion','schemaVersion','viewVersion','changeCount']='{}'::jsonb
            AND (reason_code IS NULL OR reason_code NOT IN ('WORKFLOW_DEFINITION_SAVE','WORKFLOW_ENABLE','WORKFLOW_CLOSE'))
        ), false)
        OR COALESCE((
            event_type='application_structure_changed'
            AND outcome='success' AND actor_user_id IS NOT NULL AND object_id IS NOT NULL AND object_type='form'
            AND jsonb_typeof(change_summary)='object'
            AND change_summary ?& ARRAY['appId','flowId','operationId','revision','state','action']
            AND change_summary-ARRAY['appId','flowId','operationId','revision','state','action']='{}'::jsonb
            AND jsonb_typeof(change_summary->'appId')='string'
            AND (change_summary->>'appId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            AND change_summary->>'appId'<>'00000000-0000-0000-0000-000000000000'
            AND jsonb_typeof(change_summary->'flowId')='string'
            AND (change_summary->>'flowId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            AND change_summary->>'flowId'<>'00000000-0000-0000-0000-000000000000'
            AND jsonb_typeof(change_summary->'operationId')='string'
            AND (change_summary->>'operationId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            AND change_summary->>'operationId'<>'00000000-0000-0000-0000-000000000000'
            AND jsonb_typeof(change_summary->'revision')='number'
            AND (change_summary->>'revision') ~ '^[0-9]+$'
            AND (change_summary->>'revision')::numeric BETWEEN 1 AND 9007199254740991
            AND jsonb_typeof(change_summary->'state')='string'
            AND change_summary->>'state' IN ('disabled','enabled','closing')
            AND jsonb_typeof(change_summary->'action')='string'
            AND (
                (reason_code='WORKFLOW_DEFINITION_SAVE' AND change_summary->>'action'='workflow.definition.save' AND change_summary->>'state' IN ('disabled','enabled'))
                OR (reason_code='WORKFLOW_ENABLE' AND change_summary->>'action'='workflow.enable' AND change_summary->>'state'='enabled')
                OR (reason_code='WORKFLOW_CLOSE' AND change_summary->>'action'='workflow.close' AND change_summary->>'state' IN ('disabled','closing'))
            )
        ), false)
    );

-- +goose Down
LOCK TABLE archive.authentication_events IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM archive.authentication_events WHERE reason_code IN ('WORKFLOW_DEFINITION_SAVE','WORKFLOW_ENABLE','WORKFLOW_CLOSE')) THEN
        RAISE EXCEPTION 'cannot remove workflow archive audit constraint while workflow history exists'
            USING ERRCODE = '55000';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE archive.authentication_events
    DROP CONSTRAINT ck_archive_events_summary;
ALTER TABLE archive.authentication_events
    ADD CONSTRAINT ck_archive_events_summary CHECK (
        (event_type <> 'application_structure_changed' AND COALESCE((
            (event_type='personnel_changed' AND object_type IS NOT NULL AND object_id IS NOT NULL AND change_summary IS NOT NULL
                AND object_type IN ('department','member','identity','template') AND jsonb_typeof(change_summary)='object')
            OR (event_type='application_changed' AND actor_user_id IS NOT NULL AND object_id IS NOT NULL AND change_summary IS NOT NULL
                AND jsonb_typeof(change_summary)='object'
                AND change_summary ?& ARRAY['appId','operationId','beforePolicyRevision','afterPolicyRevision','changeCounts']
                AND change_summary-ARRAY['appId','operationId','beforePolicyRevision','afterPolicyRevision','changeCounts']='{}'::jsonb
                AND jsonb_typeof(change_summary->'appId')='string'
                AND (change_summary->>'appId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
                AND jsonb_typeof(change_summary->'operationId')='string'
                AND (change_summary->>'operationId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
                AND jsonb_typeof(change_summary->'beforePolicyRevision')='number' AND (change_summary->>'beforePolicyRevision') ~ '^[0-9]+$'
                AND jsonb_typeof(change_summary->'afterPolicyRevision')='number' AND (change_summary->>'afterPolicyRevision') ~ '^[0-9]+$'
                AND (change_summary->>'afterPolicyRevision')::numeric=(change_summary->>'beforePolicyRevision')::numeric+1
                AND jsonb_typeof(change_summary->'changeCounts')='object'
                AND (change_summary->'changeCounts')-ARRAY['applications','groups','members','grants']='{}'::jsonb
                AND (NOT (change_summary->'changeCounts' ? 'applications') OR (jsonb_typeof(change_summary->'changeCounts'->'applications')='number' AND (change_summary->'changeCounts'->>'applications') ~ '^[0-9]+$'))
                AND (NOT (change_summary->'changeCounts' ? 'groups') OR (jsonb_typeof(change_summary->'changeCounts'->'groups')='number' AND (change_summary->'changeCounts'->>'groups') ~ '^[0-9]+$'))
                AND (NOT (change_summary->'changeCounts' ? 'members') OR (jsonb_typeof(change_summary->'changeCounts'->'members')='number' AND (change_summary->'changeCounts'->>'members') ~ '^[0-9]+$'))
                AND (NOT (change_summary->'changeCounts' ? 'grants') OR (jsonb_typeof(change_summary->'changeCounts'->'grants')='number' AND (change_summary->'changeCounts'->>'grants') ~ '^[0-9]+$'))
                AND ((reason_code='APPLICATION_CREATED' AND object_type='application' AND object_id::text=change_summary->>'appId'
                    AND change_summary->>'beforePolicyRevision'='0' AND change_summary->>'afterPolicyRevision'='1')
                  OR (reason_code IN ('GROUP_CREATED','GROUP_UPDATED','GROUP_MEMBERS_REPLACED','GROUP_GRANTS_REPLACED')
                    AND object_type='permission_group' AND (change_summary->>'beforePolicyRevision')::numeric>=1)))
            OR (event_type NOT IN ('personnel_changed','application_changed') AND object_type IS NULL AND object_id IS NULL AND change_summary IS NULL)
        ),false)) OR COALESCE((
            event_type='application_structure_changed' AND actor_user_id IS NOT NULL AND object_id IS NOT NULL
            AND object_type IN ('directory','table','form') AND jsonb_typeof(change_summary)='object'
            AND change_summary ?& ARRAY['appId','operationId','structureVersion','schemaVersion','viewVersion','changeCount']
            AND change_summary-ARRAY['appId','operationId','structureVersion','schemaVersion','viewVersion','changeCount']='{}'::jsonb
        ),false)
    );
