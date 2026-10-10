SELECT 'lifecycle',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.app_id,p.table_id,p.record_id),'[]') FROM applications.record_lifecycle p;
SELECT 'events',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id),'[]') FROM applications.record_lifecycle_events p;
SELECT 'operations',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.actor_user_id,p.operation_id),'[]') FROM applications.operations p;
SELECT format('SELECT %L,COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),%L) FROM %I.%I r;',tablename,'[]',schemaname,tablename) FROM pg_tables WHERE schemaname IN ('appdata','applications') ORDER BY schemaname,tablename
\gexec
SELECT 'roles',jsonb_build_array(has_table_privilege('auth_backup','applications.record_lifecycle','SELECT'),has_table_privilege('auth_app','applications.record_lifecycle','SELECT'),has_table_privilege('auth_app','applications.record_lifecycle','UPDATE'),has_table_privilege('auth_reader','applications.record_lifecycle','SELECT'));

