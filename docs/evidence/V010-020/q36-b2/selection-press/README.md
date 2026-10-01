# Q36 selection press interruption

Scope: PR21 current-page checkbox regression after `cf0f13152261f5dbab089cda24df2496e067ba0c`. The integration owner approved a consuming CSS rule for the existing decorative checkbox SVG. Selection state, query contracts, vendor sources, animation, accessibility and backend remain unchanged.

## Cause and independent RED

The failed [CI run](https://github.com/Hubujiu/WeaveOS/actions/runs/36933878149) trace shows the filtered POST completed at 370345.600ms and the target row existed with `aria-busy=false` before the checkbox action. The disappearing indeterminate SVG was still under the pointer when the press began. This is not a loading/empty-table failure. `CI-timeline.json` resolves Playwright snapshot back references; the original trace and screenshot are retained.

The first independent WebKit probe and two repeats recorded trusted `pointerdown` on `path`, followed by SVG removal and trusted `pointerup` on `BUTTON`, without a click. The checked oracle failed. Original unmodified and observation-only cases each passed 30 repeats; those diagnostic passes do not prove the race is absent.

Commit `92d2903` retained the initial probes before production CSS changed. The first formal three-engine failures require a qualification: subsequent CSS exploration passed Chromium/WebKit but failed Firefox three times. Firefox's filter dialog still intercepted hit testing during its exit (`elementFromPoint` returned HTML, checkbox event list was empty). Its initial failure is not valid evidence of the checkbox defect. `initial-green-failed.txt` and `initial-event-green-failed.txt` retain those failures.

The corrected stimulus waits for the real filter dialog to hide, checks the filtered current page, then uses a trusted keyboard Space to deselect it. A real mouse press begins while the existing selection SVG remains and ends after its actual DOM removal. No animation/state mock, sleep, timeout increase, retry or skip is used. The observer retains the final pointer press, excluding setup clicks. The earlier observer-count failure and exploration are retained separately.

With the consuming CSS removed again, the corrected independent probe failed twice in each engine: **6 failures**, all recording trusted `pointerdown` on `svg`, trusted `pointerup` on `BUTTON`, and **no click**. See `corrected-event-red.txt` and the immutable test source. The loading/empty-page case already passed in all engines before the fix; it covers genuine mouse clicks on disabled controls and ensures selection does not reappear after restoring rows or changing page.

## Pending verification

The minimal consuming rule is `.personnel-source-scope [role="checkbox"] svg[aria-hidden="true"] { pointer-events:none; }`. It keeps hit testing on the stable button while preserving the decorative exit animation and native focus/keyboard behavior. Final repeated and full regression results will be recorded after execution. The owner separately confirmed all product steps for historical head `cf0f131` in [product run](https://github.com/Hubujiu/WeaveOS/actions/runs/36933878180); that result does not establish a future head's CI status. Final integration/acceptance belongs to the root owner; no merge or deployment is performed here.
