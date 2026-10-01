# Q36 B2 frontend delivery evidence

Authority: `docs/tasks/V010-020-PLAN.md`, version `Q36-approved-2026-10-01`, and the root's subsequent B2 delegation. This is the frontend worker's evidence, not root acceptance or a replacement shared contract. The root retains task/PLAN/contracts/migration ownership. Notion recovery and synchronization remain with the root; latest Notion content is not claimed as read here.

## Source and scope

Branch: `task/V010-020-q36-front-b1`, isolated Windows checkout `pr21-frontend-isolated`. B1 remote baseline `f4891938bf202f5d0ab230666fc455743d395112` contains the originally authorized `7bca3b108c2961536ae282174f37578471a3469d`. Backend `61aa705545011eb0ec4de5eba3d964cde7ecf64d` was merged normally in `cdd0df2b2bfaa19a9acf99367ed1e132160e2a0f`; this worker did not independently change backend, DTO, HTTP contracts, migrations, shared dependencies, CI or the original PR21 branch.

The final frontend consumes Arca Table directly; retains controlled pages, page sizes, selection, widths and order; uses typed POST filters and server time ordering/display; carries opaque contexts through navigation; distinguishes changed/expired/busy outcomes; refreshes explicitly without losing editor input. Unrelated definition writes do not discard member contexts. Durable drafts are explicitly saved and restored, preserve the original base version, expose invalid references/target conflicts/CAS resolution, and acknowledge exact draft versions on business writes so newer drafts survive.

## Reproducible verification

Run from repository root with Node 22.23.1 and locked pnpm 10.28.2:

```powershell
pnpm --filter @weaveos/web typecheck
pnpm --filter @weaveos/web build
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts q36-b2.component
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/components.config.ts q36-front-
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/legacy.config.ts
$env:WEAVEOS_WEB_URL='https://localhost:<isolated-port>'
$env:WEAVEOS_ACCEPTANCE_FIXTURES='<private synthetic fixtures.json>'
node node_modules/@playwright/test/cli.js test --config docs/evidence/V010-020/q36-b2/real.config.ts
node docs/evidence/V010-020/q36-b2/capture-narrow-draft.mjs
```

The pinned pnpm CLI was used when the pnpm shim was unavailable. The existing pnpm global rc warning and the build's >500 kB chunk warning are recorded; no dependency changes were introduced to silence them. Public component config starts/reuses loopback Vite at 43121 (override `WEAVEOS_COMPONENT_PORT`); legacy config uses Chromium and two workers. B1 has 56 passes and seven existing conditional skips: unsupported/unsafe native transitions in WebKit and the WebKit-only crash regression on other engines. Chromium/Firefox native shared transitions and all three engines' fallback/reduced paths run.

Actual E2E has ten serial Chromium cases, reaching real loopback HTTPS/Nginx/BFF/PostgreSQL/Redis without business API mocks. It covers full filters/pages/selection/column preservation, server event projection/time ordering/range, unrelated versus related changes, real LRU expiration, all five durable draft kinds and fresh login, original base/explicit conflict resolution, CAS/current-input preservation, exact cleanup/newer draft survival, two authorized accounts' draft isolation, twenty nested conditions/empty results, and formal-page open/close/reopen recording. No credential-bearing traces or automatic videos are enabled. Login occurs through the context API before recording the formal page.

Final real instance: `weaveos-q36-b2-1790878899100`, `https://localhost:19445`, PostgreSQL 18.6, Redis 8.2.10, Nginx 1.30.5; migrations 00001–00004 applied. The earlier instance at 19444 and its drafts are retained. Both are isolated synthetic test instances with only loopback HTTPS published. Private fixtures, TLS keys, environment files, database state and execution artifacts remain under ignored `.work/` and must not be committed. This test instance uses the database owner and does not prove least-privilege runtime roles. Go race/vet, backend fault suites and complete PR CI were not rerun by this worker. The new Q36 real spec uses this dedicated public config; root CI wiring/integration remains separate.

## RED chronology and corrected evidence

Behavioral RED preceded implementation: `628bb25` (eight genuine initial failures), `62bad01` (missing reference/modal refresh), `4e7503a` (sort geometry), `7a293d7` (unrelated card refresh and reopen intent), `e39f1e4` (real long-group overflow/isolation), `a53ea08` (long-group overflow in three engines). Logs are retained here. Row geometry, inert fillers, reverse keyboard focus, queued focus scroll and page-size menu clipping were repaired; valid geometry/focus assertions were retained. Legacy fixtures were updated for approved POST contexts/backend display and removal of member text sorting; page shrinking now requires explicit refresh instead of silent clamping.

Actual narrow editor detail pixels exposed long conflict actions clipped by the right-hand container. `fd93981` retains the independent viewport-bounds assertion and RED: Chromium/WebKit right edge 407 exceeded width 390; Firefox passed the same case. The repair constrains the draft flex item and conflict box and allows action text to wrap. The unchanged assertion then passed all three engines. `draft-*-action-clipped-red.png` retains the pre-repair actual HTTPS pixels. Final supplementary detail captures are inspected after rebuilding; scrolling to the conflict area alone is not enough to claim a fully readable action.

`narrow-reopen-real-red.txt` exposed a screenshot-helper mistake: Base UI stays mounted during native close, so aria-expanded alone was not a reliable requested-state oracle. It is not product-native-failure evidence. The separate `reopen-intent-red.txt` is a genuine last-intent regression. The real long-group test initially treated empty text as invalid; the frozen contract allows it. Its corrected independent oracle uses an empty group as invalid, three levels/twenty leaves and footer bounds. The corrected long-group footer RED and three-engine RED are genuine. The final repair combines flex/height constraints with Base UI sticky placement when resizing moves the anchor outside the viewport.

## Actual pixels and motion

All 28 original screenshots in this directory are preserved, including the long-group RED screenshot and clipped-pager reproduction. Original `members-filter-expanded-narrow.png` and `events-filter-expanded-narrow.png` did not correctly establish an open panel; they are historical, not approved expanded-state proof. Use `final/` for the final actual HTTPS page. `final/*-narrow-detail.png` supplements the initial viewport shots by scrolling to the actual editor conflict controls. The capture script advances only a synthetic draft version to demonstrate CAS and intentionally preserves the newer draft.

Final captures cover default/applied/expanded members and events at desktop and 390×844; changed/expired query messages; five saved kinds; restored department; target/CAS conflicts; two-account isolation; invalid/long nested filters; empty results; formal motion states. The final 9.48-second `final/actual-filter-open-close-reopen.webm` records the formal page, waits 1.1 seconds after each close, reopens, and finishes with a stable trigger. `motion-video-last-frame.png` is extracted with FFmpeg `-sseof -0.08` (9.40 seconds) and was visually inspected for restored text/icon/focus and absence of the popup. Additional `motion-video-t*.png` frames are diagnostic sampled frames; their timestamps alone do not classify transition completion. Formal `motion-open`, `motion-closed-stable`, `motion-reopened`, `motion-final-stable` screenshots use explicit state and completed-animation checks. B1 verifies 300 ms open / 220 ms close native geometry.

Effective runtime model/reasoning metadata is unknown. No model setting is inferred from requested model/high labels. No private conversation history or memory writes were used. No merge, deployment, force push, unrelated worktree write, or process/container shutdown is authorized or performed.

## Final checkpoint

All production changes are frozen before the final commands. Typecheck and build exited 0. B2: 39/39 passed across Chromium/Firefox/WebKit (`b2-green.txt`); actual HTTPS E2E: 10/10 passed (`real-green.txt`); supplementary actual draft pixels exited 0 (`narrow-detail-green.txt`). Final actual captures contain 41 PNGs plus one 9.48-second WebM. Inspected final pixels include members/events expanded at 390×844, twenty long nested conditions with visible footer, both narrow draft conflict actions completely wrapped inside the editor, and the video last frame with restored trigger.

Full legacy Chromium suite: 107/107 passed (`legacy-green.txt`) before the final draft-control-only CSS repair. After that repair, the related layout/editor/navigation selection ran 39 cases: 38 passed and one timed out inside initial `page.goto` waiting for load, before invitation interaction (`layout-first-final.txt`). Its screenshot showed the rendered admin page; the failed test's behavior was not reached. The identical test passed separately on unchanged final code (`layout-invitation-green.txt`, 1/1). The specific resource responsible for the initial navigation timeout was not established; no timeout or assertion was weakened. B1: 56 passed / seven existing conditional skips (`b1-green.txt`); subsequent CSS changes were scoped solely to personnel draft controls/conflicts and do not alter the filter motion shell.

An interrupted legacy run had stopped at case 69 without a completion marker when execution sessions disappeared. Read-only process inspection found no remaining duplicate test process, so it was restarted once and completed as the 107-pass result above. Active isolated database/BFF/Nginx instances were retained throughout; no process/container was closed manually.

`SHA256SUMS.txt` binds the public logs, configs, scripts, historical and final pixels and recording (excluding itself). Text hashes use repository LF endings; normalize Windows checkout CRLF to LF before checking text hashes. New log copies have only trailing whitespace/blank EOF normalized; original execution logs remain private under `.work/`. All RED/log corrections above are retained. Remote publication is reported separately with the actual verified SHA; this README does not claim root acceptance. Root integration into PR21, complete PR CI and final acceptance, and Notion synchronization remain outside this worker's completed frontend scope.
