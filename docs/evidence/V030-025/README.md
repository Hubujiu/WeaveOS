# V030-025 evidence

Draft [PR36](https://github.com/Hubujiu/WeaveOS/pull/36), base `task/V030-024-integrate-verified`. No main merge or deployment. Root owns design, assertions and acceptance.

## Provenance and independent oracle

Verified candidate `21a7912245779c679d7c94bbcd8d5e9cb6eb0bd4`; original Root test-first commit `46a73789220449a6b93df731b9aa7d0c1a8a75f5`. Before implementation: **4 failed / 2 passed**, exit1, typecheck exit0. Two desktop cases failed sidebar ownership; Exit and Save cases failed missing global-header controls. Preview/focus and narrow/reduced-motion cases passed. [Raw RED](red/run.txt), [runtime manifest](red/environment.json), immutable original test, prior V023 test and hashes remain in red/. RED checkpoint `b87594241def3a159ec05b073259bbd5e8a567e7` precedes implementation `e8b7566b390ec3e8522b0d4316ab3deb0229fedb`.

Root reviewed three contract/fixture conflicts and authored correction `e44de4ee8bab8576cccd0d798990e50330b4a2b8`; cherry-pick `fa67481` retains Root authorship. Exactly the current-form POST records/search is allowed after runtime return (read only); all other writes remain forbidden. Two existing tests now inspect requested Exit/top context; persistent Shell and source-route assertions remain. [Correction patch and snapshots](root-corrections/). Implementation never changed assertions. [486 baseline test/fixture byte comparisons](test-integrity-final.json) show zero worker changes relative to Root correction.

## Validation

Default pinned image `mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`, original repository browser configs, no sandbox/host-validation overrides. Docker mounts only isolated task checkout and existing pnpm installation; browser runs at container-local127.0.0.1:4173. API traffic uses existing test fixtures; production API is unchanged.

- Root exact six: **6 passed** ([log](green/root-final.txt)). Command: `pnpm exec playwright test -c apps/web/playwright.component.config.ts root-monochrome-designer.component.spec.ts --reporter=list --output=/tmp/root-results`.
- Independent corrected Shell/return/menu cases: **7 passed** ([log](green/root-corrected.txt); targeted subset includes4Root cases).
- Final three-browser forms: **234 passed**, exit0,3.6m ([log](green/forms-final.txt)). Previous log also retained.
- Complete first attempt at4workers: **496 passed,1skipped,4failed** ([log](green/components.txt)). Three Root-corrected expectations; one personnel-menu interception. Unchanged menu test passes at default1worker. Final complete default1worker regression **500 passed /1existing skipped /0failed**, exit0,13.4m ([log](green/components-final.txt)); no weakened selectors or timeout changes.
- Final typecheck/build: PASS ([typecheck](green/typecheck-final.txt), [build](green/build-final.txt)). Existing chunk-size advisory only. Initial generated Vite-cache EACCES retained separately; only task cache ownership repaired.
- Governance/foundation: **240 passed** ([log](green/governance-final.txt)); [task scope](green/tasks-final.txt) and [repository](green/repository-final.txt) PASS. Task required exact Scope heading and PR metadata were corrected, without changing policy.

## Browser screenshots

Root's Figma498:9471 context/screenshot defines visual target; not used as an image asset. Approved existing React/CSS/GSAP and Lucide controls reused. All11 fields and4layout kinds, extra property/keyboard controls, save/preflight/permissions/recovery/leave behaviors retained. Screenshots use API-shaped Root fixture plus four fields added through real UI, reduced motion; they are visual evidence, not live production content. Failed-save image uses genuine preflight INVALID_INPUT400 rejection and retained draft. Screenshot helper [capture.cjs.txt](capture.cjs.txt) contains no assertions; its fixture is original Root prefix before tests, with import removed for transpilation. Actual viewport sizes:1440x1000,1280x800,390x844. Mobile center/properties scroll within shared Shell and remain reachable; no forced desktop page width.

| State |1440|1280|390|
|---|---|---|---|
|Designer|[PNG](screenshots/designer-filled-1440.png)|[PNG](screenshots/designer-filled-1280.png)|[PNG](screenshots/designer-filled-390.png)|
|Field settings|[PNG](screenshots/field-settings-1440.png)|[PNG](screenshots/field-settings-1280.png)|[PNG](screenshots/field-settings-390.png)|
|Preview|[PNG](screenshots/preview-1440.png)|[PNG](screenshots/preview-1280.png)|[PNG](screenshots/preview-390.png)|
|Dirty exit|[PNG](screenshots/dirty-exit-1440.png)|[PNG](screenshots/dirty-exit-1280.png)|[PNG](screenshots/dirty-exit-390.png)|
|Preflight error|[PNG](screenshots/save-failure-1440.png)|[PNG](screenshots/save-failure-1280.png)|[PNG](screenshots/save-failure-390.png)|

All15PNG files are committed as GitHub evidence. Screenshot archive `WeaveOS-V030-025-screenshots.zip` successfully saved to ChatGPT Library; returned identity in [library.json](library.json), local identity persisted successfully. Root must review visual fidelity. CI result must be checked against exact final head; earlier candidate/implementation checks do not establish final validity.
