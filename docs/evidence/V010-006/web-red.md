# V010-006 frontend RED evidence

2026-09-25 16:38 Asia/Shanghai; Windows PowerShell, Node/pnpm, Playwright Chromium, Vite frontend only. API server not used for these three page checks.

Command: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'FR-004|FR-002: registration|FR-018: four'`

Exit code: 1; 3 failed. FR-004 reached `toHaveValue('synthetic-prefill-value')` and failed because the invitation input was absent. FR-002 and FR-018 timed out while looking for the account/password fields, so they are *not* counted as valid target assertion RED yet. The empty React mount is the observed cause; a broken dependency or network was not involved in FR-004.

Test source SHA-256: `27068D8EB6B83F3CE96CBD12ABDD94BC8EE5A51446CF214A1CB88CFB662B6322`; frontend Playwright config SHA-256: `E4BD2C765467E13ABC355C41762D585890526D6397D3194F6D3531DDDAED93A4`.

Independent oracle: current v0.1.0 PRD FR-002/004/018, approved account case/space decision, and updated Figma registration node `26:439`. The test version in the immediately preceding branch commit is retained for RED→GREEN review.

Additional valid RED observed at 2026-09-25 16:42 Asia/Shanghai with `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'FR-001/011|FR-007: an anonymous'` (exit 1): FR-001/011 heading `登录` absent at `toBeVisible`; FR-007 `/app` remained `/app` at `toHaveURL(/\/login(?:\?|$)/)`.

GREEN at 2026-09-25 16:43 Asia/Shanghai: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'FR-004|FR-002: registration|FR-018: four|FR-001/011|FR-007: an anonymous|password visibility'` exited 0, 6 Chromium tests passed. `pnpm typecheck` exited 0; `pnpm build` exited 0. The anonymous route test was fail-closed while the local BFF was unavailable, so it does not prove a server-issued 401 path. Full stack and remaining interactions are NOT RUN.
# 2026-09-25 17:39 · System failure and network UX RED

Independent oracle: accepted ADR-002 requires infrastructure/dependency errors to remain distinct from unauthenticated sessions; v0.1.0 PRD asks for clear error states on login. Browser-only Playwright mocks only the external API boundary, not the page behavior under test.

Test snapshot: `tests/acceptance/web.spec.ts` SHA-256 `96C23D1961508A542B418532120284B449F83B98481F1A54F22D10FFA68DF6FD` on branch after main merge `378eb02c2cca681446e6910acb54d3cbf6df501e`. Command from repository root: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'API-14: a current-session|FR-001 UX'`. Real Chromium / Vite frontend, API boundary stubbed to 503 or connection failure. Exit 1; both reached their target assertions: 503 current-session request redirected to `/login` instead of remaining on `/app` with system error; aborted login request showed raw browser text `Failed to fetch` instead of a plain-language network error. This is a frontend behavior RED, not a claim of full backend acceptance.

GREEN after the test commit `d54b7df`: the same two tests passed (2/2); `pnpm typecheck` and `pnpm build` exited 0. The page now keeps system dependency failures distinct from unauthorized sessions and maps a failed network request to a readable message. Full HTTP/Redis E2E is still pending.

An expanded frontend-only run initially found the pre-existing FR-007 test relied on an absent BFF connection and had treated any failure as anonymous. That setup was wrong: after the correct system-error behavior, the real network failure stayed on `/app`. The browser-only tests now supply explicit 401 responses at the external API boundary; the real product `tests/acceptance/web.spec.ts` retains its unmocked 401/login tests and the simulated 503/network cases live in `apps/web/src/auth.component.spec.ts`. No required assertion was dropped to turn this green. The isolated Chromium component suite is now 10/10 passed; typecheck and build exited 0. Full product tests still require the real BFF.
