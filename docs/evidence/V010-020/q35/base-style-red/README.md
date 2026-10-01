# Original base style / dark variant RED

Independent oracle: official c0319d8 index.css custom dark variant at line6, @layer base body bg-background/text-foreground/antialiased and default border/outline, and user Q35 direct original-effect request. Existing light workspace remains light; scoped source controls must not inherit the blue workspace text or apply system-dark utilities absent a .dark class.

Two actual Chromium commands on independent4176, Windows / Node22.23.1 / pnpm10.28.2 / Playwright1.63, both exit1:

- `pnpm exec playwright test --config .work/q35-font.config.ts apps/web/src/personnel-arca-keyboard.component.spec.ts --project chromium --grep 'original base'`: scope root color rgb(16,32,68), required original oklch(.145 0 0).
- `pnpm exec playwright test --config .work/q35-font.config.ts apps/web/src/personnel-arca-theme.component.spec.ts --project chromium`: light page1 white passes; OS dark absent any .dark changes original page button to oklab(.922 0 0 / .3), required white.

Both reach target assertions and are observed before any corresponding CSS implementation. ZIP preserves current tests, unpatched source styles/config and raw logs. Smoothing is asserted only on engines that expose the original -webkit-font-smoothing property; color and scope assertions run everywhere. No dark product theme is added.
