SELECT 'private',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id),'[]') FROM applications.table_presets p;
SELECT 'receipts',COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.actor_user_id,o.operation_id),'[]') FROM applications.operations o WHERE operation_kind IN('preset.create','preset.update','preset.discard');
SELECT 'legacy',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id),'[]') FROM personnel.table_presets p;
SELECT 'fields',COALESCE(jsonb_agg(to_jsonb(f) ORDER BY f.id),'[]') FROM applications.fields f;
SELECT 'forms',COALESCE(jsonb_agg(to_jsonb(f) ORDER BY f.id),'[]') FROM applications.form_views f;
SELECT 'tables',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.logical_tables t;
SELECT 'roles',jsonb_build_array(has_table_privilege('auth_backup','applications.table_presets','SELECT'),has_column_privilege('auth_app','applications.table_presets','name','UPDATE'),has_column_privilege('auth_app','applications.table_presets','owner_user_id','UPDATE'),has_table_privilege('auth_reader','applications.table_presets','SELECT'));
