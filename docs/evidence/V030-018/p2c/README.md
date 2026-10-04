# P2c Root catalog tests: effective RED

Root test commit `8d0ab3f521d702b994ce2fa2bb9be5f88181f387` has parent V018 head `4d3d78026b3d451aec4ff002a7f9a5d1b24524b3` and was cherry-picked as `d21d97f`. It adds the 16 Root-authored `TestRootCatalog*` cases, the compile-only `ErrNotReady` catalog declarations, the implementation plan and migration-15 backup fixture entry. No test assertion was changed.

Before/after RED SHA-256 for `services/bff/internal/apprecordservice/root_workflow_catalog_test.go` is `4e8c48e041f6ba2a42e9748f3c36c46bb5a72259497843df08100e43ede6d4b1`; the matching hash files are preserved beside this report.

## Isolated fixture

- PostgreSQL 18.6, ephemeral container bound to a dynamically assigned loopback port; `WEAVEOS_TEST_DATABASE_URL` points to `weaveos_ci_test`.
- Goose migrations 1–14 applied; `infra/runtime/roles.sql` applied afterward. The catalog relations do not exist yet.
- Redis 8.2.10, separate ephemeral loopback-only container; `WEAVEOS_TEST_REDIS_URL` points at database 15.
- Go 1.27.1; `GOCACHE=/tmp/v030018-go-cache`, `GOMODCACHE=/workspace/.weaveos-tools/go-mod`.

## Valid RED

Command: `go test -race -count=1 -run '^TestRootCatalog' ./internal/apprecordservice` from `services/bff`, with the isolated PostgreSQL and Redis URLs above. Exit code 1. All 16 top-level Root catalog cases ran into the compile-only implementation and failed against their behavior assertions; the visible failures are `workflow catalog not ready` where durable catalog behavior was required, and `ErrNotReady` where invalid graph rejection was required. This is the intended missing behavior, not a connection, migration, Redis or compilation failure.

`red-command.txt`, `red-output.txt`, and `red-exit.txt` contain the exact command record, raw output and exit code. A first setup attempt omitted `WEAVEOS_TEST_REDIS_URL` and failed during fixture construction; it is retained as `setup-attempt-* (raw output compressed as `setup-attempt-output.txt.gz`)` and explicitly does not count as RED.

The next authorized step is implementation of the frozen P2c catalog and migration 15 only. The Root tests, migrations 1–14, and out-of-scope BFF/UI files remain untouched.
