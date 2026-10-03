-- Apply after separate archive migration and shared NOLOGIN group creation.
REVOKE ALL ON SCHEMA archive FROM PUBLIC, auth_app, auth_reader;
REVOKE ALL ON ALL TABLES IN SCHEMA archive FROM PUBLIC, auth_app, auth_reader, auth_maintenance;
GRANT USAGE ON SCHEMA archive TO auth_maintenance;
GRANT SELECT, INSERT, DELETE ON archive.authentication_events TO auth_maintenance;
REVOKE ALL ON ALL TABLES IN SCHEMA archive FROM auth_backup;
GRANT USAGE ON SCHEMA archive, public TO auth_backup;
GRANT SELECT ON ALL TABLES IN SCHEMA archive, public TO auth_backup;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA archive, public TO auth_backup;

-- V030-013 audit constraint addition retains the existing archive reader/mover capability.
