# V044 durable start admission checkpoint

Not a complete backend or complete V044 delivery. Manual entry qualification is still a pending product question; rework/review/deletion/history/template work remains.

## Evidence and ordering
- The previously committed public-trigger RED (2487111 / 25ee76a) reached starting != active after real HTTPS, Java deployment and record Create.
- 7871a6c adds compile-only start declarations and four admission tests; all four actually fail on missing admission. 51b2bfa fixes the not-yet-reached query to use the existing JSON command protocol field; corrected RED remains four genuine failures. preimplementation-source.patch reconstructs these sources from a022a2e.
- a175920 adds the actual host test before host injection; public-trigger-host-red fails at the starting state, not setup.
- 7b1a12a expands independent CI identity/selection tests before gate/runner changes: 2 failures + 1 error. Afterwards all 11 gate tests pass.
- Supplemental permission/fault/latest-record/recovery coverage was added after initial implementation. It is not claimed as an original preimplementation RED.

raw-evidence.tar.gz losslessly retains all 17 new run directories. Each original byte count and SHA256 is in archive-manifest.json; every tar member was read back and checked. Includes commands, source heads, uncommitted source snapshots, exit codes and original timestamped JSONL. Directory names containing green are intended run names, NOT verdicts: inspect tests.exit and logs.

## Intermediate failures retained
1. First implementation tried a direct typed-table row lock denied to auth_app; fixed by using the existing appstructure owner capability, with no role expansion.
2. Persisted non-condition nodes carry JSON null; normalized consistently with the existing catalog decoder before graph validation.
3. The old record-only fixture lacks canonical defaults and text maxLength, invalid for strict audit encoding. Reused the existing capture fixture; did not weaken the codec or expected behavior. Attempts with only defaults fixed are retained.
4. Revocation fixture originally tried deleting grants before restricted FK children; fixed fixture ordering only.
5. First full regression found due start commands from the new test fixtures interfering with the global scheduler test. Reused existing rootTaskKeepDispatchPrivate cleanup, preserving commands and all assertions; final full run passes.

## Verified scope
- Final six-package race: 501 top-level + 227 subcases PASS, no failure/skip.
- Five native-loopback genuine Java/Flowable HTTP cases PASS, including original-command lookup after dropping a genuine successful Execute reply and a fresh worker, plus automatic configured host admission. This is not a claim of OS-level BFF restart in the new start case.
- Native harness authenticates RPC using the real ServiceTokenInterceptor and synthetic test-only identity. It retains strict loopback and separate engine schema. Existing Docker transport fixture remains isolated with unpublished ports and gains real database health RPC; its complete old/new eight-case run is still required from CI.
- Node 512 PASS, gate 11 PASS, go vet and Maven test-compile pass, source whitespace/scope checks pass, Gitleaks finds zero leaks.
- No main merge, deployment, new credentials or database-role expansion. No old migrations edited.
