-- Root-authored callable declarations for RED in an isolated test DB only.
-- This file is not a production migration and has no business behavior.
CREATE OR REPLACE FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint)
RETURNS bigint LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS 'SELECT 0::bigint';
CREATE OR REPLACE FUNCTION applications.release_record_command_fence(uuid,uuid,uuid,uuid,bigint,bigint)
RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS 'SELECT false';
REVOKE ALL ON FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) FROM PUBLIC;
REVOKE ALL ON FUNCTION applications.release_record_command_fence(uuid,uuid,uuid,uuid,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION applications.acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint) TO auth_app;
GRANT EXECUTE ON FUNCTION applications.release_record_command_fence(uuid,uuid,uuid,uuid,bigint,bigint) TO auth_app;
