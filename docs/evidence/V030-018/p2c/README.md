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

## Performance gate: effective RED and GREEN

Root added `services/bff/internal/apprecordservice/root_workflow_catalog_scale_test.go` in commit `13d85cde1ba34cb24f03b623cecdfb3fadc3176`. Its SHA-256 remained `5fa0b6be82c72624c9d07526b25b553d2eb48c90ce7b942b658eb90f110ba12b` before RED, after RED, and after GREEN.

On the migrated isolated PostgreSQL 18.6 fixture, the unchanged production query first failed the actual-plan bound: for 4,999 retired immutable versions and one live version, PostgreSQL sequentially scanned all 5,000 `workflow_versions` rows, filtered 4,999, and the test measured aggregate plan work 10,004 against a limit of 256. `scale-command.txt`, `scale-output.txt`, and `scale-exit.txt` preserve this valid RED.

The query now UNIONs keys for current enabled/closing versions and starting/active instance versions, then joins those keys to `workflow_versions` by its primary key. Migration 15 adds `ix_workflow_definitions_scope(app_id,table_id,id)` and adds `flow_id` to the partial instance compatibility index prefix. The Root test was not edited. The same test now passes; its logged actual plan has aggregate work 8 for 4,999 retired plus one live version (about 3.52 ms). Full 17-case catalog GREEN, final-source backend race, vet, backup, upgrade, and fresh migration/down/up outputs are recorded in adjacent `root-catalog-green-*`, `final-*`, and `migration-final-*` files. The final migration SHA-256 is `5056a36d4f9ed64fdd98432f231ec67012cde693439ac75139e595b51f0911fc` and is pinned in the compatibility manifest.

The first scale-test retry against a new database was a fixture setup error because the role SQL path was container-local and `auth_app` lacked schema USAGE. `scale-runner-setup-note.txt` labels that failure as transcribed/non-RED; roles.sql was subsequently piped into psql, its privilege was verified, and the retry passed. The actual RED evidence is the preceding performance failure, not this setup issue.

P2c remains limited to the durable workflow catalog and its schema compatibility query. Manager/session authorization, appstructure Save/preflight, and HTTP are the next independently tested P2d segment; passing this catalog gate does not claim those are implemented or accepted.
