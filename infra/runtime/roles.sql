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
GRANT USAGE ON SCHEMA personnel TO auth_app, auth_backup;
GRANT SELECT, INSERT, UPDATE, DELETE ON
 personnel.departments, personnel.department_members, personnel.identities,
 personnel.permission_templates, personnel.member_configuration,
 personnel.member_identities, personnel.identity_templates,
 personnel.identity_permissions, personnel.template_permissions TO auth_app;
GRANT SELECT ON personnel.permission_catalog, personnel.activity_events TO auth_app;
GRANT SELECT, INSERT, DELETE ON personnel.drafts TO auth_app;
GRANT UPDATE (payload_json, draft_version, updated_at) ON personnel.drafts TO auth_app;
GRANT SELECT ON ALL TABLES IN SCHEMA personnel TO auth_backup;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA personnel TO auth_backup;
REVOKE ALL ON FUNCTION personnel.lock_permission_catalog(uuid) FROM PUBLIC, auth_reader, auth_maintenance, auth_backup;
GRANT EXECUTE ON FUNCTION personnel.lock_permission_catalog(uuid) TO auth_app;
