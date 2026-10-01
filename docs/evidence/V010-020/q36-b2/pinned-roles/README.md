# Reviewed Q36 installed role policy pin

Product acceptance run 36927466857, job 110588221955, at `69bdc20d761dffd2898850b15aa36c7d641ba898` failed the fixed-policy unit and real personnel upgrade cases with `Installed personnel role policy differs from reviewed source`. Root confirmed preceding real product/runtime/backup/restore/rollback/browser stages and CI had passed; this evidence covers the newly authorized installed-policy repair only.

The obsolete `a6ac2dbdeabbe508c3cb65d95076a1e440d3c7496da6137e216a9d1f31a21295` matches roles at historical main `6ffba4d`. Independent comparison with current roles shows only approved Q36 differences: personnel schema usage for maintenance, draft SELECT/INSERT/DELETE and UPDATE of payload_json/draft_version/updated_at, revision SELECT for the application, and lock_query_revisions EXECUTE for application/maintenance. Current canonical LF roles SHA256 is `a32bf6618bca1dae92eb3efa78523047d92a3aa8b2f929c5825df7aa393b90b9`. The full diff is retained.

Before changing the implementation pin, the original unit case failed. After adding approved minimum-privilege assertions, the complete existing migration/personnel-upgrade suite on **new isolated PostgreSQL containers** produced two passes/two failures, both from the obsolete role pin. Existing Goose expansion/failed transactional migration/application rollback passed. Source snapshots and complete RED output were committed in `3c20369`.

The production repair changes only the fixed roleHash literal. It still accepts only one reviewed digest after CRLF-to-LF normalization and rejects arbitrary appended GRANT SQL. It does not compute a trusted digest from runtime roles or release-uploaded content, remove validation, change credentials, install on a real server, modify roles.sql, or widen production privileges. The updated module remains administrator-installed.

Complete GREEN: four passed, zero skipped. New real database checks prove application draft CRUD except blanket UPDATE, exactly three approved update columns and immutable ownership/context columns, revision SELECT without direct INSERT/UPDATE/DELETE (actual mutation attempts receive permission denied), maintenance without revision UPDATE, approved application/maintenance lock EXECUTE, and no backup/reader lock EXECUTE. The application actually executes the reviewed lock function in a transaction. Existing cold-before-hot migration, historical auth insert compatibility, preserved identity/user/audit rows, read-only backup, failed transaction ledger and application rollback assertions remain unchanged and pass.

```powershell
$env:WEAVEOS_TEST_GOOSE=(Resolve-Path .work/q36-pinned-roles/tools/goose).Path
node --test infra/server/deploy/migrate.test.mjs infra/server/deploy/personnel-upgrade.test.mjs
node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs
node scripts/verify-repo.mjs
node scripts/check-tasks.mjs
node scripts/check-release.mjs
```

Goose v3.28.0 was installed using the existing official golang1.27.1 image, read-only module cache and a separate build cache/output. RED and GREEN invocations each create fresh no-published-port PostgreSQL containers named by their child process PID; the test removes only its own containers. Existing runtime/review stacks and worktrees are unaffected. Governance/foundation: 132 passed. check-release reports structural recorded evidence completeness, not product/user acceptance.

## Follow-on packaging review requiring root coordination

Search of fixed role hashes found only the active module pin and historical `docs/evidence/V010-019/receiver-review-hashes.json`; the latter is preserved as historical evidence. Current module/roles/receiver canonical LF digests are recorded in `reviewed-pins.json`.

`infra/server/deploy/package.mjs` enumerates every current migration, and `validateEnvelope` calls the unchanged strict `validateCompatibility` approval check. `infra/server/deploy/compatibility.json` still contains only the published 00001/00002 hot/cold migrations, so the actual policy rejects Q36 with `Migration not approved`. `packing-pins-review.json` records that all four published migration digests and compose/nginx configuration digests match; only these approved new migration entries are absent:

| Path | Canonical LF SHA256 |
| --- | --- |
| migrations/00003_query_drafts.sql | 9b1f546d158931d09245070a4d5b6b46dd64b128b008363ae7a44f74bae8860a |
| migrations/00004_query_revision_writers.sql | 85faf7406d0e82ce7ca14aaf10276f80ef5a88a5c957170f111a261e910c925d |

This shared approval mapping was reported to root and not changed under the personnel-upgrade two-file authorization. Root must coordinate its update before packaging can pass. No unknown migration, published hash change or gate relaxation is proposed. Packaging OCI export itself was not re-run here. Final HEAD source archive scan and remote CI status are reported separately against their actual SHA.
