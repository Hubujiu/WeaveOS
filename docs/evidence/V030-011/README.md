# V030-011 B5a local backend delivery

Current root P1/P2 repair is documented in [repair-review.md](repair-review.md),
[repair-final-verification.json](repair-final-verification.json) and
[repair-cost-observations.md](repair-cost-observations.md). Actual successful
COMMIT plus PgError now stays UNCONFIRMED; set SQL removes List/member N+1.
Final race: 498 passes / 233 top-level, zero fail/skip. Vet/build, 222 governance/
foundation, 33 contracts, 6 migration/upgrade/restore and OpenAPI lint pass.
The original rejected delivery is superseded locally, pending root acceptance.
See the task document for full frozen B5a scope and the unchanged input SHAs.

## Historical integration checkpoint

The following records the earlier checkpoint; its contract gates were later
released by root in the B5a.1–10 readback. It does not describe current blockers.

This checkpoint completes fixed-baseline integration and a reusable transaction
access boundary. It is **not completed B5 application storage/HTTP**. Root released
code after PRD 01:54:30.776Z / ADR009 01:54:36.067Z readback. Explicit draft
contracts in B5.4/6/8 remain reserved to root and are reported in the task.

Integrated base is 08e8a97abc222ab3fd7fd99461d972b613950fa2, with all four fixed
inputs and all three normal merge parents recorded in integration-manifest.json.
No conflicts occurred. All old hot/cold migration bytes, runtime roles,
compatibility manifest and upgrade hash pin remain identical to fixed PR21.
Hot 00006 / cold 00003 remain absent and reserved; no schema was invented.

Independent B5 tests exercise trusted current database facts and dependency locks
in PostgreSQL 18.6. `AccessForWrite` consumes the caller's pgx.Tx, never falls
back to a pool or Session authorization cache, ignores forged Bootstrap flags,
rechecks account/auth_version, returns direct/template sources and observes a
committed revoke in the next transaction. Existing AuthorizeWrite delegates but
continues to enforce personnel.manage. The helper does not create app grants.

Actual chronological evidence:

- access-red: 02:01:54 UTC, exit 1, compiled missing-behavior assertions. Source
  archive retains no-behavior stubs and the original tests.
- access-green: first implementation run, exit 1. A dependency probe incorrectly
  expected a row first inserted in the writer's transaction to be visible to a
  second transaction. Q25 distinguishes unique-key protection for first insert
  from locks on a preexisting row. This fixture defect is not hidden.
- access-corrected-red: 02:03:28 UTC, exit 1, actually replayed the no-behavior
  stub with a committed dependency fixture. It is labeled as a later corrected
  run; original timestamps and assertions remain in the earlier evidence.
- access-green-final: 02:04:12 UTC, exit 0, 7 test/subtest events (4 top-level).
  Corrected test bytes have the same hash as corrected RED.
- integrated-bff-race: `go test -race -p 1 -count=1 -json ./...`, exit 0,
  453 pass events / 200 top-level tests, zero fail/skip, real isolated PG/Redis.
  This includes existing Session/CSRF, personnel locks/revoke/drafts/presets and
  archive role/copy/conflict regression. It does not test future B5 APIs.
- integrated-vet/build: exit 0. No module/dependency changes.
- integrated-governance: 182/182 pass, zero skip.
- repository structure passes. Task-specific validateTask(V030-011) returns [];
  global task checker exit 1 solely because B0 now rejects inherited PR21's
  V010-020-PLAN.md filename. No source was renamed or guard weakened.

The exact commands, UTC times, source hashes, all raw result events and summary
counts are committed here. RED sources are durable tar archives. Test data and
credentials are synthetic, private and excluded from Git and delivery archives.

Remaining root contract requests: applications.create code, restricted catalog
registration contract, exact HTTP methods/DTO/errors/idempotency-result query,
actual menu-resource persistence/registration source, and audit event/reason
names. Owner transfer, group delete effects, member search, inherited resources,
records/Flowable/reversal actions stay excluded. Full B5 acceptance remains open.
