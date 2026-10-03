# V030-011 / PR26 inherited filter-panel focus repair

Local repair ready for parent review and B5-owner integration. This evidence does
not claim remote CI success, merge, deployment, or whole-product acceptance.

## Scope and authorization

- Diagnosis baseline: PR26 `062495e2146f1a52f0328ba058dcfc73dbd5b864`.
- Integration base: `08e8a97abc222ab3fd7fd99461d972b613950fa2`.
- Inherited PR21 source: `74cc5824ee4336dd76c72874f0cc4cf38fe9e889`.
- Branch/worktree: `task/V030-011-webkit-focus` in the saved cloud environment.
- Parent froze B5a.18 before implementation. Relevant PRD readback was
  `2026-10-03T06:53:46.766Z`; ADR009 readback was
  `2026-10-03T06:53:53.535Z`. Exact bounded release excerpts are retained in
  [frozen-plan-readback.md](frozen-plan-readback.md). Whole-page statuses remain
  PRD pending review / ADR proposed.
- Product implementation is confined to `apps/web/src/TablePresetManager.tsx`.
  Tests append to `apps/web/src/q36-b2.component.spec.ts`; evidence uses this
  new owned subtree. Shared motion helpers, styles, backup fixtures, CI files,
  and the B5-owned task document are unchanged.
- No remote branch writes. PR21 remains untouched. B5 owns shared fixture/docs
  integration, final task update, and same-head CI.

## Confirmed defect and resulting behavior

The fallback exit callback at old `TablePresetManager.tsx:58` restores trigger
focus a second time after Base UI already restores it on close. Deliberate
checkbox focus does not change the old animation generation, so that completion
steals the checkbox's newer focus. CI subsequently sends Space to the trigger
and reopens the panel. The captured previous active element and callback stack
in [exit-focus-events.json](exit-focus-events.json) identify this exact transfer.
The native `finishShared` path had the same unguarded restoration authority.

Each close now has a monotonic lifecycle token and single-use restoration
authority shared by Base UI finalFocus and native/fallback completion. Ordinary
close still restores the trigger; popup removal causing body focus still permits
that restoration. External focus or newer deliberate pointer/keyboard input,
reopen, and component unmount revoke the old authority. Native callbacks and
the existing completion timer carry their captured token. Base UI performs its
guarded restoration synchronously rather than queuing an additional transfer.

The implementation preserves dirty-input confirmation, Escape/outside-close
behavior, popup geometry, origin animation, existing curves and durations,
reduced-motion terminal behavior, and repeated-open/close intent.

## Recoverable RED and unchanged assertions

The first tests-only commit is
`fe125489f4a1490039aff636292d27acae70c338` (06:48:42 UTC). It retains the actual
WAAPI exit animation and gates delivery of its real finished notification until
after deliberate checkbox focus. It adds no sleep, retry, skip, or timeout
extension. Chromium, Firefox, and WebKit all fail the same focused assertion
on the original implementation. The old CI test passed once locally on its
ordinary timing; this race-sensitive pass did not override the confirmed RED.

The second tests-only commit is
`ea164b63af916341a16ba012799a0c1fc4d79cdb` (07:05:45 UTC). It adds close authority
and lifecycle coverage before implementation. Source snapshots in
[red-and-final-sources.tar.gz](red-and-final-sources.tar.gz) preserve original,
primary deterministic RED, additional RED, final test, and before/final product
source. The inherited tests and the entire primary deterministic RED remain a
byte-identical prefix of the final test file; no original assertion changed.

Additional harness corrections are disclosed rather than counted as product RED:

- A search field covered by the centered popup was an invalid outside-click
  target. The corrected test uses the visible select-all checkbox.
- Removal-induced body focus is captured inside the same browser evaluation;
  a later evaluation could observe Base UI's legitimate restoration instead.
- Seven first post-repair Escape cases started while the intended popup control
  was still disabled/loading, leaving focus on the trigger and producing zero
  restoration calls. The final setup waits for and explicitly focuses that
  control. Its expected exactly-one restoration assertion remains unchanged.
- A TypeScript timer-wrapper annotation was corrected. An evidence-only
  `test.use` video/trace declaration was rejected before test loading and removed.

Mixed initial lifecycle logs are retained, including their failures. They are
not a substitute for the valid original target RED. After implementation began,
a separately labelled baseline replay used the final test bytes against the old
implementation: five duplicate-restoration failures and one pass over six
ordinary-close cases. [baseline-red-replay.log](baseline-red-replay.log) is a
replay, not a claim of pre-implementation execution. No history was reordered.

## Final checks

All browser runs use Playwright 1.63.0, one worker, zero retries, and all three
engines. Q36's nine inherited capability skips are unchanged.

| Check | Pass | Fail | Skip | Exit |
| --- | ---: | ---: | ---: | ---: |
| Full Q36 B1/B2 selection, including new focus cases | 159 | 0 | 9 | 0 |
| Q36 Chromium subset | 55 | 0 | 1 | 0 |
| Q36 Firefox subset | 55 | 0 | 1 | 0 |
| Q36 WebKit subset | 49 | 0 | 7 | 0 |
| Full saved-filter-manager component matrix | 72 | 0 | 0 | 0 |
| Pure-filter checks | 4 | 0 | 0 | 0 |
| Governance and foundation checks | 222 | 0 | 0 | 0 |
| Original deterministic RED recheck with retained focus events | 3 | 0 | 0 | 0 |
| Typecheck, including tests | — | — | — | 0 |
| Production build | — | — | — | 0 |
| Repository/task structure checks and diff whitespace check | — | — | — | 0 |

Raw list output is in `q36-final.log`. `q36-result-counts.json` explicitly records
derived per-engine counts; it is not presented as an original JSON reporter
artifact. Saved-filter raw JSON/list output and the other command logs are
retained. The build has its inherited over-500-kB chunk warning.

`target-final.json` retains the original deterministic test's actual GREEN focus
event attachments, also extracted per engine. This short additional recording
run preserves the focus transfers because the full list reporter does not retain
successful body attachments. Source-and-evidence digests use explicit path/sha256
records and preserve the original secret gate.

Coverage includes normal close-button/Escape restoration exactly once; newer
focus even after its target is removed; removal-induced body focus; outside
pointer input and Tab; close/reopen; completion delivered after unmount; normal
engine selection; forced fallback; reduced motion; and the unchanged original
trusted-press/Space regression. Chromium and Firefox exercise native transitions;
WebKit uses its existing safe fallback. The saved-filter suite also preserves
unsaved editor confirmation and existing CRUD/query/visibility checks.

Twelve [screenshots](screenshots/) capture manager control, editor input, closed
trigger, and checked checkbox focus in Chromium/Firefox/WebKit. Assertions check
actual DOM focus and keyboard behavior; screenshots provide review evidence.

## Reproduction commands and environment

```sh
export PATH=/workspace/.weaveos-tools/bin:$PATH
export PLAYWRIGHT_BROWSERS_PATH=/workspace/.weaveos-tools/browsers
export LD_LIBRARY_PATH=/workspace/.weaveos-tools/sysroot/usr/lib/x86_64-linux-gnu
export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1
export MOZ_DISABLE_CONTENT_SANDBOX=1
pnpm exec playwright test --config docs/evidence/V010-020/q36-b2/components.config.ts 'q36-front-|q36-b2\.component' --output .work/focus-check
pnpm exec playwright test --config docs/evidence/V010-020/filter-manager/frontend/components.config.ts --output .work/manager-check
node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs
node apps/web/src/q36-front-filter.test.mjs
pnpm typecheck
pnpm build
```

Saved host-library checks proved the private libGLESv2 library loads, but it is absent from system ldconfig's
cache. The environment-only host-validation bypass uses the saved browsers; it
does not skip WebKit. Firefox needs its local content-sandbox process setting.
Initial launch/setup errors remain in the original diagnosis archive and final
Library delivery; they are not counted as product assertion failures.

## Delivery limits and next action

The Library delivery contains the incremental Git bundle/patch, this durable
evidence, and the immutable initial diagnosis ZIP with CI trace/video and target
RED traces. Parent should review the actual diff and cherry-pick the ordered
tests-only and repair commits onto B5's current PR26 branch. The branch advanced
to `684e59966f678cff970c5c6027e46df98a8f979a` during this task; read-only comparison
found its new changes confined to B5 fixture/docs evidence, with these frontend
paths unchanged.

Not run here: B5 backend/PostgreSQL/backup integration suites and same-head remote
CI after integration. No production/manual Safari or whole-product acceptance is
claimed. Parent review, B5 integration/task record, and same-head CI remain.
