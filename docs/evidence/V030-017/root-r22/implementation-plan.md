# Root R22: one record editor using the shared operation hook

Purpose: finish the actual ordinary-record editor used by the Shell, without a second operation controller. Implementation model is gpt-6-luna; every test and acceptance oracle is personally authored by Root. Preserve original dirty trees. This is a successor to R21, not permission for broader features.

## Ordered prerequisites
1. Root will provide the exact published V012 R21 commit. Merge that task commit into this isolated V017 task branch only, preserving V017 R19 RecordForm layout and both sets of tests. Do not guess the base or use an old dirty worktree.
2. Root's new root-editor suite has ten cases. Run it before editing RecordForm and preserve actual HTTP/UI RED; old code may fail because onSave/onRecover were removed from fixture props. Do not count pre-migration TypeScript mismatch as behavioral RED.
3. Existing recovery, renderer and layout fixture callers have been migrated by Root. Their HTTP mock now passes through the real shared API/hook rather than returning SaveOutcome. No worker may edit fixtures/tests, assertions, selectors or test runner settings.

## Exact public interface
Keep view, identity, record, mode, authorityKey, loadCandidates, onConfirmed, onDirtyChange and onDiscard.
Remove RecordSaveCommand, SaveOutcome, onSave, onRecover.
Add queryVersion?: string; onUnauthorized():void; onIdentityMismatch():void; registerLeaveGuard:RegisterLeaveGuard; onRefresh():void.
RecordForm imports and uses useApplicationOperation<MutationResult> directly. It must not generate operation UUIDs or implement another pending/unknown state machine.

## Identity and immutable recovery
- Resource is {kind:'record',appId,viewId,creationNonce:identity.clientDraftId} for new records, or {kind:'record',appId,viewId,id:identity.recordId} for saved records. Scope string is a deterministic record-editor namespace; hook already includes resource and actor.
- This component handles create/edit/read records only. A draft identity must not be written to an ordinary record endpoint; keep it non-submittable until the separate approved draft flow supplies a real record identity.
- On a remount with an unresolved packet, reconstruct only currently editable inputs from the frozen packet's values/changes over the current projected defaults/baseline. Do not copy fields denied by current runtime. A different actor must see neither the packet nor inputs.
- Freeze field editing while preflight, write pending, unknown, confirmed, or structural review is required. Keep previously entered values on a definite rejection. A failed lookup never converts an unknown write into a fresh editable submission.
- Keep recovery label '恢复保存结果'. It invokes shared query(). A same-key retry may use shared retry(), never start(). Keep exact status '保存结果待确认' for unresolved state and a safe alert for failed lookup.
- On valid confirmation invoke onConfirmed once with minimal receipt and the confirmed record identity. Freeze further save on this mounted instance. The Shell will perform the authoritative reread; do not synthesize record values from the submitted body.

## Write packets
POST applications/<app>/forms/<view>/records, status201:
body={expectedSchemaVersion:startView.schemaVersion, values:current editable present values, ...(queryVersion?{queryVersion}:{})}.
PATCH .../records/<id>, status200:
body={expectedSchemaVersion:startView.schemaVersion, expectedRecordVersion:record.recordVersion, changes:only changed currently editable values, ...(queryVersion?{queryVersion}:{})}.
Do not send viewVersion, identity, clientDraftId, extra operationId, labels, or readonly/system/denied fields. The shared hook adds the operationId exactly once.
Use isMutationResult(result,packet.operationId,expectedRecordId) as the strict additional receipt validator; hook already checks positive versions and Location for POST.
New records may submit defaults even when not dirty. Edit requires a changed editable value. Schema/query/record/policy conflicts require refresh before any fresh write; keep input and display a safe message containing '刷新' plus button '刷新记录' calling onRefresh. Do not silently rebase.

## Leave behavior
Register the exact record scope with recordId or clientDraftId.
Use the existing createLeaveController with refs that read current values, current packet ID and synchronous operation.getPendingStatus(). Status precedence: operation pending/unknown, then dirty draft, then clean. Fingerprint must change for values, packet/state and runtime review changes. Cancel unsent preflight on discard; retain sent operation and original key on retain_operation. Never call onDiscard while a write is pending/unknown from the ordinary footer. Keep the registry instance-safe.
Do not erase recovery on unmount or actor switch.

## Layout and styling
Retain R19 recursive group/field/divider/system rendering and current-permission projection. Add description as plain text, never HTML. Keep per-field lookup Map (O(fields+layout)).
Use existing admin-button/dialog/form tokens. Ensure the record-form subtree has border-box sizing so grid controls do not overflow; no global design changes. Root's earlier isolated screenshots did not constitute visual acceptance.
No modification to renderer internals or navigation components in this slice.

## Allowed implementation files
apps/web/src/applications/records/RecordForm.tsx
apps/web/src/applications/applications.css (record-form scoped rules only)
No test edits; no other API/hook changes without reporting the exact missing contract to Root.

## Evidence and stop
After GREEN, run all records component suites and Root shared-operation suites, typecheck/build. Save full outer pinned-container commands, mounted raw logs, exit codes and screenshots of create, edit and unknown states. Do not run the whole 400+ suite for this isolated slice.
Report exact patch and results. Root reviews before publication; no main merge or deployment.
