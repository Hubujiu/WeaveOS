-- +goose Up
-- Immutable approval evidence. Business rows remain in their typed tables.
CREATE TABLE applications.workflow_evidence_blobs (
 app_id uuid NOT NULL REFERENCES applications.apps(id) ON DELETE RESTRICT,
 field_id uuid NOT NULL CHECK(field_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 content_hash bytea NOT NULL CHECK(octet_length(content_hash)=32 AND content_hash<>decode(repeat('00',32),'hex')),
 body bytea NOT NULL CHECK(octet_length(body) BETWEEN 10 AND 4194304 AND substring(body FROM 1 FOR 8)=decode('57564645464c0001','hex')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(app_id,field_id,content_hash)
);

CREATE TABLE applications.workflow_evidence_documents (
 app_id uuid NOT NULL,
 evidence_hash bytea NOT NULL CHECK(octet_length(evidence_hash)=32 AND evidence_hash<>decode(repeat('00',32),'hex')),
 table_id uuid NOT NULL,
 view_id uuid NOT NULL,
 record_id uuid NOT NULL CHECK(record_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 created_by uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
 schema_version bigint NOT NULL CHECK(schema_version BETWEEN 1 AND 9007199254740991),
 record_version bigint NOT NULL CHECK(record_version BETWEEN 1 AND 9007199254740991),
 field_count integer NOT NULL CHECK(field_count BETWEEN 0 AND 1600),
 body bytea NOT NULL CHECK(octet_length(body) BETWEEN 10 AND 1048576 AND substring(body FROM 1 FOR 8)=decode('575646454d460001','hex')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(app_id,evidence_hash),
 FOREIGN KEY(app_id,table_id) REFERENCES applications.logical_tables(app_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(app_id,table_id,view_id) REFERENCES applications.form_views(app_id,table_id,id) ON DELETE RESTRICT
);
CREATE INDEX ix_workflow_evidence_record ON applications.workflow_evidence_documents(app_id,table_id,record_id,created_at,evidence_hash);

CREATE TABLE applications.workflow_evidence_members (
 app_id uuid NOT NULL,
 evidence_hash bytea NOT NULL,
 field_id uuid NOT NULL,
 content_hash bytea NOT NULL,
 PRIMARY KEY(app_id,evidence_hash,field_id),
 FOREIGN KEY(app_id,evidence_hash) REFERENCES applications.workflow_evidence_documents(app_id,evidence_hash) ON DELETE RESTRICT,
 FOREIGN KEY(app_id,field_id,content_hash) REFERENCES applications.workflow_evidence_blobs(app_id,field_id,content_hash) ON DELETE RESTRICT
);
REVOKE ALL ON applications.workflow_evidence_blobs,applications.workflow_evidence_documents,applications.workflow_evidence_members FROM PUBLIC;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 -- Store readers and writers look up the document before touching its blobs.
 LOCK TABLE applications.workflow_evidence_documents,applications.workflow_evidence_blobs,applications.workflow_evidence_members IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM applications.workflow_evidence_blobs LIMIT 1)
 OR EXISTS(SELECT 1 FROM applications.workflow_evidence_documents LIMIT 1)
 OR EXISTS(SELECT 1 FROM applications.workflow_evidence_members LIMIT 1) THEN
  RAISE EXCEPTION 'workflow evidence contains immutable history; Down is refused' USING ERRCODE='55000';
 END IF;
 DROP TABLE applications.workflow_evidence_members;
 DROP TABLE applications.workflow_evidence_documents;
 DROP TABLE applications.workflow_evidence_blobs;
END $$;
-- +goose StatementEnd
