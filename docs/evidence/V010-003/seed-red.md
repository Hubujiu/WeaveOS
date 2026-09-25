# Bootstrap Seed RED

2026-09-25 17:00 Asia/Shanghai. Source: v0.1.0 PRD Bootstrap Admin initialization and current Notion DDL/transaction specification. Isolated PostgreSQL 18.4 database after migration version 1. Synthetic hash strings are test data, not usable credentials.

Command in `services/bff` with the isolated `WEAVEOS_TEST_DATABASE_URL`: `go test ./internal/persistence -run TestBootstrapSeed -count=1 -v`.

Exit code: 1. Both tests loaded and reached `SeedBootstrap`; the no-behavior stub returned `bootstrap seed not implemented`. Required outcomes: first seed creates the Bootstrap Admin; rerun preserves password hash, Disabled status and auth_version; same-name ordinary account is never promoted.

Test SHA-256: `025C23E8F5B1673CE9E777D5974C60A0E4F4046BA38E49E788347B18B4E4663B`; stub SHA-256: `F52C68024E383AE215472D2139D7B47122D4420CD166923122273C6FF0C4445F`. The branch RED commit preserves both files before implementation.
