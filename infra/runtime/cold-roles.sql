-- Apply after separate archive migration and shared NOLOGIN group creation.
REVOKE ALL ON SCHEMA archive FROM PUBLIC, auth_app, auth_reader;
REVOKE ALL ON ALL TABLES IN SCHEMA archive FROM PUBLIC, auth_app, auth_reader, auth_maintenance;
GRANT USAGE ON SCHEMA archive TO auth_maintenance;
GRANT SELECT, INSERT, DELETE ON archive.authentication_events TO auth_maintenance;
