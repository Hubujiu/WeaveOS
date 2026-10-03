# R18 fixed frontend combination check

## Source and merge

Local integration commit: `8ac485f9bca4056e2b4b571e0ad7b76c6125c51d`.
Parents: V012 `d761fbb0136bb4dedcd092d7fbae132a5612c6a8` and V017
`e49ffe4afa025dda7a155dfbdfa278adfb1c05b3`. Merge base:
`3e84662bf2f44efecceb193ee22eea347a9b74b0`. The merge was conflict-free.
No code or test files were edited during this verification checkpoint.

## Verification

All commands used the pinned `mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`
image (Playwright 1.63.0), `CI=true`, and
`PLAYWRIGHT_BROWSERS_PATH=/ms-playwright`. The full outer Docker invocations
and logs are preserved alongside this README.

- `pnpm typecheck`: passed, exit 0 (`typecheck.log`).
- `pnpm exec playwright test --config playwright.component.config.ts`: 411
  collected, 410 passed, 1 skipped, exit 0 (`components-all-src.log`). This is
  the existing `Q36 WebKit edits nested groups through the safe animated
  fallback` test, conditional on WebKit, while this config runs only Chromium.
  No skip or timeout setting was changed.
- During the component run Vite logged a proxy connection refusal for
  `/api/v1/applications` to `127.0.0.1:8080`. The suite nevertheless completed
  with 410 passed and 1 skipped; the message is retained as environment
  evidence, not omitted or represented as an API success.
- `pnpm build`: passed, exit 0 (`build.log`); Vite transformed 7093 modules and
  emitted the usual chunk-size advisory for the minified bundle.

The component config is Chromium-only and its API behavior is mocked. This
combination does not verify an actual records route, Shell wiring, or a real
records API flow. Those remain incomplete; V030-012 stays `in_progress`.
No merge to main or deployment is authorized.

The successful `tsc --noEmit` invocation is silent, so `typecheck.log` records
the command and observed exit status rather than fabricated compiler output.
