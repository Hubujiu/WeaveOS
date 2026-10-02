# B0 V030 governance compatibility

Authorized source: PRD/ADR009 B0 PLAN edited 2026-10-02 13:39:52.010Z /
13:39:58.459Z, read after root explicitly released this independent task. The
whole PRD/ADR statuses remain pending/proposed; B0 authorizes no product changes.
Main `6ffba4d41cb7ee1c70a862e12a9bd9ca88be77a5` and read-only PR21
`74cc5824ee4336dd76c72874f0cc4cf38fe9e889` were verified. No V020 tasks/support or
existing V010-021 task were found. B1's branch remains unchanged at `72b52e1…`.

Four executable touchpoints:

- `task-policy.mjs`: canonical typed V010/V030 IDs, exact branch/filename identity
  helpers and dependency validation; no dynamic regex from untrusted IDs.
- `check-tasks.mjs`: validate supported task documents, reject malformed or
  unsupported version-looking names, check actual complete PR/task identity and
  unchanged own-document/allowedPaths rules using the actual Git diff.
- `task.mjs`: same ID check before any Git/gh invocation, then original gates.
- `cleanup-remote-task.mjs`: same complete versioned branch identity before the
  original remote acceptance, squash, tip and conditional-deletion checks.

Only current AGENTS/workflow/task-template/PR-template path wording is updated.
Historical task IDs and original 132 tests are retained. `evaluateAcceptance`,
`evaluateCleanup`, `remoteCleanupDecision` and the workflow YAMLs are unchanged.

Authentic test sequence:

1. Before implementation, the 49-test B0 suite returned exit 1 with 10 target
   failures. `red-run.json` and `red.stdout.json` contain the timestamp/command/output;
   `*.red.txt` and `red-sha256.json` preserve the actual original source/tests.
   The earlier 46-test metadata/source snapshots remain under `.first`, with output
   in `red-first.stdout.json`; three further malformed ID
   cases were added before implementation and the 49-test RED rerun was committed.
2. First GREEN passed 48/49. The start fixture incorrectly used a ready task;
   the existing restart guard correctly rejected it. The fixture now uses planned
   and an additional independent test asserts ready still cannot restart. The
   original failed GREEN artifacts are retained, not hidden or called a pass.
3. Corrected tests were replayed against old source snapshots. This is labeled
   REPLAY in `corrected-red-replay-run.json`, not original RED evidence. Final
   current-source GREEN passes all 50 tests, exit 0.
4. The original baseline test-file list passes 132/132. Current combined
   governance/foundation passes 182/182; zero skips/cancellations. Repository
   structure/task checks and diff whitespace checks pass. Exact commands and
   timestamps are in `verification-runs.json`.
   Raw failure output has trailing spaces emitted by Node. The four affected
   logs are stored as `*.stdout.json` with exact `rawOutput` and its byte SHA256;
   JSON decoding reproduces the original bytes unchanged. Original raw text is
   also retained in earlier commits. Source/docs and the final full diff pass
   whitespace checking; no evidence lines or assertions were stripped.
5. B1's original task metadata passes the patched validator without modifying
   B1. B0's tracked changed paths are inside its enumerated owned scope; see
   `scope-and-b1-validation.json`. Root still coordinates business PR bases/shared
   patch ownership: unrelated shared changes remain outside their allowedPaths.

PR-scope cases run the actual checker against real temporary Git repositories.
CLI/cleanup cases run the real scripts/policy with external Git/gh spies, verify
malformed IDs make no calls, exact remote task paths and exact SHA-bound deletion
arguments, and prove unmerged/cross-version cases never attempt deletion. They
do not demonstrate live cleanup or call a real remote. Synthetic fixtures only;
temporary fixture trees are removed by tests.

Expected parser cost is linear in branch/file length (ID has fixed length);
document/dependency/path validation retains existing linear scans and PR scope
retains its changed-path × allowed-path scan plus Git cost. No new dependency,
latency quota, workflow permission, approval/acceptance or production capability.

NOT RUN: final-head GitHub Actions or draft-PR review, actual start --apply,
remote/local accepted-task cleanup, product DB/API/browser tests, merge or deploy.
No B0 implementation blocker remains; root owns review and coordinated
integration/consumption. No claim of main acceptance or production readiness.
