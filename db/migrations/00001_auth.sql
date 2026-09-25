-- +goose Up
CREATE SCHEMA auth;

CREATE TABLE auth.users (
    id uuid CONSTRAINT pk_users PRIMARY KEY
        DEFAULT gen_random_uuid(),
    account varchar(254) NOT NULL,
    account_key text GENERATED ALWAYS AS (account) STORED NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'active',
    is_bootstrap_admin boolean NOT NULL DEFAULT false,
    auth_version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_account_key UNIQUE (account_key),
    CONSTRAINT ck_users_account CHECK (
        char_length(account) BETWEEN 1 AND 254
        AND account = btrim(account)
        AND position(' ' in account) = 0
        AND account !~ '[[:cntrl:]]'
    ),
    CONSTRAINT ck_users_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT ck_users_auth_version CHECK (auth_version > 0)
);

CREATE UNIQUE INDEX uq_users_single_bootstrap
    ON auth.users (is_bootstrap_admin)
    WHERE is_bootstrap_admin;

CREATE TABLE auth.password_credentials (
    user_id uuid CONSTRAINT pk_password_credentials PRIMARY KEY,
    password_hash text NOT NULL,
    password_changed_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_password_credentials_user FOREIGN KEY (user_id)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_password_credentials_hash CHECK (
        char_length(password_hash) BETWEEN 1 AND 1024
    )
);

CREATE TABLE auth.invitations (
    id uuid CONSTRAINT pk_invitations PRIMARY KEY
        DEFAULT gen_random_uuid(),
    code_hash bytea NOT NULL,
    created_by uuid NOT NULL,
    used_by uuid,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_invitations_code_hash UNIQUE (code_hash),
    CONSTRAINT uq_invitations_used_by UNIQUE (used_by),
    CONSTRAINT fk_invitations_creator FOREIGN KEY (created_by)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_invitations_consumer FOREIGN KEY (used_by)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_invitations_hash CHECK (octet_length(code_hash) = 32),
    CONSTRAINT ck_invitations_usage_pair CHECK (
        (used_by IS NULL AND used_at IS NULL)
        OR (used_by IS NOT NULL AND used_at IS NOT NULL)
    ),
    CONSTRAINT ck_invitations_usage_time CHECK (
        used_at IS NULL OR used_at >= created_at
    )
);

CREATE INDEX ix_invitations_created_by
    ON auth.invitations (created_by);

CREATE TABLE auth.authentication_events (
    id uuid CONSTRAINT pk_authentication_events PRIMARY KEY
        DEFAULT gen_random_uuid(),
    event_type varchar(32) NOT NULL,
    outcome varchar(16) NOT NULL,
    actor_user_id uuid,
    subject_user_id uuid,
    account_fingerprint varchar(81),
    client_ip inet,
    user_agent varchar(2048),
    session_ref uuid,
    reason_code varchar(63),
    request_id varchar(128) NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT fk_authentication_events_actor FOREIGN KEY (actor_user_id)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_authentication_events_subject FOREIGN KEY (subject_user_id)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_authentication_events_type CHECK (
        event_type IN (
            'register', 'login', 'logout', 'invitation_created',
            'password_reset', 'session_invalid',
            'account_status_changed', 'bootstrap_created'
        )
    ),
    CONSTRAINT ck_authentication_events_outcome CHECK (
        outcome IN ('success', 'failure', 'error')
    ),
    CONSTRAINT ck_authentication_events_fingerprint CHECK (
        account_fingerprint IS NULL
        OR account_fingerprint ~ '^[A-Za-z0-9_-]{1,16}:[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_authentication_events_request_id CHECK (
        char_length(request_id) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_authentication_events_reason CHECK (
        reason_code IS NULL
        OR reason_code ~ '^[A-Z][A-Z0-9_]{0,62}$'
    )
);

CREATE INDEX ix_authentication_events_time
    ON auth.authentication_events (occurred_at DESC, id DESC);
CREATE INDEX ix_authentication_events_subject_time
    ON auth.authentication_events (subject_user_id, occurred_at DESC)
    WHERE subject_user_id IS NOT NULL;
CREATE INDEX ix_authentication_events_actor_time
    ON auth.authentication_events (actor_user_id, occurred_at DESC)
    WHERE actor_user_id IS NOT NULL;
CREATE INDEX ix_authentication_events_request
    ON auth.authentication_events (request_id);

COMMENT ON TABLE auth.users IS 'v0.1.0 用户认证主档，不包含通用授权模型';
COMMENT ON COLUMN auth.users.account_key IS '大小写敏感的账号唯一登录检索键；Alice 与 alice 不同';
COMMENT ON COLUMN auth.users.auth_version IS '凭据重置或状态变化时递增；Session 保存快照';
COMMENT ON TABLE auth.password_credentials IS '本地密码自描述哈希；不得存明文或可逆密码';
COMMENT ON COLUMN auth.password_credentials.password_hash IS '应用密码库验证格式；长度约束不是原始密码长度策略';
COMMENT ON TABLE auth.invitations IS '无时间过期的一次性邀请码，仅存高熵原文的摘要';
COMMENT ON COLUMN auth.invitations.used_by IS 'NULL=未使用；与 used_at 同时赋值；正常应用不得清空复用';
COMMENT ON TABLE auth.authentication_events IS '只追加的最小认证审计，不是审计中心';
COMMENT ON COLUMN auth.authentication_events.session_ref IS '独立非认证 UUID，不是 Cookie SID，不是 Redis 外键';

-- Initial authentication tables contain user and audit data. Rollback is a reviewed
-- forward repair or isolated database restore, not an automatic destructive Down.

