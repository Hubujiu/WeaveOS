# P2b formal command ledger and record fence validation

Baseline: task branch `task/V030-018-approval-commands`, initial HEAD `2ca3fa4cda0760b9d26e165409f0e0f678dd929b`. The Root-authored `root_formal_ledger_test.go` and frozen implementation plan were present at that commit. Contract was also read from the last P2b section of the linked Notion ADR.

## Test source and RED

Root's seven tests use `newRecordFixture`, real PostgreSQL 18.6, Redis, the existing record fence functions and `Ledger{Namespace:"applications"}`. The only test-file change was mechanical `gofmt`: the formatted worktree file is byte-for-byte equal to `gofmt` applied to the original file at baseline. Original SHA256: `abfe1c91453d7bd612535bff7c4dc53865f91d5632b93aefbd102a4382393b4e`; normalized SHA256 before/after: `b1c33b4f7043939b9d726154f51fe2c9e55a3b281ccddb79a88b6a9dc0175c7b`. See `root-test-raw-baseline.sha256`, `root-test-gofmt-equivalence.sha256`, and the empty `root-test-gofmt-check.diff`.

On a disposable PostgreSQL 18.6 cluster with `--network none`, the Root tests were run after goose migrations 1–13 and `roles.sql`, before migration 14. All seven reached behavior assertions and failed on the absent formal ledger tables (`42P01 relation "applications.workflow_commands" does not exist`); process exit code was 1. Raw output and code are `red.log` and `red.exit-code`. This is the intended schema-gap RED, not a compile or environment failure.

## Implementation

- Added `db/migrations/00014_workflow_command_ledger.sql` for the two formal `applications` tables, exact command identity and JSON/type/length/state constraints, FK/dispatch scan index, and a Down guard that rejects either populated table with SQLSTATE `55000`.
- Added only the P2b table and column grants/revokes to `infra/runtime/roles.sql`. `auth_app` can update only `state` and `receipt_json` on commands; dispatch is SELECT/INSERT/DELETE. Backup is SELECT-only; reader, maintenance and PUBLIC receive no table privileges.
- Registered migration 14's SHA256 in `infra/server/deploy/compatibility.json` and updated only the pinned role hash in `infra/server/deploy/personnel-upgrade.mjs`.
- Root's existing `infra/runtime/backup.test.mjs` already contained migration 14 in the baseline; it was not changed.

The initial empty-Down check exposed a goose parser boundary issue (`42601 unterminated dollar-quoted string`). The raw attempt is preserved in `rollback-empty-down14-initial-splitter-defect.log`; adding goose `StatementBegin`/`StatementEnd` fixed it. Final empty Down/Up and populated Down refusal both passed.

## Environment and results

- Final clean PostgreSQL 18.6 cluster was isolated with a dedicated Unix socket and no container network. Before migration, `pg_roles` contained zero `auth_%` roles. Goose applied versions 1–14 first; only then was `roles.sql` applied. A separate archive database received the existing archive migrations and `cold-roles.sql` for appstructure regressions.
- Redis used the already cached official image digest `sha256:164c759a0c342ee69d08fc99219382b0fd682181465c0df2e0e6911f4c85d73c`, bound to loopback port 16379.
- Root's seven formal-ledger tests: GREEN, `go test -count=1 -run '^TestRootFormalLedger' ./internal/apprecordservice`.
- Full apprecordservice race regression (including the existing 14 fence tests): GREEN.
- appstructure race regression: GREEN.
- flowcommands race regression (33 top-level tests): GREEN.
- `node --test infra/runtime/backup.test.mjs`: 4/4 GREEN on a disposable PostgreSQL 18.6 container.
- `WEAVEOS_TEST_GOOSE=/workspace/.weaveos-tools/gopath/bin/goose node --test infra/server/deploy/personnel-upgrade.test.mjs`: 4/4 GREEN, including cold-first migrations and role pin validation.
- Empty ledger: goose Down 14 then Up 14 GREEN. Populated ledger (6 command rows): Down refused with SQLSTATE `55000`; goose version remained 14.
- `roles-assertion.sql`: exact column/table privileges passed against PostgreSQL catalogs.
- `go vet` for apprecordservice, appstructure and flowcommands; `scripts/check-tasks.mjs`; `scripts/verify-repo.mjs`; gofmt and diff checks passed. `hash-verification.log` checks every registered migration hash and the exact roles pin; migrations 1–13 match their pre-existing hashes.

Raw commands, logs, exit codes, and SHA checks are retained alongside this summary. Temporary test clusters used synthetic fixtures only. This package does not add or claim Flowable RPC/HTTP, actor/record authorization policy, terminal approval, or frontend behavior.
