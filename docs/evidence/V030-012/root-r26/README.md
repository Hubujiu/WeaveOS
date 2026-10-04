# R26 long form names and action hit testing

Source checkpoint: Root authored tests/plan at `b8a7cef30b67e9d866b4a3e289ffb59bfb1f7400` (parent `d1f26dfa690673f51571865943fc427314d8fd8f`). ADR §18 limits the implementation to the forms tree and its existing visual surface.

## Change

- Constrained both structure grid children and each tree list/item/row so nested rows stay inside the tree panel.
- Kept the full directory, table, and form name text in the DOM; name spans ellipsize visually and expose the full value through the title and tree item label.
- Shortened the visible open/configure action labels to `打开` and `配置`. Their full form-specific `aria-label` values and click handlers remain intact.
- Preserved the existing two-panel structure and the detail panel's pointer behavior.

## TDD and browser results

- RED: the three Root R26 component cases ran in Chromium before the source changes. Both long nested-name hit-test cases failed the panel-bounds/center-hit assertion at 1280 and 1920; the money rounding selection case passed. Full raw output: `red/r26-red-chromium.log`. Sanitized Playwright error contexts are in `red/contexts/`.
- GREEN: the complete `applications.form-shell.component.spec.ts` suite ran unchanged: 11 cases each in Chromium, Firefox, and WebKit, 33/33 passed. The two long-name cases assert all three buttons fit inside the tree and receive normal center-point input; each also opens the designer by normal click. The rounding case asserts a unique control and preserved selected values.
- The existing config also matched three other component suites when no file selector was supplied. That accidental 222-test run was stopped; it is retained as `green/r26-overbroad-run-stopped.log` and is not counted as verification. The final command selects only the configured form-shell file.
- Six actual full-page screenshots are in `green/form-shell-final-output/`: both viewport sizes in Chromium, Firefox, and WebKit. The 1280/WebKit and 1920/Firefox images were visually inspected.
- A preceding 33/33 run used the same source except that it clipped row overflow. The final run was repeated after removing row-level clipping so keyboard focus outlines remain available. The earlier raw log is retained; only the final six screenshots are included, and only the final run is the acceptance result.

All browser runs used the existing configuration `docs/evidence/V030-012/playwright.config.ts`, the repository's pinned Playwright image `mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`, and the existing installed dependencies under `/tmp/V012-R23-followup-deps`. No test or Playwright configuration was changed.

## Other checks

- Root and web TypeScript checks and Vite production build passed using the pinned container. Vite emitted the existing large-chunk advisory (1,019.85 kB JS); build completed successfully.
- `node scripts/verify-repo.mjs`: passed.
- `node scripts/check-tasks.mjs`: passed.
- `git diff --check`: passed.
- No real API/full-stack runner, main merge, deployment, or test edits were made.

See the `container-command.sh` files for exact browser/typecheck/build invocations. Logs are sanitized runner output: ANSI controls and trailing whitespace are removed without changing substantive output. They contain only synthetic component fixture data. `manifest.sha256` records hashes for all listed evidence files (the manifest does not hash itself).
