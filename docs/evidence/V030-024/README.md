# V030-024 exact integration evidence

Root contract82485dd is based on backend24d07a57de1cd9246e368af6bff365e07be8105d. Merge69c8fbf834e1e8cae2b5f92618f2a5102796efe2 has parents82485ddf94cced2d795cb8c8d5504639d577d98e and frontend3900bcaf5cfb465f0fa5e6e4d9170d1feaa5361d. Main6ffba4d41cb7ee1c70a862e12a9bd9ca88be77a5 remains an ancestor and untouched. Common input ancestor is db475e0dc00e2ac7c18fc5023cbc9039300711ac. The inputs contain403 different commits after main; metadata/evidence commits are additional, not new business work.

Only docs/tasks/index.md conflicted. Both original V022/V023 paragraphs remain verbatim; V024 was added. `blob-preservation.json` verifies all146 frontend and819 backend changed paths, excluding index, against exact original Git blobs. Every other inherited blob matches the backend plus exact frontend delta outside authorized V024 metadata/evidence. CI, Root tests/assertions, fixtures, migrations, roles and old proof are unchanged. Imported raw evidence whitespace remains unchanged. TDD:N/A for pure merge/metadata; no new tests or feature implementation.

## Completed local unchanged validation

Executed on merge69c8fbf's production/test/config tree (later candidate changes only V024 metadata/evidence/index link):

- `node --test contracts/*.test.mjs tests/governance/*.test.mjs tests/foundation/*.test.mjs tests/acceptance/topology.test.mjs`:294/294; repository/task structure checks pass.
- Pinned Playwright v1.63.0 noble digest eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27, pnpm10.28.2, original browser/config defaults: typecheck/build/OpenAPI lint pass (12 existing warnings), forms234/234 across Chromium/Firefox/WebKit; full component499 total,498 passed and1 existing WebKit-only skip under Chromium. No browser isolation/host validation overrides or modified timeout/skip. `default-offline-forms-build.txt`, exit0.
- Python gate12/12; locked Java prepare and `run-tests.sh clean verify`:18/18, zero failure/error/skip; exact manifest CLI gate pass. Actual dedicated internal-network PG/no published ports confirmed in logs. Java suite18.94s, Maven22.182s; concurrent proof load means these are not production performance figures. Original Surefire XML under `surefire-deploy/`.
- Unchanged `prototypes/flowable-local-tx/run-proof.sh clean verify`:24/24 (15 transaction+9 graphs), zero failure/error/skip. Maven23.267s. Original XML under `surefire-proof/`. This real regression is separate from the new registry CI job.
- Existing full `runAcceptance` pipeline, using only the historical local dependency adapter described below: complete Go race `-p1 -count1`, vet/build, hot1-16/cold1-5 migrations, contracts/type/build/components, real HTTPS ingress/API/browsers/faults/storage. Final exit0. Public result: API124, browser138, components498, faults3, storage controls8 and cases19; actual PG18+Redis8.2; elapsed1303.415s. `product-final-prepared.txt`, `.exit`, `product-local-result.json`. No fake HTTP responses in the real product phase.

## Preserved failures and limits

All original failed runs remain; none is relabeled as behavior RED or success:

- `product-local`: Go toolchain TLS CA failure before tests.
- `product-cache-retry`: Go package metadata TLS failure before tests.
- `product-environment-prepared`, `product-local-recovery`: PG init/OCI startup fails with ENOSPC; no valid behavior RED.
- `product-final-local`: complete Go validation passes, then npm offline install metadata is missing before Node tests.
- `product-complete-local`: Docker cannot allocate Go container (ENOSPC), before tests.
- `default-forms-build`: npm TLS dependency bootstrap failure, before browser tests.
- `runtime-contracts-local`:24 total,17 pass/7 fail; absent historical85c2ee1 object, disk/layer allocation errors and scanner startup/no-valid-result failures. The historical Git object was later fetched. No valid completed local security/runtime result is claimed. Full local runtime/OCI restore/rollback, deployment migration/package and optional verify_release are **NOT VERIFIED locally**.
- Initial host dependency preparation accidentally used fallback pnpm11; no tracked blob changed. Its logs are retained as preparation only. Actual claimed browser/product runs used pnpm10.28.2 and the fixed original Playwright image.

`local-environment-runner.mjs.txt` archives the historical dependency adapter: every original command/test ran, Go received a read-only existing public CA/SDK/cache, npm/pnpm used prefilled locked caches; no TLS checking or sandbox was disabled. It is archived evidence, not a default CI runner or permission to change trust settings. Local Docker28.4/vfs on32GiB caused repeated deep-copy capacity failures. Only identified stopped task fixtures/unused caches were cleared; private attempt configuration/logs and selected synthetic volumes remain, original heads/worktrees/source remain. No global prune or daemon/security setting changes.

Root's subsequent explicit instruction stops further local CA/network/security debugging and authorizes the unchanged standard GitHub runner's full product/runtime/migration/package checks as the formal joint verification source. Draft PR35 targets task/V030-022-flowable-deployment. Final exact candidate CI is pending at freeze; no existing input-head green result substitutes for it. Ordinary PR product success does not prove optional verify_release/user-signoff ran. No main merge/deployment, production credentials/privileges, new gRPC or publishing behavior. Root owns final differential/result acceptance.
