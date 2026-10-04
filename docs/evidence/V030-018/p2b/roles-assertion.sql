DO $check$
BEGIN
    IF NOT has_table_privilege('auth_app', 'applications.workflow_commands', 'SELECT')
       OR NOT has_table_privilege('auth_app', 'applications.workflow_commands', 'INSERT')
       OR has_table_privilege('auth_app', 'applications.workflow_commands', 'UPDATE')
       OR NOT has_column_privilege('auth_app', 'applications.workflow_commands', 'state', 'UPDATE')
       OR NOT has_column_privilege('auth_app', 'applications.workflow_commands', 'receipt_json', 'UPDATE')
       OR has_column_privilege('auth_app', 'applications.workflow_commands', 'command_id', 'UPDATE')
       OR has_column_privilege('auth_app', 'applications.workflow_commands', 'command_json', 'UPDATE')
       OR has_column_privilege('auth_app', 'applications.workflow_commands', 'command_hash', 'UPDATE')
       OR has_column_privilege('auth_app', 'applications.workflow_commands', 'created_at', 'UPDATE')
       OR has_table_privilege('auth_app', 'applications.workflow_commands', 'DELETE')
       OR has_table_privilege('auth_app', 'applications.workflow_commands', 'TRUNCATE') THEN
        RAISE EXCEPTION 'auth_app command grants do not match the P2b contract';
    END IF;
    IF NOT has_table_privilege('auth_app', 'applications.workflow_dispatch', 'SELECT')
       OR NOT has_table_privilege('auth_app', 'applications.workflow_dispatch', 'INSERT')
       OR NOT has_table_privilege('auth_app', 'applications.workflow_dispatch', 'DELETE')
       OR has_table_privilege('auth_app', 'applications.workflow_dispatch', 'UPDATE')
       OR has_table_privilege('auth_app', 'applications.workflow_dispatch', 'TRUNCATE') THEN
        RAISE EXCEPTION 'auth_app dispatch grants do not match the P2b contract';
    END IF;
    IF NOT has_table_privilege('auth_backup', 'applications.workflow_commands', 'SELECT')
       OR NOT has_table_privilege('auth_backup', 'applications.workflow_dispatch', 'SELECT')
       OR has_table_privilege('auth_backup', 'applications.workflow_commands', 'INSERT')
       OR has_table_privilege('auth_backup', 'applications.workflow_commands', 'UPDATE')
       OR has_table_privilege('auth_backup', 'applications.workflow_commands', 'DELETE')
       OR has_table_privilege('auth_backup', 'applications.workflow_dispatch', 'INSERT')
       OR has_table_privilege('auth_backup', 'applications.workflow_dispatch', 'UPDATE')
       OR has_table_privilege('auth_backup', 'applications.workflow_dispatch', 'DELETE') THEN
        RAISE EXCEPTION 'auth_backup grants do not match the P2b contract';
    END IF;
    IF has_table_privilege('auth_reader', 'applications.workflow_commands', 'SELECT')
       OR has_table_privilege('auth_reader', 'applications.workflow_dispatch', 'SELECT')
       OR has_table_privilege('auth_maintenance', 'applications.workflow_commands', 'SELECT')
       OR has_table_privilege('auth_maintenance', 'applications.workflow_dispatch', 'SELECT') THEN
        RAISE EXCEPTION 'reader or maintenance has new ledger privileges';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_class AS relation
        JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
        CROSS JOIN LATERAL aclexplode(COALESCE(relation.relacl, acldefault('r', relation.relowner))) AS acl
        WHERE namespace.nspname = 'applications'
          AND relation.relname IN ('workflow_commands', 'workflow_dispatch')
          AND acl.grantee = 0
    ) THEN
        RAISE EXCEPTION 'PUBLIC has privileges on the workflow ledger';
    END IF;
END;
$check$;
