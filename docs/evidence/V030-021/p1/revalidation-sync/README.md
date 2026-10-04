# Root-authored revalidation synchronization verification

Source: Root d0ea88048b5981f3d6e629021d73e35f8c82ba51 and fixture correction 3a27ba2bf27293adfbb1bee822ae33bf55fde732. Only Root authored test changes. Production remains byte-identical to f518200a0965d047e25ad73779b6b83c95d07d04; no production authorization repair is claimed.

The original product CI on 97589d3 failed the permission-dialog test (493 passed, 1 failed, 1 skipped). Its same-head second attempt succeeded, including later product acceptance. Both raw logs are retained. Exact original CI timing is unproven because no failure trace was available. Local controlled batching reproduced a possible missing revalidation mechanism; original-order observational probes passed 60/60 at normal speed and 100/100 at 20x CPU slowdown. Controlled traces demonstrate both merged and committed transitions; they are not claimed to be the original CI failure.

Root d0ea880 added intermediate B response/dialog0/enabled and returning A response barriers while preserving all original mask/name/CAS assertions. This exposed an invalid B fixture: generic data belonged to A, correctly rejected by production response validation. Two first-failure traces and the interrupted repeat log are retained. Root 3a27ba2 supplies exact B-scoped empty valid structures without changing generic fixtures or production validation.

Targeted command: `PLAYWRIGHT_BROWSERS_PATH=/tmp/weaveos-browsers pnpm exec playwright test -c apps/web/playwright.component.config.ts --grep 'cached structure dialog is masked after permission revocation|revalidated newer structure retains original name and CAS version' --repeat-each=25 --trace=retain-on-failure --output=/tmp/weaveos-root-revalidation-fixed-traces --reporter=line`. Result: 50 passed, exit 0.

Typecheck/build, governance 93/93, repository/task and actual PR33 scope checks passed. Existing build size advisory remains. Full component command uses the same config, no grep/repeat, `--trace=retain-on-failure --output=/tmp/weaveos-full-revalidation-traces --reporter=line`; result is recorded below after completion.

Screenshots are reused from ../screenshots: production unchanged, all 12 entries in SHA256SUMS verified. No new design or screenshot claims. Diagnostic JSON/trace files contain synthetic fixture data. Historical temporary file paths in diagnosis.md identify the original capture locations; durable copies are in this directory.

Full result on Root source 3a27ba2: 494 passed, 1 existing skipped (495 total), 13.9m, exit 0. No failed-test traces generated. Root forms file hash verified after the run. This evidence commit freezes the next CI candidate; remote exact-head CI and Root acceptance remain pending at commit time. No merge or deployment.
