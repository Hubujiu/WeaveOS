-- +goose Up
-- Q25 reviewed physical dictionary; never rewrite the published authentication migration.
CREATE SCHEMA personnel;
CREATE TABLE personnel.departments (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  parent_id uuid,
  name varchar(100) NOT NULL,
  is_root boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_departments PRIMARY KEY (id),
  CONSTRAINT fk_departments_parent_id FOREIGN KEY (parent_id) REFERENCES personnel.departments (id) ON DELETE RESTRICT,
  CONSTRAINT ck_departments_name CHECK (btrim(name) <> ''),
  CONSTRAINT ck_departments_version CHECK (version > 0),
  CONSTRAINT ck_departments_root CHECK ((is_root AND parent_id IS NULL) OR (NOT is_root AND parent_id IS NOT NULL)),
  CONSTRAINT ck_departments_parent CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE INDEX ix_departments_parent_id ON personnel.departments (parent_id);
CREATE TABLE personnel.department_members (
  department_id uuid NOT NULL,
  user_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_department_members PRIMARY KEY (department_id, user_id),
  CONSTRAINT fk_department_members_department_id FOREIGN KEY (department_id) REFERENCES personnel.departments (id) ON DELETE RESTRICT,
  CONSTRAINT fk_department_members_user_id FOREIGN KEY (user_id) REFERENCES auth.users (id) ON DELETE RESTRICT
);
CREATE INDEX ix_department_members_user_id ON personnel.department_members (user_id);
CREATE TABLE personnel.identities (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  name varchar(100) NOT NULL,
  description varchar(1000) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_identities PRIMARY KEY (id),
  CONSTRAINT ck_identities_name CHECK (btrim(name) <> ''),
  CONSTRAINT ck_identities_version CHECK (version > 0)
);
CREATE TABLE personnel.permission_templates (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  name varchar(100) NOT NULL,
  description varchar(1000) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_permission_templates PRIMARY KEY (id),
  CONSTRAINT ck_permission_templates_name CHECK (btrim(name) <> ''),
  CONSTRAINT ck_permission_templates_version CHECK (version > 0)
);
CREATE TABLE personnel.permission_catalog (
  code varchar(160) NOT NULL,
  name varchar(100) NOT NULL,
  category varchar(16) NOT NULL,
  app_id varchar(100),
  enabled boolean NOT NULL DEFAULT true,
  CONSTRAINT pk_permission_catalog PRIMARY KEY (code),
  CONSTRAINT ck_permission_catalog_name CHECK (btrim(name) <> ''),
  CONSTRAINT uq_permission_catalog_app_id UNIQUE (app_id),
  CONSTRAINT ck_permission_catalog_category CHECK ((category = 'system' AND code = 'personnel.manage' AND app_id IS NULL) OR (category = 'application' AND app_id IS NOT NULL AND btrim(app_id) <> ''))
);
CREATE TABLE personnel.member_configuration (
  user_id uuid NOT NULL,
  version bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_member_configuration PRIMARY KEY (user_id),
  CONSTRAINT fk_member_configuration_user_id FOREIGN KEY (user_id) REFERENCES auth.users (id) ON DELETE RESTRICT,
  CONSTRAINT ck_member_configuration_version CHECK (version >= 0)
);
CREATE TABLE personnel.member_identities (
  user_id uuid NOT NULL,
  identity_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_member_identities PRIMARY KEY (user_id, identity_id),
  CONSTRAINT fk_member_identities_user_id FOREIGN KEY (user_id) REFERENCES auth.users (id) ON DELETE RESTRICT,
  CONSTRAINT fk_member_identities_identity_id FOREIGN KEY (identity_id) REFERENCES personnel.identities (id) ON DELETE RESTRICT
);
CREATE INDEX ix_member_identities_identity_id ON personnel.member_identities (identity_id);
CREATE TABLE personnel.identity_templates (
  identity_id uuid NOT NULL,
  template_id uuid NOT NULL,
  CONSTRAINT pk_identity_templates PRIMARY KEY (identity_id, template_id),
  CONSTRAINT fk_identity_templates_identity_id FOREIGN KEY (identity_id) REFERENCES personnel.identities (id) ON DELETE RESTRICT,
  CONSTRAINT fk_identity_templates_template_id FOREIGN KEY (template_id) REFERENCES personnel.permission_templates (id) ON DELETE RESTRICT
);
CREATE INDEX ix_identity_templates_template_id ON personnel.identity_templates (template_id);
CREATE TABLE personnel.identity_permissions (
  identity_id uuid NOT NULL,
  permission_code varchar(160) NOT NULL,
  CONSTRAINT pk_identity_permissions PRIMARY KEY (identity_id, permission_code),
  CONSTRAINT fk_identity_permissions_identity_id FOREIGN KEY (identity_id) REFERENCES personnel.identities (id) ON DELETE RESTRICT,
  CONSTRAINT fk_identity_permissions_permission_code FOREIGN KEY (permission_code) REFERENCES personnel.permission_catalog (code) ON DELETE RESTRICT
);
CREATE TABLE personnel.template_permissions (
  template_id uuid NOT NULL,
  permission_code varchar(160) NOT NULL,
  CONSTRAINT pk_template_permissions PRIMARY KEY (template_id, permission_code),
  CONSTRAINT fk_template_permissions_template_id FOREIGN KEY (template_id) REFERENCES personnel.permission_templates (id) ON DELETE RESTRICT,
  CONSTRAINT fk_template_permissions_permission_code FOREIGN KEY (permission_code) REFERENCES personnel.permission_catalog (code) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_departments_single_root ON personnel.departments (is_root) WHERE is_root;
INSERT INTO personnel.departments(name, is_root) VALUES ('企业', true);
INSERT INTO personnel.permission_catalog(code,name,category) VALUES ('personnel.manage','人员管理','system');

REVOKE ALL ON SCHEMA personnel FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA personnel FROM PUBLIC;
ALTER TABLE auth.authentication_events
  ADD COLUMN object_type varchar(24),
  ADD COLUMN object_id uuid,
  ADD COLUMN change_summary jsonb;
ALTER TABLE auth.authentication_events DROP CONSTRAINT ck_authentication_events_type;
ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_type CHECK (event_type IN ('register','login','logout','invitation_created','password_reset','session_invalid','account_status_changed','bootstrap_created','personnel_changed'));
ALTER TABLE auth.authentication_events ADD CONSTRAINT ck_auth_events_summary CHECK (
  (event_type = 'personnel_changed' AND object_type IS NOT NULL AND object_id IS NOT NULL AND change_summary IS NOT NULL
    AND object_type IN ('department','member','identity','template') AND jsonb_typeof(change_summary) = 'object')
  OR (event_type <> 'personnel_changed' AND object_type IS NULL AND object_id IS NULL AND change_summary IS NULL)
);
CREATE VIEW personnel.activity_events WITH (security_barrier = true) AS
SELECT e.id, e.occurred_at, COALESCE(u.account, '系统') AS actor_account,
       COALESCE(e.reason_code, upper(e.event_type)) AS action,
       COALESCE(e.object_type, 'invitation') AS object_type, e.object_id,
       e.change_summary, e.outcome
FROM auth.authentication_events e LEFT JOIN auth.users u ON u.id = e.actor_user_id
WHERE e.event_type IN ('personnel_changed', 'invitation_created');
REVOKE ALL ON personnel.activity_events FROM PUBLIC;
-- No destructive Down: old authentication remains compatible; personnel data is durable.

