# Stable source compatibility GREEN before final accessibility/base extensions

Actual `pnpm exec playwright test --config .work/q35.config.ts`: 139/139 Chromium,6.3m, exit0.
Actual `pnpm exec playwright test --config .work/q35-cross.config.ts --grep 'Q3[12345]|Figma.*activity|R3.*time|R3 activity pagination'`: 195/195 Chromium/Firefox/WebKit,11.9m, exit0.
Actual targeted compatibility selector from q35/compatibility-red failures: 8/8,22.1s, exit0.

The unchanged product source includes internal-menu-scroll and activity toolbar recovery. Runs discovered their139/195 tests before the independent keyboard/theme files were added; those three new assertions are NOT included in these GREEN counts. No corresponding source was mutated during these runs. The new keyboard/base-style RED archives remain separate; final extensions require their own GREEN and regressions.

Root typecheck/build,132 governance and structural checks pass. Build reports source-motion chunk653KB/204KB gzip; warning preserved and threshold unchanged. Official-registry production audit reports no known vulnerabilities; default mirror endpoint is unavailable and its exit1 is retained as NOT VERIFIED. Tracked b57fdb3 HEAD archive fixed Gitleaks check passes1/1. One WebKit ResizeObserver undelivered-notification message occurred on compact resize; same run's actual bounded overflow/fill/resize assertions all passed, no persistent growth observed. Frontend fixtures are not real BFF/PostgreSQL/Redis verification; corresponding final-head GitHub product acceptance remains required.
