# P1 execution evidence

Base: 819aab8a25fa90790dedd092fdb04efda348dc81 (Root tests), parent db475e0dc00e2ac7c18fc5023cbc9039300711ac.
Worktree: /workspace/WeaveOS-worktrees/V030-021, branch task/V030-021-monochrome.

Read current PRD/ADR/business baseline and Figma 481:1729, 481:1839, 483:1921, 483:2332, 485:3621 via design context plus screenshots. Existing shared Table and business-operation primitives are preserved. Sample tasks/statistics/people in Figma are not production data. Component source instructions and approved Tabs/Button registry read; existing business composition is retained, no upstream component copies changed.

Environment: Node and pnpm supplied by executor (pnpm 11.19.0; repository declares 10.28.2). Frozen-lockfile install with scripts disabled; no lockfile change. Default pnpm home was unavailable, so installation used XDG_DATA_HOME=/tmp/weaveos-pnpm, PNPM_HOME=/tmp/weaveos-pnpm/bin, --store-dir /tmp/weaveos-pnpm/store. Chromium installed into /tmp/weaveos-browsers.

RED command (before production edits):
`PLAYWRIGHT_BROWSERS_PATH=/tmp/weaveos-browsers pnpm exec playwright test -c apps/web/playwright.component.config.ts root-monochrome-shell --reporter=line`
Original test hash: root-test.sha256. Raw output: red.log.

Known independent-oracle conflicts reported to Root, tests unchanged: personnel.component.spec.ts old 176px sidebar / 56px header / original AdminMaterial / fixed x16 y116 menu assertions; applications.component.spec.ts WaveOS brand versus new WeaveOS Figma. Root owns any expectation updates.

RED completed: exit 1, 7 failures on missing persistent Shell, 1 existing record authoritative-save case passed. GREEN after Shell composition: 8/8, exit 0, green.log. Intermediate green-initial.log retains the three background cascade failures (5 passed) before specificity was corrected.

Typecheck and build exit 0; retained advisory for the existing large JS chunk. Initial screenshots use the unmodified Root and personnel fixtures via capture.mjs, at 1440x1000, 1280x800 and 390x844 (reduced motion). They are interim visual evidence; old control color and icon/brand review remains open. No real HTTPS integration executed in this UI fixture environment.


Review checkpoint after Root-owned a0bca100 (cherry-picked as 5da84cc): original complete run at f1b0603 ended exit 1 with 464 passed / 29 failed / 1 existing skipped. See components.log and components-failures.md. Root tests remain byte-identical. Brand remains WaveOS by Root instruction. Current neutral-control CSS leaves Arca Table source untouched. Typecheck/build pass again; components-migrated.log is in progress.

12 refreshed review PNGs in screenshots/ use settled font/animation capture, with SHA256SUMS. Sizes: 1440x1000,1280x800,390x844 (reduced motion). Legacy icons are unresolved, so these are not final visual approval. Library upload attempted through current helper, failed before upload with `hosted apps tools/list request failed with HTTP status 401`; no Library file IDs were created. Parent may retrieve PNGs from this task branch while arranging an available delivery route.


Final density/icon checkpoint: imported Root e8988f0 as e54cc8b. density-red.log records valid failure before implementation; density-test.sha256 and root-test.sha256 verify both independent test sources. Root explicitly approved lucide House/LayoutGrid/Settings (installed dependency), replacing interim icons. Titles/actions and tabs/refresh are compacted without hiding business actions; original personnel scroll owner restored. Initial personnel-density-initial.log: 96 passed, one compact tab scroll failure. Final personnel-density-green.log: 111 passed, exit0 (complete personnel, Root monochrome Shell and Root record Shell). Typecheck-density/build-density/repository-final/task-documents-final logs pass. Screenshots and SHA256SUMS now reflect these final edits, pending Root visual review.

The complete pre-density migrated run has 489 passed / 4 failed / 1 existing skipped. Search-scroll failure was repaired and passed in final111. Three legacy visual assertions outside the authorized Root test migration remain to be reviewed by Root: applications.component.spec.ts:728 (56/176), personnel-arca-keyboard.component.spec.ts:58 (outer title #102044), q36-b2.component.spec.ts:128 (narrow x>=176). No executor-authored tests, weakened assertions or added skips. Full final suite is not claimed green; draft PR held until remaining oracle work and review.
