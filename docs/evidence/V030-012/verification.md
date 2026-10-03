# V030-012 verification, 2026-10-03 UTC

Source branch: `task/V030-012-original-app-shell`, initially based on verified
PR26 head `9b89e8e928aedf30df235492fe89bc52021f6fe9`. Scope excludes
editing PR26/PR21/main.
PR26 branch head `d7f70701db6475b8c8fcef858dbefa8f31797afe`, then the
later head `b4c0fb3927d71a44c9a299c9c0bc241d42663893`, were merged into
only this V030-012 worktree without conflict. Stack diff is limited to V030-012 files
and the authorized single personnel test fixture.
At inspection, PR26's own GitHub `product` job was red on a pre-existing
`tests/acceptance/web.spec.ts` response-wait timeout (116/117 browser cases
passed). Its acceptance-test repair belongs to the separate PR26 owner; this
slice does not modify that file or claim its CI is green.
The separate owner later repaired PR26. All five listed checks at the current
`b4c0fb3` head are now green; this V030-012 branch contains that exact base.
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
- After merging PR26 head `d7f7070`, the production build was served again
  through isolated HTTPS B5. Root POST create201, catalog/detail/access reopen,
  and ungranted member empty catalog/direct-link403 passed in Chromium,
  Firefox, WebKit and reduced-motion counterparts: `6 passed (35.0s)`.
  See `real-api-after-merge.txt`; isolated containers were then stopped.
- `pnpm typecheck`, `pnpm build`, `pnpm check:tasks`, and `git diff --check`
  passed. Vite's existing >500kB chunk warning persists; no dependency or
  framework change was made.
- Inherited full component suite initially ran 233 tests: 229 passed, 3 failed,
  1 skipped. Two Home header compatibility failures were fixed and rerun
  (`legacy-home-green.txt`). The remaining R2 AC02 test was independently
  reproduced RED with a malformed `GET /api/v1/applications` fixture `{}`;
  B5 requires `{items: []}`. Its single fixture response was corrected without
  changing the expected empty-state or denied-admin assertions. Targeted test
  then passed. Exact before source, RED and GREEN logs are in `legacy-fixture/`.
- `screenshots/manifest.json` records 13 inspected product/state screenshots,
  including all current entry flow views, create, empty workspace, denied,
  loading/error and unresolved operation. `assets.json` records exact unchanged
  SVG export bytes/dimensions/source nodes; all six hashes were rechecked.

The earlier Figma handoff Library ZIP helper returned a transfer failure twice.
Direct Figma contexts and screenshots supplied the original source. No bytes
from that ZIP were claimed or used. A separate V030-012 review bundle containing
the initial 13 screenshots and source evidence was saved to ChatGPT Library as
`libfile_0302d213ca6481918f863a8a3a1f82f0`, version 0. The same Library
identity is updated from the final stacked source; the delivery response
records its resulting version. Permission-group/
member/root-menu UI still needs the candidate-member endpoint and approved
original-native composition from root.

## Independent 0f881b1 review repair

The main reviewer did not accept 0f881b1 as final product code. Frozen
V030-012 ADR §6 and V030-013 ADR Appendix B now require actor-bound requests,
strict create confirmation, SPA reauthentication recovery and the server's
negative actor guard. The initial 13 screenshots in Library version 0 are
explicitly a visual checkpoint, not functional acceptance.

- Before new safety edits, the inherited Chromium suite ran `241 passed,
  1 skipped (9.4m)` after the authorized legacy fixture correction. The new
  direct trailing-slash test first failed on the catalog heading, then passed
  after exact route matching (`trailing-slash-red/`). Local Firefox/WebKit
  attempts could not start because runtime libraries are absent; pinned
  Playwright Docker will supply those browsers for final GREEN.
- `actor-recovery-red/cross-account.txt` shows two genuine browser REDs:
  A's unknown POST could be retried as B after another tab changed the cookie,
  and an A create view could start a new B write. The held assertions remained.
- `actor-recovery-red/unknown-reply-and-auth.txt` has RED for 201 `{}`,
  201 error envelope, valid-shape 200 and the lost A operation after 401 and
  same-actor login. The missing-data, malformed-app and different-actor
  controls already passed; these were not mislabeled RED. The repaired focused
  Chromium suite passed 10/10; an additional delayed catalog401 draft restore
  and exact header coverage passed after repair. Full app Chromium suite then
  passed 31/31.
- Current real B5 role matrix passed 6/6 pinned browser/reduced projects,
  using live personnel assignment to create an actual create-only actor and
  live group member/menu writes. It proves owner, Bootstrap, granted member,
  cross-app denial and grant revocation on the isolated server. See
  `real-role-matrix-six.txt`.
- `actor-recovery-red/real-guard-before-backend.txt` is an expected service
  RED: with B Session and A's `X-Expected-Actor-Id`, the pre-Appendix-B BFF
  returned 200 instead of required 409. V030-013 owns that backend fix. This
  frontend sends the exact header on every application-domain request and
  rechecks current Session before writes, but its preflight cannot close the
  inter-request race without the server guard. Final integrated GREEN is
  pending; current BFF status must not be described as complete acceptance.
- Further RED showed a parseable HTTP201 app with another owner, an invalid
  UUID or revision2 was wrongly confirmed; `create-payload-validation.txt`
  records 3/3 failures, followed by focused 3/3 GREEN. A second RED held a
  POST in flight while a late 401 forced SPA login; the module's packet was
  not yet marked unknown after remount. `pending-POST-held.txt` and
  `pending-POST-green.txt` preserve the failing then passing assertion. The
  packet is now conservatively unknown once a POST is allowed to leave the
  tab; a later valid confirmation or definitive denial resolves it.
- Two additional screenshots were visually inspected: the A→B masked account
  mismatch page and same-account recovered unknown operation. The current
  `screenshots/manifest.json` has 15 individually SHA-256 verified images.
- Cross-tab actor fixtures initially read the raw request cookie header, which
  WebKit did not expose through Playwright's route API. Four WebKit cases
  therefore missed the simulated B account even though the browser had
  switched. The fixture now reads the shared BrowserContext cookie jar and
  asserts B is present; focused WebKit normal/reduced rerun passed 4/4.
  This is a test-harness correction, not a weakened product assertion.
- A masked-account action originally hard-reloaded the page, unnecessarily
  discarding the in-memory unknown operation. `masked-switch.txt` records the
  real browser RED. The action now enters the SPA login route; focused GREEN
  passed and the original actor's recovery map remains in the document.
- A parseable HTTP201 JSON `null` receipt still escaped the unknown-outcome
  classifier through a JavaScript property access error; `null-envelope.txt`
  records that RED. Envelope type validation now classifies it as unknown and
  preserves the original operation; focused GREEN is retained alongside it.
- The broad inherited Chromium component suite completed after the major
  actor/recovery edit: `253 passed, 1 skipped (10.8m)`; see
  `full-component-safety.txt`. The six-project focused application sweep had
  `182 passed, 4 failed`, all four confined to WebKit's raw-cookie test
  fixture; the corrected four were rerun GREEN as stated above. A final
  all-project sweep after subsequent narrow validation/fixture changes is
  still required before PR review.

## Actor-guard integration candidate

Read-only source inspection of the detached V030-013 worktree at
`4dcad40f09d5d20c2d2d9f97a12c7e41b4630bed` located the
`X-Expected-Actor-Id` check after authentication and before application or
operation handling. Its compiled BFF SHA-256 was
`97d8f65d33df043378f11c2639cd1fcb99a5b1c971e047063270feb146d301ad`.
Only the ignored isolated stack's BFF binary mount was switched to this
candidate; no V030-013 source or branch was changed. `real-v013-guard-chromium.txt`
records a real HTTPS GREEN after a separate nginx file-permission setup error
was corrected. `real-v013-matrix-six.txt` then passed all 18 real HTTP/browser
cases over Chromium, Firefox, WebKit and reduced-motion counterparts: root
create/reopen and member denial, create-only owner/group/menu/cross-app role
matrix, and negative actor guard including malformed-header and legacy controls.
The old BFF RED remains in `actor-recovery-red/real-guard-before-backend.txt`.

The browser regression sweep loaded before the last two test additions and
showed `211 passed, 5 failed`; all five failures were a WebKit-only test
locator race, selecting the old account-menu button while `/login` was still
rendering. Changing the test locator to the login textbox resolved it.
`actor-recovery-red/six-focused-final.txt` passed all 36 targeted cases across
the six modes, including those five reruns, malformed JSON-null confirmation,
and a me/access actor mismatch. The final stacked application suite then passed
all `228/228` Chromium, Firefox, WebKit and reduced-motion cases; see
`app-six-final-green.txt`.

The full inherited Chromium component run reached 248/261 before a concurrent
Docker writable layer ran out of space while Playwright attached a screenshot
to an unrelated Q36 visual test. `full-component-final.txt` contains the exact
`ENOSPC` trace; no product assertion failed before that point. The prior full
run passed 253 with one skipped. The final sequential pinned-container run
passed **260/260**, with one unchanged skip across 261 listed cases;
`full-component-final-green.txt` records the complete result, including the
previously interrupted visual case. The
first PR27 exact-head real matrix attempt hit the same Docker npm temporary
space limit before test collection; its raw diagnostic is in
`real-v013-e0f-environment-retry.txt`. After the other container exited, the
available workspace disk rose from 3.4 GiB to 13 GiB. Both runs passed when
retried sequentially. A host fallback lacked Playwright's Chromium binary;
its full raw setup log is retained privately, not in this review bundle.

The earlier PR27 head `e0f1820c1d9bf579b7039b0590632c79cf98be98`
compiled with Go1.26 into SHA-256
`0e07897527f66080851c52df3242e494bf5c87d04c9d232338c333dc72b7753d`.
`real-v013-e0f-matrix-six.txt` then passed 18/18 real HTTPS cases with this
binary, including actor mismatch 409, malformed actor 400, same-actor and
legacy controls, role matrix, and root/member UI. This proves this frontend
and candidate server behavior together; it does not constitute a formal source
merge. The newer PR27 `bf4956394581979f3371a8840a95f3c59166b801`
compiled into SHA-256
`f27233763d9cac230c4ab9efc1d8fa237f838d33ae85a62b5d7c5ee55a2007b8`.
With only that BFF binary mount changed, the exact same 18 real HTTPS tests
passed again across all six browser/reduced modes; see
`real-v013-bf-matrix-six.txt`. This is isolated integration evidence, not
approval of PR27's broader server tests or its remote CI.

After the node_modules refresh, `final-typecheck-build.txt` records passing
typecheck, production build, and task structure check. `git diff --check`
also passed. The existing Vite chunk-size warning is unchanged. Draft PR28
targets `task/V030-011-applications` at `b4c0fb3`; its remote checks are
tracked against the eventual exact evidence commit.

The read-only V030-014 mounting assessment at
`cb08774ed00e715402e45c31228a508e86d93055` is in
`v014-interface-review.md`. Its checkpoint lacks the shared actor-bound
request and recovery protocol, so form mounting awaits the reviewed compatible
module SHA and root's integration freeze.
