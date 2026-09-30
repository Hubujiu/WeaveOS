# V010-019 Auth RED

2026-09-30 Asia/Shanghai，Windows，Playwright1.63 Chromium。

Oracle: current Figma r0mSerkjrPdwJVME658W6a 13:2 and40:2 read via high-fidelity context/screenshots; explicit user visual refresh + Q24, synced Notion before implementation.

Command: pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'Figma responsive|login shell|four large fields'

Exit1, eight expected failures: old header72 vs56; old fields56 vs46. Existing behavior source App.tsx/style.css copied unchanged before implementation. Test files are recoverable RED source, checksums reproducible. No credentials; only synthetic network fixtures.

Desktop sample widths490.667/704/330.667 for1920/2560/1440 from fixed192 content insets and12cols32gutter. Card content-driven height; normal login495. Mobile oracle remains usability, no horizontal overflow and visible submit. No product acceptance claim from mocked component checks.
