# Sorting menu keyboard RED

Independent oracle: confirmed R3 §5.8 keyboard/focus acceptance and the user enabling sorting. Direction keys must expose the actual menu, focus its options with a visible outline, Home/End reach boundaries, native Enter selects and Escape returns focus to the invoking trigger. These accessibility adaptations are expressly allowed in the project vendor; upstream originals are not edited.

Actual command `pnpm exec playwright test --config .work/q35-font.config.ts apps/web/src/personnel-arca-keyboard.component.spec.ts --project chromium`, Windows Node22.23.1 / pnpm10.28.2 / Playwright1.63, exit1: trigger ArrowDown leaves aria-expanded=false (expected true). Actual behavior failure, not dependency/network failure. The immutable ZIP holds original test, unchanged source listener, styles, dependency lock/config and raw log bytes. Keyboard implementation has not yet been written. Concurrent full139/cross195 runs use unchanged product source and their original discovered tests; this independent newly added test file is not part of those historical runs.

Final test also includes native Enter/Space opening and first-item focus. Actual unpatched-source repeat on4174 with .work/q35.config.ts --grep "sort menu supports" exits1 on the same ArrowDown assertion; final test/source/config/raw log bytes preserved separately before implementation.
