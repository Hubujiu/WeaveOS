# V030-017 Root authored recovery checkpoint (R09/R10)

This is a narrow component behavior checkpoint. It does not establish full V030-017 acceptance or real BFF/PG/Redis integration. Root authored all three test files in commit `7ae207fcbbe75adc9e09c6d2ae065493b14f2c14`; the fixture simulates transport outcomes and mounts the committed `RecordForm` component.

## R09 RED

- Worktree: `/workspace/WeaveOS-worktrees/V030-017-r09-check`, detached at `7ae207fcbbe75adc9e09c6d2ae065493b14f2c14`.
- Working directory: `apps/web`.
- Command: `pnpm exec playwright test --config playwright.component.config.ts src/applications/records/root-recovery.component.spec.ts`.
- Environment: `PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers`, `XDG_DATA_HOME=/tmp/weave-pnpm-data`, `XDG_CACHE_HOME=/tmp/weave-pnpm-cache`.
- Exit code: 1; 3 passed, 1 failed, no skipped tests.
- First target failure: after the first recovery returns `failed`, test line 23 expects `保存结果待确认`, but the status element is absent.
- Raw run: [r09-red.log](r09-red.log).

An initial attempt using dependencies symlinked from another worktree was aborted by pnpm before tests loaded; it is not the RED evidence. Dependencies were then installed from the existing frozen lockfile, with 71 packages reused from `/tmp/weave-pnpm-store` and zero downloaded packages, before the recorded run.

## R10 GREEN

The only implementation change in R10 removes `setUnknownOperation(null)` from `RecordForm.accept`'s `failed` outcome branch. A failed initial write still has no unknown operation and remains editable. A failed recovery query preserves the pending operation ID and keeps the original input and recovery path available.

- Same worktree and exact Playwright command as R09.
- Exit code: 0; 4 passed, no skipped tests.
- Raw run: [r10-green.log](r10-green.log).
- The three root-authored test files were not edited. Their SHA256 values remain:
  - `root-recovery-fixture.html`: `3c34290e93df1d40f0cc79beb6dfee0b38241829105e1aaa8b5d4c3cd049348d`
  - `root-recovery-fixture.tsx`: `45bba3474a7b77b7abdc713607b6d0679d8f5b771129656dd3ee9d07cb4f5753`
  - `root-recovery.component.spec.ts`: `6cdd7808474d6132fd41acf7c57507544fff059a99eda76eb02f50f8d12e8bdb`

## Typecheck boundary

`pnpm typecheck` was run in `apps/web` on both the unchanged `7ae207f` baseline worktree and the R10 worktree. Both exit with code 2 and the same four diagnostics in the already committed `record-form-fixture.tsx`: `authorityKey` is not a `RecordFormProps` property and `_field`, `request`, `_signal` are implicit `any`. The unchanged baseline reproduces these errors, so they predate the one-line R10 diff. No other file was changed to address them, and this checkpoint does not claim a successful full typecheck.

- Baseline raw output: [r10p-baseline-typecheck.log](r10p-baseline-typecheck.log).
- R10 raw output: [r10-typecheck.log](r10-typecheck.log).

The original `/workspace/WeaveOS-worktrees/V030-017` tree remains separate with its pre-existing dirty `RecordForm.tsx` and untracked `apps/web/test-results/`. This checkpoint does not include that work-in-progress.
