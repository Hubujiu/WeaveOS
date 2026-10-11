SELECT format('SELECT %L,COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),%L) FROM %I.%I r;',schemaname||'.'||tablename,'[]',schemaname,tablename) FROM pg_tables WHERE schemaname IN ('auth','personnel','appdata','applications') ORDER BY schemaname,tablename
\gexec
SELECT 'copy-role',has_table_privilege('auth_backup','applications.operations','SELECT'),has_table_privilege('auth_app','applications.operations','DELETE'),has_schema_privilege('auth_app','appdata','CREATE');
