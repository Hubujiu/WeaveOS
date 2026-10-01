# Q35 final local delivery verification

All commands on Windows / Node22.23.1 / pnpm10.28.2 / Playwright1.63.0, with isolated Vite servers and synthetic API fixtures:

- `pnpm exec playwright test --config .work/q35.config.ts`: 143/143 Chromium,5.9m, exit0.
- `pnpm exec playwright test --config .work/q35-cross.config.ts apps/web/src/personnel-arca.component.spec.ts apps/web/src/personnel-arca-keyboard.component.spec.ts apps/web/src/personnel-arca-theme.component.spec.ts apps/web/src/personnel-arca-focus.component.spec.ts`: final54/54 Chromium/Firefox/WebKit,3.2m, exit0.
- Initial54 ran53/54; the sole Firefox focus expected Chromium's UA color. Actual official Firefox155.0 native Tab read proved source :-moz-focusring/outline:auto takes original currentColor. Only that source-grounded engine oracle changed; product bytes did not. Initial log and independent source proof are retained. The143 Chromium run's oracle remains exactly the same; final-head CI runs the corrected whole test suite.
- `pnpm typecheck`, `pnpm build`,132 foundation/governance and structural checks: exit0. Typecheck repeated after the Firefox-only test oracle correction. Production dependency audit against official npm registry reports no known vulnerabilities (baseline log retained).

Final source/test/config/lock exact bytes and SHA remain in final-source-tests-config.zip and sha256.json. Raw original logs retain line endings/whitespace; readable logs are normalized. Source copies remain22 official modules; vendor.patch is the complete necessary diff, and style-original-bytes/style-consumption record original stylesheet inputs. All actual RED stages remain separate and precede their corresponding source changes in pushed commits.

Final review window4173/PID56120 uses synthetic fixtures, not real BFF/PostgreSQL/Redis acceptance. Actual1264×805 viewport, top56/side176, body355px, one real member plus7 empty grid rows and zero activity plus8, footer33px at exact body bottom; cards are queried from the DOM. Geist+Noto are scoped, outer Noto and original assets retained. Finite CSS tab animation finishes before metrics/screenshots. Active review remains available because PR21 is OPEN; no task cleanup, merge, approval-state change or deployment is claimed.

The direct original motion closure produces654.55KB JavaScript (204.34KB gzip) and66.84KB CSS (12.78KB gzip); the existing Vite chunk warning remains visible and its threshold is unchanged. The original upstream transitive npm and project pnpm resolutions differ only as documented in source.md; no full-lock identity is claimed.

Fixed tracked-HEAD Gitleaks archive scan is performed after committing this final snapshot, before push. Corresponding five GitHub checks, including real isolated product/storage/E2E/runtime acceptance, are then verified on the actual final PR head. Local component counts never substitute those checks. Task ready means reviewable; acceptance still requires remote main task and merged PR facts.
