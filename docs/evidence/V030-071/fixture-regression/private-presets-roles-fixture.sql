-- Isolated synthetic database roles only; not a production role update.
REVOKE ALL ON applications.table_presets FROM PUBLIC,auth_app,auth_reader,auth_maintenance,auth_backup;
GRANT SELECT,INSERT,DELETE ON applications.table_presets TO auth_app;
GRANT UPDATE(name,definition_json,field_kinds,version,updated_at) ON applications.table_presets TO auth_app;
GRANT SELECT ON applications.table_presets TO auth_backup;
