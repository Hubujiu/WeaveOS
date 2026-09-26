-- Run by the migration/role owner, never by BFF. Login secrets are injected
-- separately; these NOLOGIN groups carry only their explicit capabilities.
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='auth_app') THEN CREATE ROLE auth_app NOLOGIN; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='auth_reader') THEN CREATE ROLE auth_reader NOLOGIN; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='auth_maintenance') THEN CREATE ROLE auth_maintenance NOLOGIN; END IF;
END $$;
REVOKE ALL ON SCHEMA auth FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA auth FROM PUBLIC, auth_app, auth_reader, auth_maintenance;
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
