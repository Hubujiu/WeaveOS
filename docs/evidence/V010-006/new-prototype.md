# New WaveOS prototype: source and test evidence

## Source replacement

User instruction on 2026-09-26 replaces the old file with `r0mSerkjrPdwJVME658W6a`. Actual Figma metadata, high-fidelity design context and screenshots were read for [Login 13:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=13-2) and [Register 40:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=40-2), following the Figma design-to-code skill. PRD and login/identity feature records were updated with these links. Approval/online status was not changed.

1920×1080 source measurements: page padding 30×22; header 1860×94, radius 22; card 598.667×720, radius 20, content padding 42×34; field shells 56 high, button 58 high, four strength segments 6 high. Implementation uses native React/CSS under the user's earlier scoped Figma-first approval. No component-library source is modified.

Login's forgot-password, three third-party buttons and remember-account control conflict with PRD scope or lack behavior rules. [Notion Q12](https://app.notion.com/p/3e62f5a9e648814497c8df6bf27c8724) remains unanswered; these controls are not implemented. Core shell/fields are implemented, but full login visual matching is BLOCKED. Q7 still blocks Cookie/CSRF integration. These component tests do not prove full product acceptance.

Source reread timestamps: PRD `2026-09-26T01:14:32.552Z`, login feature `01:28:54.551Z`, identity feature `01:28:55.430Z`; fetched pages reported no truncation/unknown-block warning. Figma tool did not provide a last-modified timestamp. Final local Chromium run after visual corrections again passed 12/12; separate final typecheck, build, task structure, repository structure and diff checks exited 0.

## Genuine RED → GREEN

- Immutable pre-implementation test commit: `ed1d757`. Recoverable test source: `new-prototype-red.spec.ts`, SHA-256 `15157A9D6A31FC6FA7F2B0A8F9578217ECC6C96169E6187E48618D112B04ECE7`.
- RED command: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'WaveOS Figma'`. Two Chromium cases loaded; exit 1. Login reached the brand assertion: expected WaveOS, received DocWeave. Register reached the new subtitle assertion: expected text absent. Executed before changing App/CSS/assets.
- GREEN command: `pnpm exec playwright test --config apps/web/playwright.component.config.ts`. Exit 0, 12/12 Chromium component cases passed. API responses are mocked only in this isolated suite; real BFF/PG/Redis browser E2E remains pending.
- `pnpm typecheck` and `pnpm build` exited 0. The first partial implementation run had one invitation accessible-name failure; fixed by excluding the visual required marker from the input's accessible name, without weakening the test.

## Assets and visual verification

SVGs were downloaded from the exact Figma asset URLs provided by the high-fidelity response. All referenced files are nonempty local imports; no temporary asset URL is in application source and no screenshot is used as an asset. `waveos-asset-geometry.json` records actual loaded/natural dimensions and bounding boxes in Chromium. All images loaded; logo 62×46, error 26×26, field/visibility/invitation icons 24×24, expanded glow 1260×1260. Registration geometry matches the Figma field slots within subpixel rounding. Identical lock/eye hashes across both nodes permit reuse of their login files; separate brand/glow exports are retained for each screen.

| Local asset | Figma slot | Rendered use |
| --- | --- | --- |
| waveos-brand-login.svg / waveos-brand-register.svg | 13:6 / 40:5 | BrandHeader, 62×46 |
| waveos-glow-login.svg / waveos-glow-register.svg | 13:3 / 40:13, expanded blur bounds | AuthLayout, 1260×1260 |
| waveos-user-login.svg / waveos-user-register.svg | 13:29 / 40:30 | AccountField, 24×24 |
| waveos-lock-login.svg | 13:37, identical 40:38 / 40:85 | PasswordField, 24×24 |
| waveos-eye-login.svg | 13:41, identical 40:42 / 40:89 | Visibility button, 24×24 |
| waveos-invitation.svg | 40:97 | Invitation field, 24×24 |
| waveos-error.svg | 13:18 | Dynamic error alert, 26×26 |

`waveos-register.png` was inspected against the Figma screenshot: layout, header, card, fields, icons, strength bars and button positions aligned at 1920×1080. `waveos-login.png` captures a synthetic 401 component response to verify the error icon/core fields; Q12 controls are absent, so login is not marked visually complete. User remains the final visual/risk/recovery acceptance owner. Documentation/link changes: TDD:N/A, verified by rereading source pages and repository checks.
