# Root R23: connect ordinary records to the actual application Shell

Root personally owns all tests and decisions. Implementation: gpt-6-luna. This completes the page wiring for the first record create/read/edit slice; it does not claim all v0.3 features are complete.

## Dependencies and branches
Use the exact Root-provided R21 published commit plus this test package. R22 RecordForm implementation is being completed separately; Root will supply its exact published commit for integration before final GREEN. Never substitute original dirty WIP or invent an adapter to the old onSave/onRecover interface. Preserve R19/R20/R21/R22 changes and Root test files.

## Route and UI contract
- /app/applications/:appId/forms/:viewId is the runtime records page for owner and ordinary authorized users.
- /.../forms/:viewId/design is the owner/Bootstrap design route. Validate canManage before mounting FormDesigner. Ordinary actors must not call definition or structure from a direct runtime route.
- Add explicit owner '配置表单' control on runtime page; in the application directory use '打开表单 <name>' for records and '配置表单 <name>' for owner design. The latter callback is optional on ApplicationStructurePanel so standalone structure fixtures keep their old interface.
- Root migrated the old form-shell regression to the explicit configuration action and /design URL; its leave/auth assertions remain unchanged.
- Preserve the existing Shell DOM and original 56/176 chrome. Root's design exploration remains paused; no visual redesign here.

## RecordRoute coordinator
Add applications/RecordRoute.tsx to compose existing RecordWorkspace, Modal and RecordForm, not a second query/mutation framework.
Key it by verified actor/app/view. Hold only editor identity/mode/current RuntimeView/current authorized RecordItem/queryVersion/read status and modal origin, not another write state machine.
- New: onCreate opens centered existing Modal title '新建记录' with stable createNewRecordIdentity. Pass current queryVersion and exact R22 props to RecordForm.
- Row: use only row ID to fetch ordinary GET .../records/<id>; do not treat the list row as authoritative edit data. Validate with parseRecordItem and current RuntimeView. Title '记录详情', initial mode read. Show '编辑记录' only when current row/field permissions permit it, using scopeAllows/projectRuntimeFields.
- Before opening a row, RecordWorkspace must revalidate the old query using its existing useResourceQuery.prepareChange(parameters), then accept that result. It must reuse current page/filter/sort/pageSize/queryVersion, not mint a new token. Keep old rows on APPLICATION_QUERY_CHANGED/CONTEXT_EXPIRED and require refresh; do not open detail. If implementation needs the accepted page, hold a ref to the last parsed page in transport; never inspect raw unvalidated data. Use the accepted runtime and query token for onOpenRecord.
- List stays mounted behind the modal. Do not issue owner definition calls to render record controls.
- R22 onConfirmed is called only for a validated receipt: immediately mark '记录已保存' as confirmed, unmount/freeze the write form, clear old query context by refreshing/remounting RecordWorkspace at page1, then reread runtime + ordinary detail to obtain normalized values.
- For a readable record, display the authoritative reread in read mode under '记录详情'. Allow a subsequent edit with fresh row/schema versions. Do not keep the pre-write queryVersion for this edit; the list has been refreshed.
- For create-only/no-read, show the confirmed message and close the editor without issuing record detail or record search. Never expand permissions to manufacture a detail view.
- If post-confirmation runtime/detail read fails, keep '记录已保存', show a safe alert containing '无法读取' and '重试读取记录'. Do not show a save action or turn the confirmed write into an unknown/failed write. Retry only GETs.
- On an unconfirmed operation retained across route unmount, discover only this actor/app/view's record packets through existing scopedUnconfirmed(). Offer recovery using packet.resource.id or creationNonce and current runtime. Never choose a different UUID for the original operation, show other actors' inputs, or silently send a new write. Do not persist unsent editor contents in a new store.

## Guard, focus and refresh
All modal close/Escape/footer discard routes go through existing requestSectionLeave; dirty draft requires the existing discard dialog, while sent/unknown operations are retained. Continuing editing preserves input. On confirmed discard/close, Modal restores focus to the actual originating trigger. Browser Back and app-tab close remain covered by Shell registry.
RecordForm onRefresh requests the same leave confirmation, then reloads runtime/detail and discards old query context to page1. Do not silently overwrite dirty input.
Abort stale detail/runtime reads and use identity/generation checks so actor/app/view changes cannot paint an earlier result. Authentication failures follow existing Shell callbacks.

## Ordinary reference candidates
Use the existing applicationApiEnvelope GET wrapper for applications/<app>/forms/<view>/reference-candidates with fieldId, action=create|edit, q, pageSize=20 and optional pageToken; edit includes recordId, create must omit it. This is separate from owner configuration member-candidates/department-candidates. Validate candidate IDs, active status, label and pagination before passing them into the existing FieldRenderer loader. Only invoke for current editable reference fields; never use owner candidate APIs for ordinary forms. Stop and report if the documented backend route differs.

## Implementation boundaries
Allowed: AppShell.tsx, ApplicationWorkspace.tsx, ApplicationStructurePanel.tsx (optional config callback only), RecordWorkspace.tsx (old-context open precheck only), new RecordRoute.tsx, applications.css (record modal scoped styling only).
R22 owns RecordForm; do not edit it in parallel or add compatibility adapters. No tests, mock transport, schemas, backend, vendor table or unrelated navigation changes.
Runtime list precheck adds one bounded page query before detail; no whole-result materialization. Parsing remains O(pageSize * visible fields), memory O(pageSize * visible fields). Do not claim OFFSET deep-page performance is solved.

## Test gates
Root's eight new Shell tests: ordinary runtime route, create->normalized detail->edit, confirmed-but-read-failed, create-only, stale-query row open, dirty modal Escape/focus, ordinary forbidden design, ordinary field-scoped reference candidates. The existing eight form-shell regressions are migrated to explicit design routing.
Run RED against baseline and save actual failures. Implement route/read wiring; await the exact R22 commit before final complete GREEN. Final targeted tests also include root-workspace, root-editor and all records suites, then typecheck/build. Capture real Shell screenshots for list/create/detail/edit/error states in the pinned browser container. Preserve full outer commands, mounted logs, exit codes and PNGs.
Return the smallest diff and results for Root review; no main merge or deployment.
