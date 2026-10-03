# R14/R15 checkpoint evidence

This checkpoint isolates the R14 root-boundary contract tests and the R15 API/recovery implementation. Product changes in the checkpoint are limited to `apps/web/src/applications/api.ts` and `apps/web/src/applications/recovery.ts`.

## Results

- R15 root-boundary component suite: 8/8 passed. Exact command and stdout are in `r15-root8.log`.
- Broader related component suite: 156/157 passed. Exact command and stdout are in `r15-related-components.log`. The remaining failure is `src/applications/forms/forms.component.spec.ts:620:1`, “A to B to A navigation ignores a late first-generation preflight”; the expected save-preflight dialog count was 0 but observed 1 after releasing the first request.
- Static verification: `CI=true pnpm typecheck && pnpm build` completed successfully. TypeScript checks passed and Vite built successfully (7093 modules transformed; standard large-chunk warning). This is recorded from the completed command; its full stdout was not retained as a file.
- No real HTTPS record API acceptance was run for this checkpoint.

## R14 test baseline

R14 used `pnpm exec playwright test --config playwright.component.config.ts src/applications/root-boundary.component.spec.ts` from `apps/web`. It ran 8 cases: 4 passed and 4 failed. Two failures demonstrated behavior REDs: the retained packet was mutable, and an empty DELETE 204 response was decoded as JSON. Two other cases failed because `applicationReadPost` had not yet been exported; those are missing-interface failures, not behavior REDs. The mutation-path scope case appeared to pass because calling the absent export threw `TypeError` before any request, so it was a false-positive and did not demonstrate the intended guard. The other three cases passed. This is a transcribed result summary; initial raw stdout was not retained.

The R14 run and its test source belong to the root-owned test work. This checkpoint does not modify tests or claim that all 8 R14 cases were behavior REDs.

## Task state

V030-012 remains `in_progress`. The broader component suite has one outstanding failure, and real HTTPS record API acceptance remains unverified. This checkpoint does not claim end-to-end completion.
