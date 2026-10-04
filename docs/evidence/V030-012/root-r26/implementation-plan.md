# Root R26 form-tree hit testing and rounding locator
Base d1f26dfa690673f51571865943fc427314d8fd8f. Actual CI37164228258:133/138 browsers passed. Two long nested form configuration clicks intercepted by detail panel; three Root journey attempts could not match exact wrapped select label. Earlier503/permission fixture failures passed.

Root authored all test changes. Read V012 ADR §18.
Implementation limited to:
- apps/web/src/applications/forms/forms.css
- apps/web/src/applications/forms/ApplicationStructurePanel.tsx

Keep the accepted Shell/global navigation and two-panel structure. Constrain grid children and row/first-name flex sizing; do not let long unbroken names expand past tree. Compact tree action visible labels to 打开 and 配置 while retaining full existing aria-label including form name. Keep form/directory name intact in DOM/accessibility, with ellipsis or wrapping and optional full-name title. Actions must stay visible and normally clickable. Use min-width:0/appropriate flex sizing; detail panel must remain interactive. No pointer-events:none workaround, forced click, hidden action or arbitrary z-index overlay.

Root added three component cases to the existing form-shell suite: nested long names at1280/1920 with actual center hit-testing + panel bounds for all3buttons, normal configure click, and a money rounding selection case. Root also corrects only the actual full journey rounding locator to a unique combobox name prefix, asserts selected HALF_UP. Keep every saved value/API status assertion and138 actual cases.

Run the3 new cases RED before implementation. Then implement the two-file bounded fix, run the complete form-shell suite in Chromium/Firefox/WebKit with existing pinned Playwright container. Do not edit config/tests to gain these projects; use a temporary command/config only if existing config already supports them, otherwise report to Root for the exact harness. Capture actual screenshots and full raw stdout/stderr/exit. Run typecheck/build, structural checks and diffcheck. Root may use the current Chromium fixture plus existing all-browser config discovered in repo. No full local stack, trust changes, main or deployment.
Preserve old evidence. Force-add exact sanitized .log paths and verify manifest members exist committed. Push same taskbranch once source/tests/checks are ready; Root reviews screenshots and exact-headCI. Report test/harness issues before improvising.

Complexity: CSS layout adds no data transforms or copying; existing directory traversal unchanged.
