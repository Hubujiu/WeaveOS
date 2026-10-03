# V030-012 verification, 2026-10-03 UTC

Source branch: `task/V030-012-original-app-shell`, based on verified PR26 head
`9b89e8e928aedf30df235492fe89bc52021f6fe9`. Scope excludes PR26/PR21/main.
All tests below exercised actual React/Vite code. HTTP boundary fixtures are
identified separately from the isolated real B5 service.

- First TDD stage: four app component tests reached React and failed for missing
  entry/catalog/create/workspace behavior. Exact source and output are in `red/`.
- Create/recovery/geometry/notice/unknown-close corrections each preserve the
  preceding failed assertion and test source under matching `*-red/` folders.
- Final core component behavior: Chromium, Firefox, WebKit and their reduced
  motion projects, `90 passed`. Command: `pnpm exec playwright test --config
  docs/evidence/V030-012/playwright.config.ts --reporter line` inside the pinned
  Playwright1.63 container. `behavior-green90.txt` retains output.
- Loading, error, retry, 401, access denial and unknown-write states: `24
  passed` across the same six projects; `states-green24.txt`. After the final
  unknown-close wording change, `12 passed` across six projects in
  `targeted-last-six.txt`; focused Chromium `4 passed` for final screenshots.
- Isolated live service: PostgreSQL18.6, Redis8.2.10, BFF built from this
  branch using Go1.26, TLS Nginx ingress and `auth_app` member database login.
  Synthetic Bootstrap administrator POST-created a durable app (HTTP201),
  GET list/detail/access reopened it, and an ungranted member saw an empty
  catalogue and direct-link 403. Six browser/reduced projects passed; see
  `real-api-green6.txt`. A final Chromium production-build run also passed.
  Test credentials exist only in ignored mode0600 `.work/v030012/fixtures.json`;
  no login screenshots, trace or video were saved.
- `pnpm typecheck`, `pnpm build`, `pnpm check:tasks`, and `git diff --check`
  passed. Vite's existing >500kB chunk warning persists; no dependency or
  framework change was made.
- Inherited full component suite ran 233 tests: 229 passed, 3 failed, 1
  skipped. Two failed because the Home header's historical `.home-header`
  selector was absent; that selector now exists and both failures were
  rerun successfully (`legacy-home-green.txt`). The remaining R2 AC02 test
  mocks the new `GET /api/v1/applications` as `{}`. B5 requires `{items: []}`;
  the product correctly reports malformed data. This older personnel test
  fixture is outside V030-012's owned paths and awaits its owner. Exact first
  run is `inherited-regression.txt`.
- `screenshots/manifest.json` records 13 inspected product/state screenshots,
  including all current entry flow views, create, empty workspace, denied,
  loading/error and unresolved operation. `assets.json` records exact unchanged
  SVG export bytes/dimensions/source nodes; all six hashes were rechecked.

The earlier Library ZIP helper returned a transfer failure twice. Direct
Figma contexts and screenshots supplied the original source. No ZIP bytes were
claimed or used. Permission-group/member/root-menu UI still needs the
candidate-member endpoint and approved original-native composition from root.
