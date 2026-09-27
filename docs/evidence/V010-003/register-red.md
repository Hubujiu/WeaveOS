# Registration storage RED

2026-09-25 16:56 Asia/Shanghai; isolated PostgreSQL 18.4 on 127.0.0.1:55483 after Goose migration version 1, Go 1.27.1, pgx/v5 5.7.6. No production data.

Independent oracle: current Notion PRD FR-002/003, approved account decision (`Alice` and `alice` distinct, trim edge ordinary spaces, reject internal spaces), reviewed PostgreSQL DDL and transaction specification. Tests cover one-time invitation consumption, account conflict rollback, case-sensitive uniqueness and account validation.

Command in `services/bff`, with `WEAVEOS_TEST_DATABASE_URL=postgres://weave_test@127.0.0.1:55483/weaveos_v010003?sslmode=disable`: `go test ./internal/persistence -run TestRegister -count=1 -v`.

Exit code: 1. Both tests connected, seeded their own synthetic invitations and reached the target call: `Register` returned `storage not implemented` instead of registering `Alice`. The no-behavior stub is in the same RED commit. A prior run without the required database URL failed at environment setup and is **not** counted as RED.

`register_test.go` SHA-256: `579C0D53647E91A7C47DF5F07F8ED15616DE3AB17B2B284D7C419A69552890A7`; no-behavior `register.go` SHA-256: `EE3A6C07BB473D44950AE29B4C6DC8AF417CCCC624671D43CA25F0D9973538CA`.
