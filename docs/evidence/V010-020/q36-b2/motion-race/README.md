# Final CI native reversal repair

Original PR head: `95b4d8e1c7427f691d99d1e811cc6e5e724862b1`.
CI: run 36921604535 / job 110568764724, original quick native reversal failed after the final trusted Escape: the dialog had `data-closed` but `aria-hidden=false` and remained visible past the unchanged 3000ms assertion.

The deterministic regression gates the **real browser** `document.startViewTransition` update callback, sends a trusted keyboard Escape while the opening commit is pending, then releases that pending native commit. No component, business state or native transition is mocked. The pre-fix component failed with the same closed-root/visible-content DOM. Initial source, failed trace/video/pixels and raw output were committed in `4636c79`; independent repeat before implementation failed twice. Exploratory drivers that failed to deliver an input or waited for an uncommitted opening are preserved separately and are not claimed as valid product RED.

An additional 650ms hold exercises expiry of the existing 600ms completion fallback. Its RED is an explicitly labeled **post-fix REPLAY** against archived `QueryFilterPanel-before.txt` loaded by an isolated Vite transform, not an initial RED or an overwritten checkout. Both holds use the same visibility, focus, active-class and ARIA oracle. No timeout was increased and no Firefox skip added.

The repair checks generation and last intent when the queued state updater applies, forces expanded content closed on the latest close, and binds completion callbacks to the generation of their render. Stale completion callbacks cannot borrow a later generation. Native durations remain 300ms opening / 220ms closing; the 600ms fallback and existing WebKit safe fallback remain unchanged. CSS, contracts, backend and dependencies are unchanged.

Verification on the repaired source:

| Check | Result |
| --- | --- |
| Original quick reversal + delayed commit first check, Chromium/Firefox | 4 passed |
| Quick reversal, both delayed holds, exact restored trigger pixels/focus and shell geometry/timings, repeat-each=2 | 48 passed, 12 conditional engine capability skips |
| Complete B1/B2, all three engines | 102 passed, 9 conditional engine capability skips |
| Root typecheck and build | Passed; existing >500kB bundle warning retained |
| Governance/foundation | 132 passed |

The complete suite previously had 105 cases with seven capability skips. The two new native cases per engine bring it to 111 cases: both run in Chromium/Firefox, and the two corresponding WebKit cases use the existing unsupported/unsafe native condition. WebKit fallback/reduced motion cases pass. Logs record actual engines and conditions.

Commands from repository root (pnpm CLI resolved to the installed local pnpm 10.28.2):

```powershell
# RED: old source and original pending callback case, before repair
$env:WEAVEOS_COMPONENT_PORT='43121'
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts --project firefox --grep 'delayed native opening commit' --repeat-each 2
# REPLAY only: archived pre-fix source server with separate optimizer cache
$env:WEAVEOS_COMPONENT_PORT='43122'
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts --project firefox --grep 'after fallback expiry'
# GREEN: independent Vite server/cache serving current source
$env:WEAVEOS_COMPONENT_PORT='43123'
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts --grep 'quick native reversal|delayed native opening commit|close restores trigger pixels|native shell shares' --repeat-each 2
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts 'q36-front-|q36-b2\.component'
```

`record-formal.mjs` records rebuilt production pages on an existing isolated loopback HTTPS/BFF/PostgreSQL18.6/Redis8.2.10 GREEN instance. Credentials are read only from a private ignored fixture and used by API login before opening any recorded page; no credential-bearing page/trace is saved. Unlike the deterministic regression, formal recording does not gate native callbacks. It observes native transitions, sends trusted inputs and applies short CPU busy slices while closing/reopening/closing. It checks exact focused-trigger PNG equality, three 1100ms stable closed holds, focus, actual 300/220ms shell timings, narrow members and desktop events. Final screenshots, real WebM recordings and metrics are in `final/`; source SHA256 in metrics ties these recordings to the repaired component.

The first formal driver stopped after completing Chromium member checks because it used nonexistent `.event-table` instead of the actual `.activity-table` selector. This was a driver error, not product RED; `formal-driver-selector-error.txt` records the cause. The corrected script re-runs both engines completely.

Final PR CI and product acceptance remain the root owner's responsibility; local checks do not claim those complete. Existing review processes, worktrees and credentials are retained.
