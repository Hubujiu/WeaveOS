# Root review supplement to 229c6da

The root explicitly requested four checks and subsequently authorized minimal `.github/workflows/ci.yml` wiring. Work remains in `task/V010-020-q36-front-b1`; the original PR21 branch, Nginx policy, dependencies, services, secrets and workflow permissions are unchanged. This supplement supersedes the prior checkpoint's remaining local full-regression gap and its statement that Q36 component CI wiring remained solely with the root.

## Hidden selections

Root source review was correct: direct Table's `use-row-selection.ts` builds a set from all controlled IDs; `toggleAll` copies that set and adds/removes visible IDs, without pruning hidden IDs. `PersonnelAdmin` cleared selection on other query changes but omitted `memberFilter`. This caused a selected member to reappear checked after a custom filter hid it, current-page selection was applied, and the filter was reset.

Commit `5405801` preserves the independent user-interaction test and the three-browser RED (`selection-red.txt`). The production fix adds `memberFilter` to the existing clearing effect; the shared Table's selection semantics are unchanged. The identical targeted case passed three engines (`selection-green.txt`). A real HTTPS/BFF/PG/Redis browser case passed separately (`selection-real-green.txt`) and added three actual pixels: `final/members-selection-before-filter.png`, `members-selection-filtered-current-page.png`, and `members-selection-reset-cleared.png`. All three were inspected: the old member is checked first, only the filtered current member is checked second, and both members/header are unchecked after reset. This is a new relevant real test; the earlier ten-case real suite was not repeated for unrelated motion/draft behavior.

## Real Nginx size boundaries

The existing Go `query_engine_http_test.go`, `drafts_http_test.go` and contract `query-post.contract.test.mjs` provide the independent route/raw/canonical assertions. No existing repository entry script combined all four Q36 limits at actual HTTPS; the public `ingress-boundaries.test.mjs` carries those assertions to the already running isolated Nginx/BFF/PG/Redis. The API header policy follows `infra/acceptance/ingress.test.mjs`, without running its disruptive gateway-recreation/pause cases.

The running image ID is verified against repository-pinned official Nginx 1.30.5 digest `b972f831f200b19ef0767938224f9711e74cd783718738cd7405d5cabf75c442`. The mounted config is asserted equal to the shared renderer output with only loopback listen port 19443→19445. It contains no `client_max_body_size` expansion. All five real tests pass (`ingress-green.txt`):

| Independent boundary | Accepted | Rejected | Additional proof |
| --- | --- | --- | --- |
| Large Chinese query body | 1,800 repetitions of 中文, >8KiB, HTTP 200 | — | Actual HTTPS route, not a Go httptest or mocked response |
| Query raw body, members and events POST | 65,536 bytes, HTTP 200 | 65,537 bytes, HTTP 400 / COMMON_INVALID_ARGUMENT | Small `{}` plus raw whitespace isolates the raw cap |
| Filter canonical JSON, members and events POST | 16,384 bytes, HTTP 200 | 16,385 bytes, HTTP 400 | Chinese escape spelling plus whitespace still succeeds at query raw 65,536 bytes with the same allowed canonical filter |
| Draft raw create and update | 131,072 bytes, create 201 / update 200 | 131,073 bytes, HTTP 400 | Oversized create adds no draft; oversized update preserves saved version/payload |
| Draft canonical payload | 65,536 bytes, create 201 / update 200, wrapper below raw cap | 65,537 bytes, create/update HTTP 400 | Unique Chinese permission-code strings respect individual field lengths; rejected update preserves exact prior payload |

The tests explicitly remove only their two freshly created synthetic boundary drafts with the exact successful versions. Existing test drafts are preserved. Credentials stay in the private fixture file; no request/cookie/credential values are logged. Every API response checks one Nginx request ID, nosniff and no-store headers.

The first harness attempt (`ingress-fixture-attempt.txt`) had two setup errors: requiring the Compose tag string to literally contain the digest even though the running image already had that exact digest, and a boundary generator with the wrong filler byte arithmetic. These were not product REDs. The corrected checks compare immutable image IDs and assert exact independent UTF-8 byte lengths before sending. A subsequent added exact shared-renderer config comparison also passed.

```powershell
$env:WEAVEOS_WEB_URL='https://localhost:19445'
$env:WEAVEOS_ACCEPTANCE_FIXTURES='<private synthetic fixtures.json>'
$env:WEAVEOS_Q36_NGINX='weaveos-q36-b2-1790878899100-nginx-1'
node --test docs/evidence/V010-020/q36-b2/ingress-boundaries.test.mjs
```

## Pinned security scan

The official repository-fixed Gitleaks image `zricethezav/gitleaks:v8.30.1@sha256:c00b6bd0aeb3071cbcb79009cb16a60dd9e0a7c60e2be9ab65d25e6bc8abbb7f` is already cached locally. A normal execution of `node --test infra/runtime/security.test.mjs` passed its Gitleaks tracked-HEAD archive case at source `229c6da8214e2f5d12abf05d966400882ea768de`, plus the file's three unchanged dependency/vulnerability assertions (`security-229c6da-green.txt`, four passes). No alternate registry/version/credentials were used. This earlier scan is attributed to that exact HEAD and does not claim to have scanned later uncommitted changes. Final-source archive scan provenance is reported when the supplement is committed.

## Minimal continuous regression

The root authorized only two Q36 commands in the existing browser job after browser installation: `node apps/web/src/q36-front-filter.test.mjs` and the committed three-engine component config restricted to `q36-front-` and `q36-b2.component`. Output goes under the job's existing `test-results/` artifact path. The existing smoke, contract checks, root typecheck/build, fixed actions/dependencies, safety checks, service definitions and `contents: read` permission are retained. This job does not run the actual Q36 HTTPS stack.

`ci-wiring.test.mjs` observed missing execution as RED in `5405801`, then passed on the four-line workflow change (`ci-wiring-green.txt`). Existing CI/query POST/ingress policy assertions passed seven cases (`ci-contracts-green.txt`). Pure filter tests passed four cases (`pure-filter-green.txt`). Root `pnpm typecheck` includes web and `tsc -p tsconfig.tests.json`; it and root build passed (`root-typecheck-green.txt`, `supplement-build-green.txt`). Full component and old-regression batch results are collected once; no duplicate test process was running when they were started.

## Narrow layout provenance

The narrow editor width is inherited from the B1 baseline `f4891938bf202f5d0ab230666fc455743d395112`: `.admin-shell` has `--side:176px`, `.admin-sidebar` has `min-width:176px`, and `.definition-details` has `padding:24px`. Those same declarations are present in the final source. At 390px, the fixed sidebar plus content/card padding leaves a narrow editor; this is an existing design/layout constraint, not a new sidebar-width regression. The B2 conflict controls now wrap inside that editor. No sidebar architecture change is made under this supplement.

## Final results

Final production/workflow/test source is committed as `17285e397462d2c5ca9b85648103d6209a241143`. Root typecheck/build exit 0; real ingress five tests, real selection one test, targeted selection three engines, pure filter four tests and existing CI/POST/ingress assertions seven tests pass. The source commit's fixed Gitleaks tracked archive passes 1/1 (`gitleaks-17285e3-green.txt`). The complete old component suite passes 107/107 on this final source (`legacy-final-green.txt`), in one uninterrupted 6.1-minute batch. The prior invitation navigation timeout does not recur.

The local combined CI component command matched all 105 B1/B2 cases. Its first batch reached 72 passes, six conditional skips and 27 failures after the concurrent legacy runner naturally stopped the Vite process it had launched; the combined runner had reused that process. At diagnosis there was no remaining Vite process, and failed navigation errors report connection refusal. This is a local shared-server lifecycle failure, not a filter/draft assertion RED. The original log is retained in `ci-components-environment-first.txt`. A separately owned Vite was started and retained; only the 27 recorded failed cases are rerun with Playwright `--last-failed`, using the unchanged source/config and the same output directory. The actual CI job runs this Q36 command serially, so it owns its Vite until completion and does not overlap the legacy job. Recovery results are finalized below; no test timeout/assertion/conditional-skip rule is changed.

The affected-case recovery completed on unchanged frontend/workflow source: 26 passed and the remaining case took its existing WebKit native-transition conditional skip (`ci-components-recovery-green.txt`, exit 0). Across the original selection plus that recovery, all 105 cases are accounted for: B1 56 passes / seven established conditional skips; B2 42 passes / zero skips. This is accurately reported as combined batch plus affected-case recovery, not a first-batch all-pass result. No duplicate complete suite was launched. The final actual-pixel directory contains 44 PNGs and the preserved 9.48-second formal-page video.

The only subsequent script adjustment expands the canonical filter byte checks to both query routes; its final five-test real ingress batch remains green. It does not change frontend/workflow/backend/config. Final evidence is committed separately; fixed Gitleaks is also executed on that final published HEAD with its result reported in the handoff. Root integration, remote PR CI, Notion synchronization and acceptance remain with the root.
