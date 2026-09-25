# V010-006 frontend RED evidence

2026-09-25 16:38 Asia/Shanghai; Windows PowerShell, Node/pnpm, Playwright Chromium, Vite frontend only. API server not used for these three page checks.

Command: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'FR-004|FR-002: registration|FR-018: four'`

Exit code: 1; 3 failed. FR-004 reached `toHaveValue('synthetic-prefill-value')` and failed because the invitation input was absent. FR-002 and FR-018 timed out while looking for the account/password fields, so they are *not* counted as valid target assertion RED yet. The empty React mount is the observed cause; a broken dependency or network was not involved in FR-004.

Test source SHA-256: `27068D8EB6B83F3CE96CBD12ABDD94BC8EE5A51446CF214A1CB88CFB662B6322`; frontend Playwright config SHA-256: `E4BD2C765467E13ABC355C41762D585890526D6397D3194F6D3531DDDAED93A4`.

Independent oracle: current v0.1.0 PRD FR-002/004/018, approved account case/space decision, and updated Figma registration node `26:439`. The test version in the immediately preceding branch commit is retained for RED→GREEN review.

Additional valid RED observed at 2026-09-25 16:42 Asia/Shanghai with `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'FR-001/011|FR-007: an anonymous'` (exit 1): FR-001/011 heading `登录` absent at `toBeVisible`; FR-007 `/app` remained `/app` at `toHaveURL(/\/login(?:\?|$)/)`.

GREEN at 2026-09-25 16:43 Asia/Shanghai: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'FR-004|FR-002: registration|FR-018: four|FR-001/011|FR-007: an anonymous|password visibility'` exited 0, 6 Chromium tests passed. `pnpm typecheck` exited 0; `pnpm build` exited 0. The anonymous route test was fail-closed while the local BFF was unavailable, so it does not prove a server-issued 401 path. Full stack and remaining interactions are NOT RUN.
