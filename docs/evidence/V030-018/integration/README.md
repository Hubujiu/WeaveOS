# V030-019 into V030-018 integration verification

Inputs: V030-018 `4d3d78026b3d451aec4ff002a7f9a5d1b24524b3`; V030-019 `b7ebd9534f01db76929183b19176f98e4dc8228d`. The three-way merge preview and `git merge --no-commit --no-ff` completed without conflicts. V019 source, test files, task metadata and evidence are retained unchanged. V018 task metadata now allows all four V019 paths and the explicitly authorized P2c paths.

## Go checks

Executed from `services/bff` with Go 1.27.1. The isolated PostgreSQL 18.6 fixture had migrations 1–14 and `infra/runtime/roles.sql`; Redis 8.2.10 was a separate loopback-only test container.

| Check | Result | Captured output |
| --- | --- | --- |
| `go test -race -count=1 ./internal/flowcommands` | exit 0 | `flowcommands-race-output.txt` |
| `go vet ./internal/flowcommands` | exit 0 | `flowcommands-vet-output.txt` |
| `go test -race -count=1 ./internal/flowgraph` | exit 0 | `flowgraph-race-output.txt` |
| `go vet ./internal/flowgraph` | exit 0 | `flowgraph-vet-output.txt` |
| `go test -run '^TestRootExportEngineCompatibilityFixtures$' ./internal/flowgraph` | exit 0 | `flowgraph-export-output.txt` |

The compiler export was directed to `/tmp/v030018-engine-fixtures`; both generated BPMN files were byte-identical to the committed V019 Java fixtures. Their SHA-256 values are in `flowgraph-export-hashes.txt`.

## Flowable proof

`bash prototypes/flowable-local-tx/prepare-build.sh` exited 0. `bash prototypes/flowable-local-tx/run-proof.sh verify` then ran the real Flowable 8.0.0/PostgreSQL 18.6 proof in its isolated Docker network and exited 0: 24 tests, 0 failures, 0 errors, 0 skipped (15 `LocalCommandExecutorTest`, 9 `GeneratedGraphTest`). Captured Maven output and exit values are `flowable-prepare-output.txt.gz`, `flowable-prepare-exit.txt`, `flowable-proof-output.txt.gz`, and `flowable-proof-exit.txt`. Maven's raw console output contains trailing-space-only `[INFO]` lines; output is gzip-preserved so repository diff checks remain clean.

This validates graph compilation and the existing local engine transaction proof only. It does not claim product RPC/HTTP, caller authorization, cross-service atomicity, UI integration or deployment. Full relevant Go backend regressions are rerun after the P2c implementation.

`node scripts/check-tasks.mjs`, `node scripts/verify-repo.mjs`, and `git diff --check` plus `git diff --cached --check` all exited 0. Their captured output/exit values are in the sibling `check-tasks-*`, `verify-repo-*`, and `diff-check-*` files.
