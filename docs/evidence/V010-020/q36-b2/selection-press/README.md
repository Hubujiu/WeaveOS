# Q36 selection press interruption

Scope: PR21 current-page checkbox regression after `cf0f13152261f5dbab089cda24df2496e067ba0c`. The integration owner approved a consuming CSS rule for the existing decorative checkbox SVG. Selection state, query contracts, vendor sources, animation, accessibility and backend remain unchanged.

## Cause and independent RED

The failed [CI run](https://github.com/Hubujiu/WeaveOS/actions/runs/36933878149) trace shows the filtered POST completed at 370345.600ms and the target row existed with `aria-busy=false` before the checkbox action. The disappearing indeterminate SVG was still under the pointer when the press began. This is not a loading/empty-table failure. `CI-timeline.json` resolves Playwright snapshot back references; the original trace and screenshot are retained.

The first independent WebKit probe and two repeats recorded trusted `pointerdown` on `path`, followed by SVG removal and trusted `pointerup` on `BUTTON`, without a click. The checked oracle failed. Original unmodified and observation-only cases each passed 30 repeats; those diagnostic passes do not prove the race is absent.

Commit `92d2903` retained the initial probes before production CSS changed. The first formal three-engine failures require a qualification: subsequent CSS exploration passed Chromium/WebKit but failed Firefox three times. Firefox's filter dialog still intercepted hit testing during its exit (`elementFromPoint` returned HTML, checkbox event list was empty). Its initial failure is not valid evidence of the checkbox defect. `initial-green-failed.txt` and `initial-event-green-failed.txt` retain those failures.

The corrected stimulus waits for the real filter dialog to hide, checks the filtered current page, then uses a trusted keyboard Space to deselect it. A real mouse press begins while the existing selection SVG remains and ends after its actual DOM removal. No animation/state mock, sleep, timeout increase, retry or skip is used. The observer retains the final pointer press, excluding setup clicks. The earlier observer-count failure and exploration are retained separately.

With the consuming CSS removed again, the corrected independent probe failed twice in each engine: **6 failures**, all recording trusted `pointerdown` on `svg`, trusted `pointerup` on `BUTTON`, and **no click**. See `corrected-event-red.txt` and the immutable test source. The loading/empty-page case already passed in all engines before the fix; it covers genuine mouse clicks on disabled controls and ensures selection does not reappear after restoring rows or changing page.

## Verification

The minimal consuming rule is `.personnel-source-scope [role="checkbox"] svg[aria-hidden="true"] { pointer-events:none; }`. It keeps hit testing on the stable button while preserving the decorative exit animation and native focus/keyboard behavior. Corrected RED commit `589da4f` retained the formal **3 failures / 3 loading-empty passes** before restoring this rule. The original hidden-selection test keeps its business oracles and adds explicit row-ready preconditions; those preconditions are not presented as the defect's cause or fix.

The same three cases across Chromium, Firefox and WebKit, repeated three times, passed **27/27**, without retries or skips. Full B1/B2 completed **117 cases: 108 passed, 9 existing capability-conditioned skips, 0 failures** (7.2m). No new case skips; the unchanged native/fallback motion, focus/pixel restoration, table interactions, drafts, query-context and hidden-selection oracles passed. Root typecheck and web build passed; the build retains the existing >500kB chunk warning. Foundation/governance passed **132/132** and pure filter checks **4/4**. Repository/task structure checks also passed. The actual final committed HEAD must pass the existing fixed Gitleaks source-archive test before authorized ordinary fast-forward publication; that scan's exact HEAD and result are returned separately to the owner.

The unchanged independent press stimulus then passed **3/3**, recording trusted `pointerdown`, `pointerup` and exactly one `click`, all targeting `BUTTON` in each engine. `trusted-event-difference.json` is derived from the retained raw RED/GREEN logs and includes source hashes; the corrected public test source is byte-identical after normalized LF. The GREEN trace records the successful checked state and the later reset/hidden-member oracles. The `*-green.png` files are raw test-end screenshots and can retain an in-flight decorative/dialog exit; they are not claimed as settled visual comparisons.

These are real Playwright browser interactions with synthetic REST fixtures, not a new full-backend/storage acceptance run. Pointer-only CSS changes neither paint nor geometry; existing Q36 production recordings remain historical evidence. This round retains actual browser traces/screenshots and trusted event observations for the newly isolated press regression. Public trace request/response authorization, cookie and set-cookie header count was independently checked as zero before publication.

The owner confirmed historical head `cf0f131`'s [product run](https://github.com/Hubujiu/WeaveOS/actions/runs/36933878180) succeeded. Independent API re-read confirms the same head and successful real product, runtime/security, restore/rollback/browsers, migrations and accepted OCI packaging steps. The final `Complete check-release gate including user signoff` step was skipped; `product-history.json` retains exact conclusions. This does not establish a future head's CI status or user acceptance. Final integration/acceptance belongs to the root owner; no merge or deployment is performed here.

Commands from the repository root, using the installed Playwright 1.63.0 and a separate current-source Vite server/cache at port 43123:

```powershell
$env:WEAVEOS_COMPONENT_PORT='43123'
# Corrected RED, consuming SVG pointer rule absent:
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts --grep 'trusted press spanning|trusted select-all clicks'
# GREEN after the minimal consuming rule:
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts --grep 'select-current-page cannot resurrect|trusted press spanning|trusted select-all clicks' --repeat-each 3
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts 'q36-front-|q36-b2\.component'
```
