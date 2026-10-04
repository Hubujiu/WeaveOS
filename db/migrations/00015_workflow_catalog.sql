-- +goose Up
-- V030-018 P2c formal workflow catalog. Callers own transactions and auth.
CREATE TABLE applications.workflow_definitions (
 id uuid PRIMARY KEY,
 app_id uuid NOT NULL,
 table_id uuid NOT NULL,
 view_id uuid NOT NULL,
 name varchar(100) NOT NULL CHECK (btrim(name)<>''),
 revision bigint NOT NULL CHECK (revision BETWEEN 1 AND 9007199254740991),
 state varchar(16) NOT NULL CHECK (state IN ('disabled','enabled','closing')),
 current_version bigint NOT NULL DEFAULT 0 CHECK (current_version BETWEEN 0 AND 9007199254740991),
 candidate_version bigint NOT NULL CHECK (candidate_version BETWEEN 1 AND 9007199254740991 AND candidate_version>=current_version),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (app_id,id),
 UNIQUE (app_id,id,table_id,view_id),
 FOREIGN KEY (app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT,
 FOREIGN KEY (app_id,table_id,view_id) REFERENCES applications.form_views(app_id,table_id,id) ON DELETE RESTRICT
);

CREATE TABLE applications.workflow_versions (
 app_id uuid NOT NULL,
 flow_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version BETWEEN 1 AND 9007199254740991),
 version_id uuid NOT NULL UNIQUE,
 schema_version bigint NOT NULL CHECK (schema_version BETWEEN 1 AND 9007199254740991),
 graph_json jsonb NOT NULL CHECK (jsonb_typeof(graph_json)='object'),
 bpmn_xml text NOT NULL CHECK (btrim(bpmn_xml)<>''),
 allow_withdraw boolean NOT NULL DEFAULT false,
 created_by uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (app_id,flow_id,version),
 FOREIGN KEY (app_id,flow_id) REFERENCES applications.workflow_definitions(app_id,id) ON DELETE RESTRICT
);

CREATE TABLE applications.workflow_deployments (
 app_id uuid NOT NULL,
 flow_id uuid NOT NULL,
 version bigint NOT NULL,
 deployment_id text NOT NULL CHECK (length(deployment_id) BETWEEN 1 AND 200 AND btrim(deployment_id)<>''),
 confirmed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (app_id,flow_id,version),
 FOREIGN KEY (app_id,flow_id,version) REFERENCES applications.workflow_versions(app_id,flow_id,version) ON DELETE RESTRICT
);

CREATE TABLE applications.workflow_instances (
 id uuid PRIMARY KEY,
 app_id uuid NOT NULL,
 flow_id uuid NOT NULL,
 table_id uuid NOT NULL,
 view_id uuid NOT NULL,
 record_id uuid NOT NULL,
 initiator_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 definition_version bigint NOT NULL,
 state varchar(16) NOT NULL CHECK (state IN ('starting','active','completed','rejected','withdrawn','no_effect')),
 sequence bigint NOT NULL DEFAULT 0 CHECK (sequence BETWEEN 0 AND 9007199254740991),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (app_id,flow_id,definition_version) REFERENCES applications.workflow_versions(app_id,flow_id,version) ON DELETE RESTRICT,
 FOREIGN KEY (app_id,flow_id,table_id,view_id) REFERENCES applications.workflow_definitions(app_id,id,table_id,view_id) ON DELETE RESTRICT
);

CREATE INDEX ix_workflow_instances_drain ON applications.workflow_instances(app_id,flow_id,id)
 WHERE state IN ('starting','active');
CREATE INDEX ix_workflow_instances_schema ON applications.workflow_instances(app_id,table_id,definition_version)
 WHERE state IN ('starting','active');

-- No catalog table is granted to PUBLIC here. roles.sql installs the explicit
-- runtime/backup capabilities after migration, including on a fresh cluster.
REVOKE ALL ON applications.workflow_definitions,applications.workflow_versions,
 applications.workflow_deployments,applications.workflow_instances FROM PUBLIC;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
 LOCK TABLE applications.workflow_definitions IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE applications.workflow_versions IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE applications.workflow_deployments IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE applications.workflow_instances IN ACCESS EXCLUSIVE MODE;
 IF EXISTS (SELECT 1 FROM applications.workflow_definitions LIMIT 1)
 OR EXISTS (SELECT 1 FROM applications.workflow_versions LIMIT 1)
 OR EXISTS (SELECT 1 FROM applications.workflow_deployments LIMIT 1)
 OR EXISTS (SELECT 1 FROM applications.workflow_instances LIMIT 1) THEN
  RAISE EXCEPTION 'workflow catalog contains durable state; Down is refused' USING ERRCODE='55000';
 END IF;
 DROP TABLE applications.workflow_instances;
 DROP TABLE applications.workflow_deployments;
 DROP TABLE applications.workflow_versions;
 DROP TABLE applications.workflow_definitions;
END $$;
-- +goose StatementEnd
