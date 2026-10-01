# Q35 compatibility and menu RED

Independent oracle: user Q35, R3 §5.8, Q25 paging, Figma activity toolbar 164:3506 (search → safety hint → flexible space → time filter, right inset 16), and unchanged business requirements.

Windows / Node22.23.1 / pnpm10.28.2 / Playwright1.63. Actual commands, all exit 1:

- `pnpm exec playwright test --config .work/q35.config.ts`: 137 cases, 131 passed / 6 failed (6.4m). Product defects: two real time-range input clicks blocked by the sticky header and time filter shifted 276px. Test preparation defects: two Q32 queries accidentally selected the new hidden original pager listbox instead of the existing PersonnelControls menu, and Q31 expected virtualized row 19 before scrolling it into the rendered range.
- `pnpm exec playwright test --config .work/q35-cross.config.ts --grep 'Q35|Q34|Q32|Q31|Q33|Figma responsive'`: 126 cases, 119 passed / 7 failed (8.3m). Six are the same incorrect Q32 global listbox query; Firefox original header sorting failed because its own menu scroll closed the menu.
- `pnpm exec playwright test --config .work/q35-font.config.ts apps/web/src/personnel-arca.component.spec.ts`: 39 cases, 38 passed / 1 failed (2.7m). All three source-font assertions pass after the prior font RED; the same Firefox sorting defect remains.
- `pnpm exec playwright test --config .work/q35.config.ts apps/web/src/personnel-arca.component.spec.ts --grep 'sorting menu stays'`: 1 failed (3.6s). Dispatching scroll on the actual menu makes its trigger `aria-expanded=false`, independently required true. This focused RED was observed before changing the source scroll listener.

Original byte logs remain in original-run-logs.zip; readable copies only strip trailing whitespace. The current snapshot contains the font GREEN and the new focused menu test, before menu/toolbar fixes. The archived e31709a tree recovers the earlier baseline (font test was added after the full137 discovery, and the focused menu test after all three longer runs). Do not present snapshots as original execution timestamps. Correct the two test preparations without removing any corner/gap/intermediate-height, real row, bounds, request, or safety assertion. No force click, sleep, skip or retry workaround.
