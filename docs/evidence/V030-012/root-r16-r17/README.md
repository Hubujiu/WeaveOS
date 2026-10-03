# R16–R17 scoped operation checkpoint

This evidence records separate historical source revisions and does not rewrite their provenance.

## R16 — test correction only

R16 ran against `d3e45c3c639e0533047cf9a38b42a7cc31e56268`. The user-corrected A→B→A test passed 1/1, and the same six-file component suite passed 157/157. Raw outputs are `r16-single-1of1.log` and `r16-joint-157of157.log`; container invocations and pinned image details are in `r16-container-commands.txt`. R16 did not change production code. The earlier R15 checkpoint separately recorded root-boundary tests passing 8/8. `r16-typecheck.log` preserves the static check run from the R16 period.

## R17 — shared scoped operation integration

R17 is based on Root's test commit `c1fbc58ffd4c5103f909a6f03e9e0b74d02da08e`. The implementation changes are limited to the shared packet/hook/type contract plus four approved optional-body reads in the existing create and permission screens. Root-authored tests were not changed.

Initial R17 scoped and root-boundary isolated runs passed 8/8 each (`r17-scoped-8of8.log`, `r17-boundary-8of8.log`). The initial combined run passed 165/165 before the four compatibility reads; its log is retained separately. Initial typecheck identified four optional-body accesses, after which Root authorized only four optional chaining changes in the two consumer files. No validator or test was weakened.

After those four changes, `pnpm typecheck && pnpm build` passed. Vite emitted its ordinary large-chunk advisory; the check log and complete outer container invocation are `r17-typecheck-build.log` and `r17-typecheck-build-outer-command.sh`. The same 165-case suite was rerun on the final five-file tree and passed 165/165 (`r17-joint-final-165of165.log`, with `r17-joint-final-outer-command.sh`). All R17 browser runs used the pinned Playwright 1.63.0 image digest `eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`, `PLAYWRIGHT_BROWSERS_PATH=/ms-playwright`, and distinct fresh output directories.

The complete five-file diff is preserved in `r17-five-file.diff`. The scoped tests exercise the shared hook and recovery at the mocked fetch boundary; real record Shell wiring/API acceptance remains unverified. V030-012 remains `in_progress`; these results do not authorize merge or deployment.
