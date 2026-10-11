# P4 implementation plan, not implemented

Extend the existing useApplicationOperation's finite resource union with task-action and preserve the same identity preflight, immutable original packet, synchronous inFlight guard, lifecycle cancellation and per-actor scoped recovery. Do not create a second framework or treat record-save confirmation as execution success.

- start only exact resource five-ID /actions POST with closed action/basisToken; operationId is generated once. Existing record/draft whitelists unchanged.
- specialized transport now tested separately reuses the private request() and retains actual HTTP200/202; ordinary envelope shape stays unchanged. Scope/body validator may be extracted from this tested helper to avoid duplicating its whitelist.
- perform sends action or reads application-workflow-operations/{original operationId}; strict validWorkflowOperation ties operation/instance/first-confirmed command and HTTP/method to the three closed DTOs. Pending Location must target same-origin exact operation path.
- pending stores the original immutable packet plus confirmed command ID and exposes accepted, never confirmed callback. Unknown retains it; retry uses same body/key. Terminal success/no_effect clears recovery and delivers actual result. Ordinary application-operations cannot substitute execution status.
- TaskPage Save/Agree use independent typed scopes but synchronous shared exclusion via getPendingStatus from both. Pending/unknown prohibits new Save/Agree and browser departure warning persists. Guard includes both operation IDs/statuses.
- Agree with unsaved local fields warns, never auto-saves. Definitive stale basis requires explicit refresh. Terminal success shows approval result without treating a subsequent closed-task GET as a failed approval; explicit return/refetch remains available.
- Reject UI is blocked solely on the pending reason-input product decision, not on these common technical rules.

Eight independent UI tests were committed before action hook/UI implementation and are running on remote45dc8c9c. Wait for actual behavior RED before adding this behavior. Pure response35 and transport23 already have preserved RED→GREEN; no action UI test has yet passed.

17:48UTC checkpoint: implemented after the preserved eight actual RED cases. Exact remote5cfe2119 / product38160670031 / job114533039328 now reports122 passed, including all eight original Agree cases and prior Save/preview/inbox cases. New hardening cases have only compiled; full CI remains running. This historical plan header describes its pre-implementation state, not the current status.
