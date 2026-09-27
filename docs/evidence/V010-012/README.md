# Responsive repair evidence · 2026-09-27

User reported a 2560×1440 window with the page occupying only its top-left portion. The existing manual browser configuration explicitly emulated a 1360×920 viewport. Live inspection confirmed innerWidth=1360, innerHeight=920 and site=1360×920. Direct CDP override clearing did not repair that context; those attempts are not reported as success. Reopening the same persistent profile with `manual-browser.json` (`viewport:null`, TLS verification retained) fixed the actual window. No credentials/Cookies were read or included.

Actual natural-window resize verification: outer1440×900 → inner1424×805, site1409 wide (15px vertical scrollbar); maximized outer2560×1440 → inner2560×1351, site2560 wide. `page.viewportSize()` returned null. `server-natural-viewport.png` shows the existing server artifact filling the window and centered. This browser repair does not deploy the new CSS.

## Independent Figma oracle and genuine RED

Reread high-fidelity design context and screenshots for [Login13:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=13-2) and [Register40:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=40-2), including the `Content Area / Responsive` 12-column grid, 32px gutters and four-column centered card. Independent examples: 1920→598.667, 2560→812, 1440→438.667. Existing accepted compact/mobile adaptations remain; desktop grid applies from1280px, a host layout breakpoint retaining readable compact layouts below that size. No claim that Figma specifies this numeric breakpoint.

Before changing CSS, command `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'Figma responsive'` ran on Windows, Playwright1.63.0/Chromium. Exit1, 2PASS/4RED: both routes rendered599px at2560 (expected812) and1440 (expected438.667). Assertions loaded and reached the width checks;1920 baseline passed. Immutable preimplementation commit7d4fcd3, permanent `responsive-red.spec.ts`, `style-before.css`, `red.txt`; SHA256 manifest preserves LF Git bytes. Original ignored `.log` output is copied verbatim to tracked `.txt`, not reconstructed.

Minimal implementation: desktop card width `calc((100% - 64px) / 3)` follows the four-of-twelve-column span. Header/background, center alignment, field/button heights, authentication and compact layout stay unchanged. `register-2560.png`, `login-1440.png`, `login-mobile.png` are real local screenshots, visually inspected against the source and existing approved compact layout. These are local CSS verification; the server screenshot uses the prior artifact.

## Verification

- Full Chromium component suite:35/35, exit0, `green.txt`. Includes12 new responsive cases and23 existing account/password/loading/error/CSRF tests. Mocked API boundaries in those component tests do not establish full product acceptance.
- `pnpm typecheck`, `pnpm build`, `node scripts/check-tasks.mjs`, `node scripts/verify-repo.mjs`, `git diff --check`:exit0.
- Governance/foundation:57/57, exit0. Structural checks are not source/TDD/business approval.
- Firefox/WebKit responsive checks:24/24,exit0, `cross-browser.txt`. Initial scratch runner launched Vite from the wrong cwd;`cross-browser-environment-failure.txt` preserves it. Not counted as a behavioral RED. Saved runner is executed by copying it back to `.work/responsive.config.ts`; its path reflects this workstation.
- Asset metadata/local sizes inspected: all14 WaveOS SVG imports nonempty. Visible login assets all loaded,62×46 brand,1260×1260 glow,24×24 field/visibility/social icons,22×22 check. Register uses62×46 brand,1260×1260 glow,24×24 fields/invitation, matching its unchanged source callsites/slots. Source assets/callsites are unchanged; no screenshot/temporary Figma URL used as an asset.

Final PR head CI and full real product checks are queried separately. No merge, server deployment, Notion approval-state change or component-library change in this repair. Manual server browser remains open with normal window sizing and its existing SSH/TLS settings.

## Manual browser recovery

Use the saved config for manual viewing, with the previously established profile and URL:

```powershell
npx --package @playwright/cli playwright-cli -s=weaveos-trusted open https://weave.hubujiu.site:19443/register --browser=chrome --headed --profile=D:/Workspace/WeaveOS-worktrees/V010-011/.work/trusted-browser-profile --config=D:/Workspace/WeaveOS-worktrees/V010-012/docs/evidence/V010-012/manual-browser.json
```

Close an already running same session before applying a new launch config; do not run `resize`/device emulation on the manual user window. Automated test contexts retain explicit test sizes.
