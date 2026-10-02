-- +goose Up
-- ADR008 A1/A7: personal configurations; no business revision/audit triggers.
CREATE TABLE personnel.table_presets (
 id uuid NOT NULL DEFAULT gen_random_uuid(),
 owner_id uuid NOT NULL,
 view_key text NOT NULL,
 name text COLLATE "C" NOT NULL,
 slot smallint NOT NULL,
 -- Preserve canonical UTF-8 JSON bytes, independent of jsonb rendering spaces.
 filter_json text,
 hidden_column_ids jsonb NOT NULL,
 schema_version integer NOT NULL DEFAULT 1,
 version bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT pk_table_presets PRIMARY KEY(id),
 CONSTRAINT fk_table_presets_owner FOREIGN KEY(owner_id) REFERENCES auth.users(id) ON DELETE RESTRICT,
 CONSTRAINT uq_table_presets_owner_view_slot UNIQUE(owner_id,view_key,slot),
 CONSTRAINT uq_table_presets_owner_view_name UNIQUE(owner_id,view_key,name),
 CONSTRAINT ck_table_presets_view CHECK(view_key IN ('members','events')),
 CONSTRAINT ck_table_presets_name CHECK(char_length(name) BETWEEN 1 AND 100 AND name=btrim(name)),
 CONSTRAINT ck_table_presets_slot CHECK(slot BETWEEN 1 AND 20),
 CONSTRAINT ck_table_presets_filter CHECK(filter_json IS NULL OR (octet_length(filter_json)<=16384 AND jsonb_typeof(filter_json::jsonb)='object')),
 CONSTRAINT ck_table_presets_hidden CHECK(jsonb_typeof(hidden_column_ids)='array' AND jsonb_array_length(hidden_column_ids)<CASE view_key WHEN 'members' THEN 4 ELSE 6 END),
 CONSTRAINT ck_table_presets_schema CHECK(schema_version=1),
 CONSTRAINT ck_table_presets_version CHECK(version BETWEEN 1 AND 9007199254740991)
);
CREATE INDEX ix_table_presets_owner_view_updated ON personnel.table_presets(owner_id,view_key,updated_at DESC,id ASC);
COMMENT ON TABLE personnel.table_presets IS 'Explicit personal table configurations, no expiry or active/query state. Service validates canonical 32KiB content, editable AST, Unicode names, references and column allowlists.';
REVOKE ALL ON personnel.table_presets FROM PUBLIC;

-- +goose Down
-- Explicitly authorized destructive rollback only. Artifact rollback keeps this
-- compatible expansion and saved configurations; never run Down to mimic recovery.
DROP TABLE personnel.table_presets;
