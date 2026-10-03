-- +goose Up
-- V030-013 cold-first compatible audit addition. Old constraints retained.
-- +goose StatementBegin
DO $$ DECLARE old_expression text;
BEGIN
 SELECT pg_get_expr(conbin,conrelid) INTO old_expression FROM pg_constraint WHERE conrelid='archive.authentication_events'::regclass AND conname='ck_archive_events_type';
 ALTER TABLE archive.authentication_events DROP CONSTRAINT ck_archive_events_type;
 EXECUTE format('ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_type CHECK((%s) OR event_type=''application_structure_changed'')',old_expression);
 SELECT pg_get_expr(conbin,conrelid) INTO old_expression FROM pg_constraint WHERE conrelid='archive.authentication_events'::regclass AND conname='ck_archive_events_summary';
 ALTER TABLE archive.authentication_events DROP CONSTRAINT ck_archive_events_summary;
 EXECUTE format('ALTER TABLE archive.authentication_events ADD CONSTRAINT ck_archive_events_summary CHECK((event_type<>''application_structure_changed'' AND (%s)) OR COALESCE((event_type=''application_structure_changed'' AND actor_user_id IS NOT NULL AND object_id IS NOT NULL AND object_type IN (''directory'',''table'',''form'') AND jsonb_typeof(change_summary)=''object'' AND change_summary ?& ARRAY[''appId'',''operationId'',''structureVersion'',''schemaVersion'',''viewVersion'',''changeCount''] AND change_summary-ARRAY[''appId'',''operationId'',''structureVersion'',''schemaVersion'',''viewVersion'',''changeCount'']=''{}''::jsonb),false))',old_expression);
END $$;
-- +goose StatementEnd
-- No destructive Down.
