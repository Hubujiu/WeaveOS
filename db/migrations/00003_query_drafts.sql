-- +goose Up
-- Approved Q36 PLAN: structures only. Writer coverage/lock functions follow in
-- a new migration after their real transaction RED; these rows alone do not
-- implement query invalidation. Existing migrations are immutable.
CREATE TABLE personnel.query_revisions (
  scope text NOT NULL,
  revision bigint NOT NULL DEFAULT 1,
  CONSTRAINT pk_query_revisions PRIMARY KEY (scope),
  CONSTRAINT ck_query_revisions_scope CHECK (scope IN ('people', 'configuration', 'activity')),
  CONSTRAINT ck_query_revisions_positive CHECK (revision > 0)
);
INSERT INTO personnel.query_revisions (scope) VALUES ('people'), ('configuration'), ('activity');

CREATE TABLE personnel.drafts (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL,
  slot smallint NOT NULL,
  kind text NOT NULL,
  target_id uuid,
  base_version bigint,
  draft_version bigint NOT NULL DEFAULT 1,
  -- The service writes canonical UTF-8 JSON. Preserve that representation so
  -- the byte limit does not depend on PostgreSQL jsonb's whitespace rendering.
  payload_json text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_drafts PRIMARY KEY (id),
  CONSTRAINT fk_drafts_owner FOREIGN KEY (owner_user_id) REFERENCES auth.users (id) ON DELETE RESTRICT,
  -- Twenty owner-local slots impose the capacity bound even under concurrent
  -- inserts. Allocation/retry and the domain error are service responsibilities.
  CONSTRAINT uq_drafts_owner_slot UNIQUE (owner_user_id, slot),
  CONSTRAINT ck_drafts_slot CHECK (slot BETWEEN 1 AND 20),
  CONSTRAINT ck_drafts_kind CHECK (kind IN ('member-identities', 'member-groups', 'department', 'identity', 'template')),
  CONSTRAINT ck_drafts_target_base CHECK (
    (kind IN ('member-identities', 'member-groups') AND target_id IS NOT NULL AND base_version IS NOT NULL AND base_version BETWEEN 0 AND 9007199254740991)
    OR
    (kind IN ('department', 'identity', 'template') AND (
      (target_id IS NULL AND base_version IS NULL)
      OR (target_id IS NOT NULL AND base_version IS NOT NULL AND base_version BETWEEN 1 AND 9007199254740991)
    ))
  ),
  CONSTRAINT ck_drafts_version CHECK (draft_version BETWEEN 1 AND 9007199254740991),
  CONSTRAINT ck_drafts_payload_bytes CHECK (octet_length(payload_json) <= 65536),
  CONSTRAINT ck_drafts_payload_object CHECK (jsonb_typeof(payload_json::jsonb) = 'object')
);
CREATE INDEX ix_drafts_owner_updated ON personnel.drafts (owner_user_id, updated_at DESC, id DESC);
COMMENT ON COLUMN personnel.drafts.target_id IS 'No target FK: preserve explicit inputs after the business target is deleted; restore must show the conflict.';
COMMENT ON COLUMN personnel.drafts.base_version IS 'Original business object version; immutable on draft update. NULL only for a new definition/department.';
COMMENT ON TABLE personnel.drafts IS 'Personal explicit saves, no automatic expiry. Live authorization, field allowlist, canonical encoding and CAS are enforced by the service; no business revision bump.';
REVOKE ALL ON personnel.query_revisions, personnel.drafts FROM PUBLIC;

-- +goose Down
-- Only for an explicitly authorized rollback: destroys saved drafts. The
-- revision-dependent application must be rolled back before this migration.
DROP TABLE personnel.drafts;
DROP TABLE personnel.query_revisions;
