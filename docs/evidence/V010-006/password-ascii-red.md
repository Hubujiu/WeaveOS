# Q13 · frontend printable ASCII and special classes

2026-09-26 user answer, official PRD FR-018 and Figma Register 40:2 native annotation require only printable ASCII U+0020–U+007E, four classes A–Z/a–z/0–9/visible ASCII punctuation. Space is allowed but contributes no special class; password is preserved. No extra length floor.

Immutable test commit 1c6cfa4; permanent test/App snapshots: password-ascii-red-test.ts/password-ascii-before.tsx.

Command: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep Q13`

Actual RED exit 1: 2 failed, 1 passed. `Aa1 ` strength was 4 instead of 3. `Aa1!中` was submitted to API and displayed API error instead of rejecting non-ASCII input before request. Printable spaces already preserved (direct GREEN, no invented RED).

Minimal implementation shares four ASCII class expressions for feedback and submission, rejects non-printable/non-ASCII registration input, preserves password bytes. Complete component/type/build regression recorded in task document after actual completion. These component tests mock only the external API; they are not full product verification.

Actual GREEN: complete Chromium component suite 23/23 (49.1s), pnpm typecheck and pnpm build all exit 0. Final product E2E remains pending.
