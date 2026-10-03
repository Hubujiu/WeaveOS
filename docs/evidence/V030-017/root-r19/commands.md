# R19 / R19B command record

All test and build runs used the isolated worktree `/workspace/WeaveOS-worktrees/V030-017-r19-check`. The package lock resolves Playwright 1.63.0; `pnpm exec playwright --version` returned `Version 1.63.0`. The Chromium install was `/workspace/.weaveos-tools/browsers/chromium-1243`.

## Fixture baseline and RED

R19 initial fixture HEAD: `565add0cbf971e994b3031593ca179745bfdf754`.

```bash
set -o pipefail; XDG_DATA_HOME=/tmp/weave-pnpm-data XDG_CACHE_HOME=/tmp/weave-pnpm-cache PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers pnpm exec playwright test --config playwright.component.config.ts --output /tmp/weaveos-r19-output.5WVlEg/pw-output src/applications/records/root-layout.component.spec.ts 2>&1 | tee /tmp/weaveos-r19-output.5WVlEg/execution.log
```

Result: 3 failed, 1 passed. Raw output: [`r19-initial-565-red.log`](r19-initial-565-red.log).

Root's updated fixture/test baseline HEAD: `a0c7141bf70c13f6dbe35d69198de6147979ea11`. No test or fixture source was changed in the implementation worktree after this commit.

```bash
set -o pipefail; XDG_DATA_HOME=/tmp/weave-pnpm-data XDG_CACHE_HOME=/tmp/weave-pnpm-cache PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers pnpm exec playwright test --config playwright.component.config.ts --output /tmp/weaveos-r19-red.2E11ZF/pw-output src/applications/records/root-layout.component.spec.ts 2>&1 | tee /tmp/weaveos-r19-red.2E11ZF/execution.log
```

Result: 4 failed, 0 passed. These were target assertions: missing layout group; an unplaced field remained visible; the read-only view still exposed an unplaced field; the actual record version control was missing. Raw output: [`r19b-a0c-layout-red.log`](r19b-a0c-layout-red.log).

## Post-change verification

Layout tests (4 passed):

```bash
set -o pipefail; XDG_DATA_HOME=/tmp/weave-pnpm-data XDG_CACHE_HOME=/tmp/weave-pnpm-cache PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers pnpm exec playwright test --config playwright.component.config.ts --output /tmp/weaveos-r19-green.f1RKkZ/pw-output src/applications/records/root-layout.component.spec.ts 2>&1 | tee /tmp/weaveos-r19-green.f1RKkZ/layout4.log
```

All records component tests (18 passed):

```bash
set -o pipefail; XDG_DATA_HOME=/tmp/weave-pnpm-data XDG_CACHE_HOME=/tmp/weave-pnpm-cache PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers pnpm exec playwright test --config playwright.component.config.ts --output /tmp/weaveos-r19-full.CQ69cW/pw-output src/applications/records 2>&1 | tee /tmp/weaveos-r19-full.CQ69cW/records18.log
```

Typecheck (passed):

```bash
set -o pipefail; XDG_DATA_HOME=/tmp/weave-pnpm-data XDG_CACHE_HOME=/tmp/weave-pnpm-cache pnpm typecheck 2>&1 | tee /tmp/weaveos-r19-full.CQ69cW/typecheck.log
```

Build (passed, with the existing >500 kB chunk-size warning):

```bash
set -o pipefail; XDG_DATA_HOME=/tmp/weave-pnpm-data XDG_CACHE_HOME=/tmp/weave-pnpm-cache pnpm build 2>&1 | tee /tmp/weaveos-r19-full.CQ69cW/build.log
```

Raw post-change outputs: [`r19b-layout-green.log`](r19b-layout-green.log), [`records-18-green.log`](records-18-green.log), [`typecheck.log`](typecheck.log), and [`build.log`](build.log).

## Root-authored test source SHA-256 at `a0c7141`

| File | SHA-256 |
| --- | --- |
| `root-layout-fixture.html` | `0f26ea928f4472ff2d3bb7fcb76b63bdc7e4fd74e9b2f974be2b28d1608e174f` |
| `root-layout-fixture.tsx` | `657ae018ec3268a57d2bc0c044a81773bdda90da803ee8c3a007f206fc4bb85f` |
| `root-layout.component.spec.ts` | `823902754ace14d6ef9724907998a34ad9ac434ffff798aaf6039ad08a75d1bc` |
