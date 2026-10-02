# Original PR21 integration checkpoint

The root explicitly authorized the original PR21 update, based on the user's request to unify tables/remove the adapter and the later approval to resume the original frontend work. This checkpoint uses the current isolated `task/V010-020-q36-front-b1` checkout; no other worktree is switched or modified. PR title/body and final CI/Notion/acceptance remain with the root.

At 2026-10-01 19:26 UTC, `git ls-remote`, a targeted fetch and `gh pr view 21` agree: PR21 OPEN; head `task/V010-020-admin-shell` at `09707d90af461b66c281a51666b3ed0f586e4dff`; base main `6ffba4d41cb7ee1c70a862e12a9bd9ca88be77a5`. `git merge-base --is-ancestor 09707d90… 1483bd73…` exits 0. Only the current isolated worktree is registered in this checkout. Both normal-push targets are checked again immediately before publication; an unknown advance or non-fast-forward requires stopping and reporting, with no force or alternate credentials.

Task documentation now records “已实现并本地验证，原 PR 整合和最终 CI 待确认”, preserves Q35/RED/limitations and leaves deliveryState in_progress. The only task scope addition is `.github/workflows/ci.yml`, matching the root-approved four-line continuous-regression wiring. Historical Notion outage is retained; the root reports PRD §13 synchronized to B2 at 19:05 UTC. This worker does not claim a direct latest Notion read or final user acceptance. No PLAN/core decision is rewritten.

Documentation-only integration validation (TDD N/A): 132 governance/foundation tests pass, `verify-repo.mjs` and normal `check-tasks.mjs` pass. The PR-mode scope gate is also run locally using an explicit synthetic event containing the freshly observed original PR identity/base SHA; it checks the actual main...HEAD paths and passes. This is local gate validation, not a claim that remote CI has run. Logs: `integration-governance-green.txt`, `integration-structure-green.txt`, `integration-tasks-green.txt`, `integration-pr-scope-green.txt`.

```powershell
node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs
node scripts/verify-repo.mjs
node scripts/check-tasks.mjs
# For the local PR gate, GITHUB_EVENT_NAME=pull_request and GITHUB_EVENT_PATH
# identifies the private synthetic event populated from the observed live PR.
node --test --test-name-pattern='tracked source archive passes' infra/runtime/security.test.mjs
```

After committing this checkpoint, the repository-pinned official Gitleaks 8.30.1 scans the actual final tracked-HEAD archive. If successful and both remote preconditions still match, a normal atomic push updates the isolated branch and original PR21 branch to the same descendant commit. Final new HEAD, push output and automatically triggered workflow URLs are returned to the root from live GitHub results. This document records the pre-publication checkpoint and does not predeclare successful publication, completed CI, merge or deployment. Frontend/backend remain unchanged from the reviewed implementation; local product tests are not repeated for this documentation-only change. Active Vite/test instances and private recovery files are retained.
