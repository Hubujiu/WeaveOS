-- Run by the migration/role owner, never by BFF. Login secrets are injected
-- separately; these NOLOGIN groups carry only their explicit capabilities.
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='auth_app') THEN CREATE ROLE auth_app NOLOGIN; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='auth_reader') THEN CREATE ROLE auth_reader NOLOGIN; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='auth_maintenance') THEN CREATE ROLE auth_maintenance NOLOGIN; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='auth_backup') THEN CREATE ROLE auth_backup NOLOGIN; END IF;
END $$;
REVOKE ALL ON SCHEMA auth FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA auth FROM PUBLIC, auth_app, auth_reader, auth_maintenance;
REVOKE ALL ON ALL TABLES IN SCHEMA auth FROM auth_backup;
GRANT USAGE ON SCHEMA auth TO auth_app, auth_reader, auth_maintenance;
GRANT SELECT ON auth.users, auth.password_credentials, auth.invitations TO auth_app;
GRANT INSERT (id,account,status,auth_version,created_at,updated_at) ON auth.users TO auth_app;
GRANT UPDATE (status,auth_version,updated_at) ON auth.users TO auth_app;
GRANT INSERT ON auth.password_credentials TO auth_app;
GRANT UPDATE (password_hash,password_changed_at,updated_at) ON auth.password_credentials TO auth_app;
GRANT INSERT (id,code_hash,created_by,created_at) ON auth.invitations TO auth_app;
GRANT UPDATE (used_by,used_at) ON auth.invitations TO auth_app;
GRANT INSERT ON auth.authentication_events TO auth_app;
GRANT SELECT (id) ON auth.authentication_events TO auth_app;
GRANT SELECT ON auth.users, auth.authentication_events TO auth_reader;
GRANT SELECT, DELETE ON auth.authentication_events TO auth_maintenance;
-- PostgreSQL FOR UPDATE row locking requires UPDATE on at least one column.
-- Only the controlled mover receives this; application history remains append-only.
GRANT UPDATE (id) ON auth.authentication_events TO auth_maintenance;
GRANT USAGE ON SCHEMA auth, public TO auth_backup;
GRANT SELECT ON ALL TABLES IN SCHEMA auth, public TO auth_backup;
-- pg_dump must read original sequence positions; SELECT cannot advance/set them.
GRANT SELECT ON ALL SEQUENCES IN SCHEMA auth, public TO auth_backup;
-- Q25 personnel configuration belongs to the application; application registration
-- remains owner-controlled. Audit activity is read only through the safe view.
REVOKE ALL ON SCHEMA personnel FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA personnel FROM PUBLIC, auth_app, auth_reader, auth_maintenance, auth_backup;
GRANT USAGE ON SCHEMA personnel TO auth_app, auth_backup, auth_maintenance;
GRANT SELECT, INSERT, UPDATE, DELETE ON
 personnel.departments, personnel.department_members, personnel.identities,
 personnel.permission_templates, personnel.member_configuration,
 personnel.member_identities, personnel.identity_templates,
 personnel.identity_permissions, personnel.template_permissions TO auth_app;
GRANT SELECT ON personnel.permission_catalog, personnel.activity_events TO auth_app;
GRANT SELECT, INSERT, DELETE ON personnel.drafts TO auth_app;
GRANT UPDATE (payload_json, draft_version, updated_at) ON personnel.drafts TO auth_app;
GRANT SELECT, INSERT, DELETE ON personnel.table_presets TO auth_app;
GRANT UPDATE (name, filter_json, hidden_column_ids, schema_version, version, updated_at) ON personnel.table_presets TO auth_app;
GRANT SELECT ON ALL TABLES IN SCHEMA personnel TO auth_backup;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA personnel TO auth_backup;
REVOKE ALL ON FUNCTION personnel.lock_permission_catalog(uuid) FROM PUBLIC, auth_reader, auth_maintenance, auth_backup;
GRANT EXECUTE ON FUNCTION personnel.lock_permission_catalog(uuid) TO auth_app;

-- Revisions are read-only signals; only the fixed-order lock function is callable.
GRANT SELECT ON personnel.query_revisions TO auth_app;
GRANT EXECUTE ON FUNCTION personnel.lock_query_revisions() TO auth_app, auth_maintenance;

-- B5a finite application policy. Runtime never owns catalog CRUD or owner transfer.
REVOKE ALL ON SCHEMA applications FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA applications FROM PUBLIC, auth_app, auth_reader, auth_maintenance, auth_backup;
GRANT USAGE ON SCHEMA applications TO auth_app, auth_backup;
GRANT SELECT, INSERT ON applications.apps TO auth_app;
GRANT UPDATE (policy_revision,updated_at) ON applications.apps TO auth_app;
GRANT SELECT, INSERT ON applications.permission_groups TO auth_app;
GRANT UPDATE (name,enabled,updated_at) ON applications.permission_groups TO auth_app;
GRANT SELECT, INSERT, DELETE ON applications.group_members, applications.grants, applications.grant_fields TO auth_app;
GRANT SELECT, INSERT ON applications.menu_resources, applications.operations TO auth_app;
GRANT UPDATE (result_json,http_status,location) ON applications.operations TO auth_app;
REVOKE ALL ON FUNCTION applications.register_catalog_entry(uuid) FROM PUBLIC, auth_reader, auth_maintenance, auth_backup;
GRANT EXECUTE ON FUNCTION applications.register_catalog_entry(uuid) TO auth_app;
GRANT SELECT ON ALL TABLES IN SCHEMA applications TO auth_backup;

-- Canonical record save deltas: trusted service append/read; no UPDATE/DELETE,
-- no authentication-audit reader access and no automatic value-history purge.
GRANT SELECT,INSERT ON applications.record_change_events,applications.record_change_values TO auth_app;
GRANT SELECT ON applications.field_option_tombstones TO auth_app;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA applications TO auth_backup;

-- V030-013 explicit definition capabilities. No schema CREATE/ownership granted.

-- V015 finite typed DML and explicit draft/source/minimum-audit capabilities.
-- auth_app is a trusted BFF service role, not an independent end-user Session.
GRANT SELECT,INSERT,DELETE ON applications.record_drafts TO auth_app;
GRANT UPDATE (values_json,draft_version,updated_at) ON applications.record_drafts TO auth_app;
GRANT SELECT,INSERT ON applications.record_write_audit TO auth_app;
GRANT SELECT ON applications.record_command_fences,applications.member_sources,
 applications.department_sources,applications.member_department_sources,
 applications.reference_source_revision TO auth_app;
REVOKE ALL ON FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb) TO auth_app;

GRANT UPDATE (structure_version) ON applications.apps TO auth_app;
GRANT SELECT,INSERT,UPDATE ON applications.directories,applications.logical_tables,applications.fields,applications.form_views TO auth_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON applications.table_field_dependencies TO auth_app;
GRANT USAGE ON SCHEMA appdata TO auth_app,auth_backup;
REVOKE ALL ON FUNCTION applications.apply_schema_change(uuid,uuid,uuid,text,jsonb,jsonb),applications.apply_option_mapping(uuid,uuid,uuid,uuid,text,jsonb,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION applications.apply_schema_change(uuid,uuid,uuid,text,jsonb,jsonb) TO auth_app;
GRANT EXECUTE ON FUNCTION applications.apply_option_mapping(uuid,uuid,uuid,uuid,text,jsonb,jsonb) TO auth_app;
GRANT SELECT ON ALL TABLES IN SCHEMA appdata TO auth_backup;
GRANT SELECT ON ALL TABLES IN SCHEMA applications TO auth_backup;

-- V030-018 P2a: finite authority over a record command fence only.
GRANT EXECUTE ON FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) TO auth_app;
GRANT EXECUTE ON FUNCTION applications.release_record_command_fence(uuid,uuid,uuid,uuid,bigint,bigint) TO auth_app;

-- V030-018 P2b: durable command identity/history is append-only except for
-- the two state columns advanced by the trusted ledger in its caller tx.
REVOKE ALL ON applications.workflow_commands,applications.workflow_dispatch
 FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;
GRANT SELECT,INSERT ON applications.workflow_commands TO auth_app;
GRANT UPDATE (state,receipt_json) ON applications.workflow_commands TO auth_app;
GRANT SELECT,INSERT,DELETE ON applications.workflow_dispatch TO auth_app;
GRANT SELECT ON applications.workflow_commands,applications.workflow_dispatch TO auth_backup;

-- V030-018 P2c workflow catalog: immutable version and deployment history;
-- mutable lifecycle columns are limited to the trusted application runtime.
REVOKE ALL ON applications.workflow_definitions,applications.workflow_versions,
 applications.workflow_deployments,applications.workflow_instances
 FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;
GRANT SELECT,INSERT ON applications.workflow_definitions,applications.workflow_versions,
 applications.workflow_deployments,applications.workflow_instances TO auth_app;
GRANT UPDATE (name,revision,state,current_version,candidate_version,updated_at)
 ON applications.workflow_definitions TO auth_app;
GRANT UPDATE (state,sequence,updated_at) ON applications.workflow_instances TO auth_app;
GRANT SELECT ON applications.workflow_definitions,applications.workflow_versions,
 applications.workflow_deployments,applications.workflow_instances TO auth_backup;

-- V030-027 finite mutable dispatch state; identity and complete receipts are append-only.
REVOKE ALL ON applications.workflow_publications,applications.workflow_engine_receipts FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;
GRANT SELECT,INSERT ON applications.workflow_publications,applications.workflow_engine_receipts TO auth_app;
GRANT UPDATE(status,attempts,next_attempt_at,lease_token,lease_until,reason,updated_at,completed_at) ON applications.workflow_publications TO auth_app;
GRANT UPDATE(close_epoch) ON applications.workflow_definitions TO auth_app;
GRANT SELECT ON applications.workflow_publications,applications.workflow_engine_receipts TO auth_backup;

-- V030-033: confirmed execution projections; events are append-only to runtime.
REVOKE ALL ON applications.workflow_execution_events,applications.workflow_tasks FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;
GRANT SELECT,INSERT ON applications.workflow_execution_events,applications.workflow_tasks TO auth_app;
GRANT UPDATE(closed_command_id) ON applications.workflow_tasks TO auth_app;
GRANT UPDATE(engine_process_id) ON applications.workflow_instances TO auth_app;
GRANT SELECT ON applications.workflow_execution_events,applications.workflow_tasks TO auth_backup;

-- V030-035: scheduler leases never grant mutation of accepted payload or identity.
GRANT UPDATE(next_attempt_at,attempts,lease_token,lease_until,last_error) ON applications.workflow_dispatch TO auth_app;

-- V030-036: historical approval evidence is append-only to the runtime.
REVOKE ALL ON applications.workflow_evidence_blobs,applications.workflow_evidence_documents,applications.workflow_evidence_members FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;
GRANT SELECT,INSERT ON applications.workflow_evidence_blobs,applications.workflow_evidence_documents,applications.workflow_evidence_members TO auth_app;
GRANT SELECT ON applications.workflow_evidence_blobs,applications.workflow_evidence_documents,applications.workflow_evidence_members TO auth_backup;
