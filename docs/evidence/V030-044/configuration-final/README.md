# V030-044 versioned configuration slice — 2026-10-09

## Scope and oracle
Frozen V044 PRD/ADR/P2c plan and user message Sentinel_889b2d8e2a348191b56697658ed957c5. Configuration is immutable version metadata; omitted updates inherit candidate, explicit [] clears only the new candidate, legacy versions remain empty. Supported events: manual/record.created/record.updated, table scope; real changed data + post-write condition matching (dispatch not implemented in this slice).

Tests were committed before implementation (927858e, 81be936, 3ae49d8, b48e753, 6348a04, 5c136ea). Source snapshots and observed behavior-missing RED are preserved in the configuration-*-red directories. Initial JSON byte-order comparison was corrected to semantic comparison before implementation and RED rerun; no product expected behavior was relaxed.

## Verification
- Fresh PostgreSQL 18.6 UTF8 and Redis 8.2.10; Go 1.27.2, race, serial packages, count=1. Final regression: 380 top-level and 188 subtests pass, zero fail/skip. Includes actual HTTPS session/CSRF definition requests, real transactions, publish/enable field rechecks, immutable roles and guarded Down.
- Self-review found publication snapshot swallowed storage errors as incompatibility. A real PostgreSQL transactional missing-column fixture failed first, then the error was propagated; final full regression includes that fixed test.
- Node contracts/foundation/governance: 512 pass, zero fail/skip. Local Node 24.19.0 differs from repository 24.14.0 pin; pnpm 10.28.2, frozen lockfile, no dependency changes.
- go vet (five changed/related packages), task structure, repository structure and git diff --check passed.
- OpenAPI lint passed with 12 warnings; not claimed warning-free.
- Restricted auth_backup pg_dump/pg_restore preserved all 10 version configurations (4 empty, 6 configured), identical row SHA256, UPDATE denied. This is logical restore verification, not the Docker/encrypted-backup test. The first fixture had been emptied by persistence cleanup; zero rows were explicitly rejected and retained as an invalid fixture attempt, then a nonempty configuration fixture was verified.

## Boundaries
No public start endpoint, record Create/Edit trigger hooks, durable start intent dispatch or real Flowable end-to-end delivery in this slice. No current remote CI claim, PR merge, main or production deployment. Full V044 remains in progress.

- Gitleaks worktree scan: ~88.84 MB, zero leaks.
