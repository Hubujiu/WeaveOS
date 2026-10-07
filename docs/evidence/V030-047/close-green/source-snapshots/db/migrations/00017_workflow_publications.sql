-- +goose Up
-- V030-027: immutable publication identity, finite leased dispatch and full receipts.
ALTER TABLE applications.workflow_definitions ADD COLUMN close_epoch bigint NOT NULL DEFAULT 0 CHECK(close_epoch BETWEEN 0 AND 9007199254740991);
ALTER TABLE applications.workflow_definitions ADD CONSTRAINT uq_workflow_publication_view UNIQUE(app_id,id,view_id);
ALTER TABLE applications.workflow_versions ADD CONSTRAINT uq_workflow_publication_version UNIQUE(app_id,flow_id,version,version_id);
CREATE TABLE applications.workflow_publications (
 actor_user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 operation_id uuid NOT NULL CHECK(operation_id<>'00000000-0000-0000-0000-000000000000'),
 app_id uuid NOT NULL CHECK(app_id<>'00000000-0000-0000-0000-000000000000'),
 view_id uuid NOT NULL CHECK(view_id<>'00000000-0000-0000-0000-000000000000'),
 flow_id uuid NOT NULL CHECK(flow_id<>'00000000-0000-0000-0000-000000000000'),
 version bigint NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991),
 version_id uuid NOT NULL CHECK(version_id<>'00000000-0000-0000-0000-000000000000'),
 bpmn_sha256 bytea NOT NULL CHECK(octet_length(bpmn_sha256)=32),
 actor_auth_version bigint NOT NULL CHECK(actor_auth_version>0),
 accepted_close_epoch bigint NOT NULL CHECK(accepted_close_epoch BETWEEN 0 AND 9007199254740991),
 expected_revision bigint NOT NULL CHECK(expected_revision BETWEEN 1 AND 9007199254740991),
 expected_schema_version bigint NOT NULL CHECK(expected_schema_version BETWEEN 1 AND 9007199254740991),
 fingerprint bytea NOT NULL CHECK(octet_length(fingerprint)=32),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','unknown','confirmed','superseded','blocked')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_token uuid, lease_until timestamptz,
 reason text CHECK(reason IN ('DEPENDENCY_UNAVAILABLE','AUTHORITY_REVOKED','APPROVER_INELIGIBLE','SCHEMA_INCOMPATIBLE','FLOW_CLOSED','ENGINE_CONFLICT','ENGINE_REPLY_INVALID','CANDIDATE_SUPERSEDED')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 completed_at timestamptz,
 PRIMARY KEY(actor_user_id,operation_id),
 FOREIGN KEY(app_id,flow_id,view_id) REFERENCES applications.workflow_definitions(app_id,id,view_id) ON DELETE RESTRICT,
 FOREIGN KEY(app_id,flow_id,version,version_id) REFERENCES applications.workflow_versions(app_id,flow_id,version,version_id) ON DELETE RESTRICT,
 CHECK(actor_user_id<>'00000000-0000-0000-0000-000000000000'),
 CHECK((lease_token IS NULL)=(lease_until IS NULL)),
 CHECK(lease_token IS NULL OR lease_token<>'00000000-0000-0000-0000-000000000000'),
 CHECK((status IN ('confirmed','superseded','blocked'))=(completed_at IS NOT NULL)),
 CHECK(status IN ('pending','unknown') OR lease_token IS NULL),
 CHECK((status='pending' AND reason IS NULL) OR (status='unknown' AND reason IS NOT NULL AND reason IN ('DEPENDENCY_UNAVAILABLE','ENGINE_REPLY_INVALID','ENGINE_CONFLICT')) OR (status='confirmed' AND reason IS NULL) OR (status='superseded' AND reason IS NOT NULL AND reason='CANDIDATE_SUPERSEDED') OR (status='blocked' AND reason IS NOT NULL AND reason IN ('AUTHORITY_REVOKED','APPROVER_INELIGIBLE','SCHEMA_INCOMPATIBLE','FLOW_CLOSED','ENGINE_CONFLICT')))
);
CREATE INDEX ix_workflow_publications_due ON applications.workflow_publications(next_attempt_at,created_at,actor_user_id,operation_id) WHERE status IN ('pending','unknown');
CREATE TABLE applications.workflow_engine_receipts (
 app_id uuid NOT NULL CHECK(app_id<>'00000000-0000-0000-0000-000000000000'), flow_id uuid NOT NULL CHECK(flow_id<>'00000000-0000-0000-0000-000000000000'),
 version bigint NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991),
 version_id uuid NOT NULL UNIQUE CHECK(version_id<>'00000000-0000-0000-0000-000000000000'),
 bpmn_sha256 bytea NOT NULL CHECK(octet_length(bpmn_sha256)=32),
 engine_deployment_id text NOT NULL CHECK(length(engine_deployment_id) BETWEEN 1 AND 200 AND btrim(engine_deployment_id)<>''),
 process_definition_id text NOT NULL CHECK(length(process_definition_id) BETWEEN 1 AND 512 AND btrim(process_definition_id)<>''),
 confirmed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(app_id,flow_id,version),
 FOREIGN KEY(app_id,flow_id,version,version_id) REFERENCES applications.workflow_versions(app_id,flow_id,version,version_id) ON DELETE RESTRICT
);
REVOKE ALL ON applications.workflow_publications,applications.workflow_engine_receipts FROM PUBLIC;
ALTER TABLE auth.authentication_events DROP CONSTRAINT ck_auth_events_summary;
ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_summary CHECK (
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
-- +goose Down
LOCK TABLE applications.workflow_publications,applications.workflow_engine_receipts,applications.workflow_definitions,auth.authentication_events IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications.workflow_publications) OR EXISTS(SELECT 1 FROM applications.workflow_engine_receipts) OR EXISTS(SELECT 1 FROM applications.workflow_definitions WHERE close_epoch>0) THEN
  RAISE EXCEPTION 'durable publication or closing history exists; Down refused' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM auth.authentication_events WHERE reason_code IN ('WORKFLOW_PUBLICATION_REQUESTED','WORKFLOW_PUBLICATION_CONFIRMED','WORKFLOW_PUBLICATION_SUPERSEDED','WORKFLOW_PUBLICATION_BLOCKED')) THEN RAISE EXCEPTION 'publication audit history exists; Down refused' USING ERRCODE='55000'; END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE auth.authentication_events DROP CONSTRAINT ck_auth_events_summary;
ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_summary CHECK (
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
DROP TABLE applications.workflow_engine_receipts;
DROP TABLE applications.workflow_publications;
ALTER TABLE applications.workflow_versions DROP CONSTRAINT uq_workflow_publication_version;
ALTER TABLE applications.workflow_definitions DROP CONSTRAINT uq_workflow_publication_view, DROP COLUMN close_epoch;
