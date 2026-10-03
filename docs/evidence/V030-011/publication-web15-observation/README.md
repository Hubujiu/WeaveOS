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

Permitted diagnostic edit: only WEB-15 in tests/acceptance/web.spec.ts, with
real-time sanitized event phase/relative time/HTTP path without query/method/
status and boolean field/element validity. No credential values, request body,
full URL, trace, screenshot or video. Keep original 201/409, alert and no-Session
assertions,30s timeout,retry0 and test order. No product/auth implementation,
proxy, CI gate or release policy edit. A new remote head is required after
scope check and pinned secret scan. New evidence/results are appended here.
