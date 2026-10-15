# Final-validation WebKit toolchain repair

Actual 4882213c CI is green, including the configured-workflow restore test (0.10s) and all 39 Go packages. Full product is NOT green. The three complete product attempts each passed 143/144 browser cases; their original logs remain alongside this file. A targeted product-only retry also failed before tests because component bundles are deliberately bound to the same run attempt. The full producer/consumer chain was subsequently retried without weakening that binding.

The last two browser failures involve the unchanged B5 application-access journey. Third attempt: a new session request had no response before the original five-second assertion deadline. Fourth: browser cleanup exceeded the original 30-second test budget, with GLib object and libsoup session-feature assertions. First attempt failed a different navigation with WebKit internal error. These are failures, not accepted passes. No more blind reruns of 1.63.0.

## Independently verified upstream evidence

- https://github.com/microsoft/playwright/issues/42803 explicitly identifies Playwright 1.63.0 / WebKit2359 with bundled libsoup3.6.5 use-after-free. Maintainer comments link the upstream fix and state inclusion in 1.64.
- https://github.com/WebKit/WebKit/pull/74619 merged 2026-09-23: libsoup3.6.6 fixes double finishing/unqueue and stale session-feature references.
- https://github.com/microsoft/playwright/releases/tag/v1.64.1 is the official stable release (2026-10-14). This patch also fixes two 1.64 regressions. No prerelease or foreign binary.
- Actual registry @playwright/test1.64.1 metadata and installed frozen dependencies agree; only test/playwright/playwright-core changed in lockfile, all three exact versions/integrities.
- Official digest-addressed MCR OCI index bytes SHA256=7852dc5a7d72f5fe458a597d690f6c42fbc8e9e26ddfa95f8e6ef552558d9bbf. Tag retrieval with Docker schema conversion yields different bytes; only the digest-addressed OCI bytes are the identity proof. Index includes Linux amd64/arm64 manifests.
- Installed browsers.json exactly matches official v1.64.1 source: Chromium1248, Firefox1555, WebKit2370.

The native symptoms match the upstream defect; no project core dump was captured, so this remains strong diagnostic evidence, not independent proof of the exact failing instruction. Successful reruns alone would not prove that either.

## Scope and TDD

ADR V030-075 additive maintenance plan actually written/read 2026-10-15 20:00UTC. No product rule, source permission, migration, deployment, main, or approval-property change. Original tool configuration and all four logs preserved before changes. Seven independent pin/alignment checks reached actual assertions and failed on old package/image before implementation; pin-red.tap is original. Same tests pass after exact official version/digest update. Existing runner-security assertions remain; only independently sourced image expectation changes.

All actual container entrypoints, package/lock, and current runtime documentation move together. No test skip, additional retry, worker expansion, timeout increase, unpinned browser, LD_PRELOAD workaround, or fake backend. The original business tests are byte-for-byte unchanged. Pure config checks do not prove browser stability; exact final real CI/full product/runtime acceptance remains mandatory before develop integration. Local browser/PG execution restrictions remain honored.

## Subsequent real fixture error

0e746831 component shard1 ran all234 assertions successfully but failed with an unhandled route callback reading browserContext.cookies after the test had ended (applications.component.spec.ts336/338). It is NOT green. Original native log and original spec preserved before the fix. Only file-local afterEach now calls page.unrouteAll with behavior wait while the fixture context is still alive; it neither catches errors nor uses ignoreErrors. This follows https://playwright.dev/docs/api/class-page#page-unroute-all. Original tests, cookie-derived actor, assertions and budgets remain. Pure2298/typecheck pass; actual three-browser fixture verification remains pending.
