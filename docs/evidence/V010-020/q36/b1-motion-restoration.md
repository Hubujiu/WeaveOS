# Q36 B1 filter motion restoration

This is synthetic component evidence from the isolated frontend fixture on port 43121. It does not establish business-flow acceptance or B2 integration.

## Diagnosis and implementation

The old 3.72-second recording ends with a blank focused trigger. Earlier instrumentation of its interaction sequence observed one pending native transition at the old stopping point. Waiting for native `finished` and another second restored the trigger. The preserved old final frame demonstrates the recording symptom.

A separate real defect was observed before implementation: the first native opening had an exiting shell animation of 220ms and no shared geometry group. RED commit `b6e172477368b5f32f3ff8ce184ad4ba0f0f654f` preserves the test and actual failing output in `b1-shell-timing-red.txt`.

The popup now mounts and measures before the shell exchange. Opening and closing use the same shell class with 300ms and 220ms respectively. Popup closure and root animation suppression cleanup follow actual animation completion, with a generation guard against stale rapid-toggle completion. Text and icon snapshots have independent timing. Initial keyboard focus and Escape return focus are restored; outside clicks preserve their destination.

## Verification

The browser tests compare exact PNG bytes of the focused trigger before opening and after native completion plus one stable second. They also assert text opacity/visibility and visible SVG, then reopen and close outside. Native, fallback, reduced-motion, and rapid reversal paths are covered.

Commands:

- `node node_modules/@playwright/test/cli.js test --config .work/q36-front.config.ts --workers 2`
- `node apps/web/src/q36-front-filter.test.mjs`
- pinned pnpm 10.28.2: `--filter @weaveos/web typecheck` and `--filter @weaveos/web build`
- `git diff --check`

Latest browser result: 56 passed, 7 explicitly skipped, no failures. Chromium and Firefox each passed 20 cases, skipping only the WebKit-specific fallback regression. WebKit passed 16 fallback/reduced-motion cases; its five native cases remain skipped because of the previously reproduced engine crash. This does not claim native WebKit acceptance. Typed filter tests passed 4/4; typecheck, build, and whitespace validation passed. Build retains the existing >500kB bundle warning.

`b1-restoration-first-run.txt` preserves the initial 54-pass / 2-failure / 7-skip run, which exposed initial focus and Escape return-focus defects before their correction. `b1-restoration-final.txt` preserves the final passing rerun.

## Visual evidence

- `b1-filter-motion-restored.webm`: full initial state, opening, editing, Escape close, stable closed hold over one second, reopening, outside close, and another stable hold. Navigation white frames are trimmed without cutting the final close.
- `b1-filter-open-restored.png`: stable expanded panel.
- `b1-trigger-restored.png`: stable trigger after Escape close and 1.2 seconds.
- `b1-old-video-last-frame.png`: preserved old blank trigger symptom.
- `b1-restored-video-first-frame.png`, `b1-restored-video-closed-frame.png`, `b1-restored-video-last-frame.png`: actual encoded-video frames, all checked visually for visible trigger text and icon.

Recording source: `apps/web/src/q36-front-record.mjs`. Old evidence remains unchanged. Parent visual acceptance is pending; B2 work has not started.
