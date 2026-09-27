-- +goose Up
CREATE SCHEMA archive;
REVOKE ALL ON SCHEMA archive FROM PUBLIC;
CREATE TABLE archive.authentication_events (
 id uuid CONSTRAINT pk_archive_events PRIMARY KEY,
 event_type varchar(32) NOT NULL CHECK (event_type IN ('register','login','logout','invitation_created','password_reset','session_invalid','account_status_changed','bootstrap_created')),
 outcome varchar(16) NOT NULL CHECK (outcome IN ('success','failure','error')),
 actor_user_id uuid,
 subject_user_id uuid,
 account_fingerprint varchar(81) CHECK (account_fingerprint IS NULL OR account_fingerprint ~ '^[A-Za-z0-9_-]{1,16}:[0-9a-f]{64}$'),
 client_ip inet,
 user_agent varchar(2048),
 session_ref uuid,
 reason_code varchar(63) CHECK (reason_code IS NULL OR reason_code ~ '^[A-Z][A-Z0-9_]{0,62}$'),
 request_id varchar(128) NOT NULL CHECK (char_length(request_id) BETWEEN 1 AND 128),
 occurred_at timestamptz NOT NULL
);
REVOKE ALL ON archive.authentication_events FROM PUBLIC;
-- No destructive Down: cold audit history is durable data.
