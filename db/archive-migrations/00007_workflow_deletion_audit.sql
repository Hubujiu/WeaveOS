-- +goose Up
-- +goose StatementBegin
DO $$ DECLARE old_expression text; BEGIN
 SELECT pg_get_expr(conbin,conrelid) INTO STRICT old_expression FROM pg_constraint
 WHERE conrelid='archive.authentication_events'::regclass AND conname='ck_archive_events_summary';
 ALTER TABLE archive.authentication_events DROP CONSTRAINT ck_archive_events_summary;
 EXECUTE format($check$ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_summary CHECK(
 (reason_code IS DISTINCT FROM 'WORKFLOW_DELETION_COMPLETED' AND (%s)) OR COALESCE((event_type='application_structure_changed' AND outcome='success' AND actor_user_id IS NOT NULL AND object_type='form' AND object_id IS NOT NULL AND reason_code='WORKFLOW_DELETION_COMPLETED'
 AND jsonb_typeof(change_summary)='object'
 AND change_summary ?& ARRAY['appId','flowId','operationId','status','action']
 AND change_summary-ARRAY['appId','flowId','operationId','status','action']='{}'::jsonb
 AND change_summary->>'status'='deleted' AND change_summary->>'action'='workflow.delete.completed'
 AND jsonb_typeof(change_summary->'appId')='string' AND (change_summary->>'appId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' AND change_summary->>'appId'<>'00000000-0000-0000-0000-000000000000'
 AND jsonb_typeof(change_summary->'flowId')='string' AND (change_summary->>'flowId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' AND change_summary->>'flowId'<>'00000000-0000-0000-0000-000000000000'
 AND jsonb_typeof(change_summary->'operationId')='string' AND (change_summary->>'operationId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' AND change_summary->>'operationId'<>'00000000-0000-0000-0000-000000000000'),false))$check$,old_expression);
END $$;
-- +goose StatementEnd

-- +goose Down
LOCK TABLE archive.authentication_events IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM archive.authentication_events WHERE reason_code='WORKFLOW_DELETION_COMPLETED') THEN
  RAISE EXCEPTION 'cannot erase archived workflow deletion history' USING ERRCODE='55000';
 END IF;
END $$;
-- +goose StatementEnd
-- Preserve every existing audit alternative; new reason codes require exact publication shape.
ALTER TABLE archive.authentication_events DROP CONSTRAINT ck_archive_events_summary;
ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_summary CHECK (
    ((reason_code IS NULL OR reason_code NOT IN ('WORKFLOW_PUBLICATION_REQUESTED','WORKFLOW_PUBLICATION_CONFIRMED','WORKFLOW_PUBLICATION_SUPERSEDED','WORKFLOW_PUBLICATION_BLOCKED')) AND (

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
    )) OR COALESCE((
        event_type='application_structure_changed' AND actor_user_id IS NOT NULL
        AND object_type='form' AND object_id IS NOT NULL
        AND jsonb_typeof(change_summary)='object'
        AND change_summary ?& ARRAY['appId','flowId','operationId','version','status','action']
        AND change_summary-ARRAY['appId','flowId','operationId','version','status','action']='{}'::jsonb
        AND jsonb_typeof(change_summary->'appId')='string'
        AND (change_summary->>'appId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
        AND change_summary->>'appId'<>'00000000-0000-0000-0000-000000000000'
        AND jsonb_typeof(change_summary->'flowId')='string'
        AND (change_summary->>'flowId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
        AND change_summary->>'flowId'<>'00000000-0000-0000-0000-000000000000'
        AND jsonb_typeof(change_summary->'operationId')='string'
        AND (change_summary->>'operationId') ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
        AND change_summary->>'operationId'<>'00000000-0000-0000-0000-000000000000'
        AND jsonb_typeof(change_summary->'version')='number'
        AND (change_summary->>'version') ~ '^[0-9]+$'
        AND (change_summary->>'version')::numeric BETWEEN 1 AND 9007199254740991
        AND jsonb_typeof(change_summary->'status')='string'
        AND jsonb_typeof(change_summary->'action')='string'
        AND ((reason_code='WORKFLOW_PUBLICATION_REQUESTED' AND change_summary->>'action'='workflow.publish.requested' AND change_summary->>'status'='pending' AND outcome='success')
          OR (reason_code='WORKFLOW_PUBLICATION_CONFIRMED' AND change_summary->>'action'='workflow.publish.confirmed' AND change_summary->>'status'='confirmed' AND outcome='success')
          OR (reason_code='WORKFLOW_PUBLICATION_SUPERSEDED' AND change_summary->>'action'='workflow.publish.superseded' AND change_summary->>'status'='superseded' AND outcome='success')
          OR (reason_code='WORKFLOW_PUBLICATION_BLOCKED' AND change_summary->>'action'='workflow.publish.blocked' AND change_summary->>'status'='blocked' AND outcome='failure'))
    ),false)
);

