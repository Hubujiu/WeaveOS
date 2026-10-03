# V030-011 WEB-15 observation handoff

Root assigned this B5 cloud executor sole ownership of the WEB-15 block and
nearby private observation helper in tests/acceptance/web.spec.ts, with this
task/evidence subtree, on 2026-10-03. No other task owns that block during
this diagnosis. The original test remains verbatim in baseline-web.spec.ts.

The actual d7f7070 product job 111175246725 failed WEB-15 Firefox while
waiting for the second registration POST response. The original listener was
registered before the click. Its trace/screenshot/video were off; the job
uploaded only .work/acceptance/public and did not retain the private
error-context. The available log cannot show whether the second click submitted,
a request left the browser, or the service received it. Local isolated runs
with fresh synthetic invitations passed: three-engine target3, full ordered
browser117, and Firefox in the exact pinned CI Playwright image1. All successful
controls observed click -> form submit -> real POST -> 201 then 409. These
controls do not determine the remote failure cause. No same-head retry was
performed. See baseline-diagnosis.json and the safe remote excerpt.
The safe excerpt is curated and normalizes trailing log whitespace; the
unaltered private product log is identified by its digest in baseline evidence.

Permitted diagnostic edit: only WEB-15 in tests/acceptance/web.spec.ts, with
real-time sanitized event phase/relative time/HTTP path without query/method/
status and boolean field/element validity. No credential values, request body,
full URL, trace, screenshot or video. Keep original 201/409, alert and no-Session
assertions,30s timeout,retry0 and test order. No product/auth implementation,
proxy, CI gate or release policy edit. A new remote head is required after
scope check and pinned secret scan. New evidence/results are appended here.

The adjacent observer now emits `WEB15_PHASE` lines as events happen. Browser
console data passes a fixed field/value whitelist before stdout; only known
registration/login paths, lifecycle phases, element/field validity booleans,
relative milliseconds, POST pathname/method and response status are retained.
The test's original assertions and order are byte-exact after removing the
observer and its single call (`source-proof.json`). `observer-only.patch` is
the complete executable diff; `event-proof.json` summarizes the local phase
sequence without test inputs. No trace, screenshot, video, request payload,
query or raw form value is retained here.

Final observer bytes passed target3 and complete real browser117 across
Chromium, Firefox and WebKit using fresh synthetic invitations, typecheck,
build, governance/foundation222, contracts33 and repository structure. The
first full run had114 pass and3 WEB-17 setup failures because the command
omitted the local Redis observer. Correcting that command, with no WEB-17 or
product source edit, gave117 pass. `verification.json` identifies commands,
timestamps, private log digests, and the local environment; it does not claim
to resolve the earlier remote WEB-15 timeout. The pinned final-head scan and
new remote CI run must be checked after publication.
`prepublication-scan.json` records the pinned scanner's zero findings and
actual PR26 task-scope pass on the executable/evidence commit. The following
evidence-only commit is scanned again at its exact HEAD before publication.
