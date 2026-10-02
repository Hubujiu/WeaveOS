# Q35 final read-only source and bridge review

Reviewed 2026-10-01. Reader: /root/arca_table_source. Product writer remains /root. No worktree, Notion or Figma writes were made by this reviewer. Browser used its own context and intercepted every local API request with synthetic identities; official comparison used the public Arca page. Browser artifacts are temporary, not standalone acceptance or TDD evidence.

## Actual source consumption

The fixed reference is Hubujiu/arca-ui c0319d887e10775c8968a7d6a451f248858726ec. All 22 vendored TS/TSX files match this closure: 9 exact normalized-LF files, 7 with only import-path changes, and 6 with bounded integration/ARIA/keyboard modifications (Table index/types/header/pagination, Select, ColumnFilterMenu). Table rendering, useColumnSort/useRowSelection, virtualizer, reorder/resize hooks, Checkbox, DropdownPanel, Button, ease/touch/utils and original pager algorithm are actual source. Checkbox and DropdownPanel motion definitions and ease constants remain unchanged. No externally owned source was edited.

Direct dependencies match official fixed lock versions. pnpm transitive resolution differs for framer-motion 13.4.6 versus official 13.4.0, and motion-dom 13.4.5 versus official 13.3.0; motion-utils remains 13.3.0. This is a documented resolved dependency difference, not a observed defect. Upstream has no LICENSE designation, which must not be inferred from package dependency licenses.

## Complete index.css dependency audit

- Lines 1 and 8-49: Tailwind 4.3.3 core plus inline theme. The inline theme block is byte identical after line-ending normalization. Every required class is generated inside .arca-source. A scan of all 22 source modules against actual Vite CSS found 298 representable class selectors and zero missing selectors. Later accessibility classes compile in root's successful target runs.
- Lines 2-3: tw-animate-css and shadcn helper imports have no references in the closure. Core animate-pulse, aria/data/has/in variants and pseudo-element variants are Tailwind core. Do not import a whole library for these.
- Line 4: @fontsource-variable/geist 5.3.0 is consumed. Scoped Geist plus existing Noto Sans SC is the confirmed project fallback; outer typography stays Noto.
- Line 6: source @custom-variant dark (&:is(.dark *)) is necessary to avoid implicit OS dark mode altering a light component. Initial omission was reproduced and root has restored this exact rule.
- Lines 51-84: all 32 light custom properties match; local radius aliases are necessary for the original Button arbitrary-radius classes.
- Lines 86-118: dark theme values are outside the confirmed light-only project scope. This review does not request a dark theme.
- Lines 120-130: source base * border-border/outline-ring/50 and body bg-background/text-foreground/antialiased must be mapped into component scope. html font-sans is mapped to the component font root, retaining the confirmed Chinese fallback. Initial inherited blue pager and missing antialias were reproduced; root has restored scoped base.
- Lines 132-185: grid/dot backgrounds, page/surface/ink/line aliases, card shadows and scrollbar-hide have zero uses in the 22-module closure, including Button, Select and all table auxiliaries. They are unrelated demo/page rules.
- Lines 188-268: every original el-scrollbar rule is byte identical after line-ending normalization, now inside scope. No missing class, selector, interaction color, 340 ms opacity transition or thumb styling was found.
- Tailwind 4.3.3 preflight is exact after html/:host becomes scoped '&'. Defaults and media resets are isolated to .arca-source. Font-face and theme/property declarations are global declarations; visible rules remain scoped.

Comparison files: final-review-source-css-comparison.json; final-review-utility-coverage.json; final-review-runtime.css; final-review-compiled-css.css. Source comparison returned themeInlineExact=true, lightVariableCount=32, variableDifferences=[], scrollbarRulesExact=true. Preflight comparison returned scopedPreflightExact=true.

## Required external cascade conflict

One remaining source-style conflict was reported after restored base: apps/web/src/style.css line 8 applies global button:focus-visible and a:focus-visible. It still wins over scoped '* outline-ring/50' for original header grip and sort trigger; workspace.css already excludes the scope but style.css does not. After 250 ms, official Chromium controls show outlineColor oklab(0.708 0 0 / 0.5), offset 0px, while local controls show rgb(47,116,244), offset 3px. UA outline width/style should not be hard-coded across engines. The original table classes intentionally do not define an outline for these two controls.

Minimal fix: exclude .arca-source and its descendants from the existing style.css global focus selectors, preserving the same rule for authentication and outer shell. Do not invent focused grip/filter styles. The outer personnel-nav currently shows rgb(59,121,244), solid 2px, offset 3px via workspace.css and must retain it; the auth rule remains #2f74f4.

Exact browser evidence: final-review-focus-local.txt and final-review-focus-official.txt; fixture/probe scripts final-review-fixture125.js and final-review-focus-style.js. The public page comparison and local comparison used the same Chromium browser. Root is independently writing/observing the corresponding RED before changing style.css.

## Keyboard and bridge

Initial ColumnFilterMenu inherited source accessibility defects were observed using actual Enter/ArrowDown/Tab/End/Escape input: opening left focus on trigger, ArrowDown did nothing, Tab moved to another header, menu item had no visible focus, End did nothing, Escape from menu item dropped focus to BODY. Evidence: final-review-keyboard-result.txt and final-review-keyboard.js. Root has independently performed RED and added bounded keyboard/focus handlers plus focus-visible styling without changing DropdownPanel/variants/timing; root reports its three-engine target checks pass. This review did not substitute those checks.

manualPagination retains the fetched server page and authoritative total without a second slice. Current-page sorting does not reset server page. Paging and page-size changes reach actual API query state; authoritative total shrink recovers the legal server page. Original full pager/ellipsis algorithm and source Select remain in use. Virtualizer count/total/padding use real entries only; inert aria-hidden filler rows are outside data, ids, selection, API parameters and total counts. Selection is controlled and clears when member search, filters, page or size changes. Existing per-member operations remain bound to actual row id/version; no table insert/delete/edit callbacks or new business writes are supplied. Identities and permission templates share the restored card-list/detail path and existing dirty-edit protection.

Original scrollbar ResizeObserver, pointer drag/track jump, hover/scroll visibility and sizing remain source code. Table viewport adds fill-height and scrollPaddingTop for keyboard reachability. The only active body Portal is ColumnFilterMenu and has the source scope; row/column mutation menu modules have no handlers and are unreachable in this bridge. Existing non-source CSS table/button/input/nth-column rules are excluded in workspace.css; business cell typography and text actions remain explicit project content styles.

No additional reachable same-class CSS omissions, false data/count paths, new permission bypasses or unsupported table mutations were found in the reviewed scope. The global style.css focus conflict is the only remaining concrete issue at the end of this review. Full regression, final HEAD CI and source/evidence registration are owned by root and remain outside this read-only review's result.
