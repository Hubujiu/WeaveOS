# Root R23 Shell integration with reviewed R22 RecordForm

Reviewed V017 source integrated locally: `e123b3287bc04a4e746c5c1387747be6cf4976d8` (merge parent 2 is the exact R23 base `ea5b1e83eb81a9f30a05193e4a0f5a9de3d00e80`). The R23 `RecordRoute` composes the real shared-operation editor without an adapter.

TypeScript typecheck and Vite production build pass. The first full Shell run was 6/9 green. The three failures were: the create/edit test asserted the second authoritative detail read before the async runtime/detail refresh settled; stale-open was triggered by StrictMode's aborted initial search reaching the API before a row existed; and dirty-modal focus was lost when nested native dialogs unmounted together. The focus issue was fixed in `RecordRoute.tsx`, with its focused Root regression green 1/1. The aborted-search issue was fixed in the allowed `RecordWorkspace.tsx`; its focused stale-open regression is green 1/1. The full Shell suite then reached 8/9; only the immediate read-count assertion remains, pending Root's test-owner update.

After integrating the exact reviewed R22 follow-up `d36a0396b749da423a20fa666d53f51b7181b4a2`, the migrated legacy form-shell, root operation/scope/boundary/leave/read-contract/workspace, and record form/list/panel/recovery/editor suites passed 92/92. The targeted Root Shell run is still 8/9 because its create/edit case checks the async second GET before it settles. No Root test file was edited in this worktree.

No real HTTPS record API acceptance was run in this checkpoint. Final focused Root and record component suites, the three-browser/reduced-motion coverage, and real acceptance remain pending.

Raw Shell outputs, corrected typecheck/build logs, pinned-container commands and exit codes are included here.
