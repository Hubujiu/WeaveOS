# PostgreSQL schema RED

2026-09-25 16:52 Asia/Shanghai. Approved source: v0.1.0 Notion database design, DDL/transaction specification, ER and field dictionary re-read after the user's 2026-09-25 account/toolchain decisions. Target: four authentication tables on PostgreSQL 18.

Environment: isolated PostgreSQL 18.4 cluster under the Windows user's temporary directory, loopback port 55483, fresh database `weaveos_v010003`; no production data. Go 1.27.1 was downloaded from the official release archive, SHA-256 `A3911B5E0E1B1053F25ED0675F4C1C6AAD1E2BFCF253DF2B9BE4CAABD2EDD95D`. `pgx/v5` 5.7.6 is pinned as the approved major-version driver within the existing `go 1.23` module compatibility boundary.

Command in `services/bff` with `WEAVEOS_TEST_DATABASE_URL=postgres://weave_test@127.0.0.1:55483/weaveos_v010003?sslmode=disable` and Go 1.27.1 on PATH: `go test ./internal/persistence -run TestAuthSchemaMatchesApprovedDictionary -count=1 -v`.

Exit code: 1. The test loaded, connected and reached its domain assertions: `auth.users`, `auth.password_credentials`, `auth.invitations`, and `auth.authentication_events` were all missing. This is the expected RED before the migration exists. Test source SHA-256: `F50024997AA1C604962ADE28315AF5FC1C81A62E459806C8DF8F6C8FED07CFA9`. The RED commit in the task branch retains the exact test and the no-behavior package declaration.

The local PostgreSQL instance runs on Windows; it proves PostgreSQL 18 constraints but does not replace the requested WSL Docker / Linux and CI checks. Docker Desktop's Linux daemon did not respond during this run.

GREEN at 2026-09-25 16:54 Asia/Shanghai: `goose v3.28.0 -dir db/migrations postgres <isolated test URL> up` exited 0, applied `00001_auth.sql` in 21.15 ms and recorded version 1. The same `go test ./internal/persistence -run TestAuthSchemaMatchesApprovedDictionary -count=1 -v` then exited 0. SQL came from the updated, approved Notion DDL; no production database was touched. The migration contains an Up section only because automatic destructive Down would drop real user/audit data.
