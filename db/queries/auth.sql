-- name: LockInvitation :one
SELECT id, used_by FROM auth.invitations WHERE code_hash = $1 FOR UPDATE;

-- name: CreateUser :one
INSERT INTO auth.users (account) VALUES ($1) RETURNING id, account;

-- name: CreateCredential :exec
INSERT INTO auth.password_credentials (user_id, password_hash) VALUES ($1, $2);

-- name: ConsumeInvitation :one
UPDATE auth.invitations SET used_by = $2, used_at = clock_timestamp()
WHERE id = $1 AND used_by IS NULL RETURNING id;

-- name: AppendRegistrationEvent :exec
INSERT INTO auth.authentication_events
    (event_type, outcome, subject_user_id, client_ip, user_agent, request_id)
VALUES ('register', 'success', $1, $2, $3, $4);

-- name: GetLoginRecord :one
SELECT u.id, u.account, u.status, u.is_bootstrap_admin, u.auth_version,
       c.password_hash
FROM auth.users u JOIN auth.password_credentials c ON c.user_id = u.id
WHERE u.account_key = $1;

-- name: GetCurrentUser :one
SELECT id, account, status, is_bootstrap_admin, auth_version
FROM auth.users WHERE id = $1;
