# P6 current implementation gate

2026-10-11 18:29 UTC. Root alone. V062 ADR re-read after recovery; V075 P5/P6 ADR re-read18:20. Figma484:3324 and484:3503 high-fidelity context and actual screenshots inspected18:25–18:27. The former has left record/right340px history,20px gap, Save above independent bottom decision actions. The latter is a full-width original/current comparison with three columns, not an approval timeline. Existing tokens/Shell/Card/Button/Table are reused; no vendor/source modifications.

Wire36 independent tests: actual11 positive RED then36 GREEN. Original display/comparison12 additional cases:11 nonempty/value cases RED; empty text already matches empty declaration and is not claimed RED. All48 pass. Exact source/test/command logs retained.

History UI seven first cases are on remote4aa0f59c with no history implementation; wait for actual browser behavior RED. Four more cases (persisted-current comparison, desktop/mobile layout, event switching and leaving cancellation) are local test-only, NOT RUN. Do not implement their new behavior before actual RED.

Use one record-scoped history business component and existing authenticated request transport, not another operation framework. Cursor history cannot be cast to page/queryVersion. Both list and selected detail have independent abort/late-response protection. History errors clear list/detail and never revoke separately authorized task editing. Selection is demand-only, validates the original summary. No pending artificial event or current-value fallback. Changes in the editable form are not comparison values; only the latest persisted preview can be used.

Prior task/action fixture setup now returns an independently authorized empty history for the new read. Original action/save/permission assertions are unchanged. Dedicated history fixture owns history failure assertions. This is API fixture composition, not suppression of a real history failure.

No complete-product or pixel-perfect acceptance claimed. Comparison routing/full-width composition and final screenshots must match source without losing unsaved changes or creating new writes. Reject reason remains a separate pending user decision.
