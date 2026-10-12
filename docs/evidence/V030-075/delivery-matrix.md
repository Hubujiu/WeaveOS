# Full product delivery matrix — baseline 4d2d43a2
Read-only inventory at 2026-10-11 16:19 UTC. A backend endpoint and its own tests do not prove UI delivery.

| Capability | Baseline frontend evidence | Remaining delivery |
| --- | --- | --- |
| Application shell/catalog/access/groups | ApplicationWorkspace, PermissionManager | Full permission/identity regressions in combined product |
| Structure and 11-field designer | ApplicationStructurePanel, FormDesigner, FieldRenderer | Latest structure lifecycle/copy and version conflict integration |
| Record runtime/list/create/edit | RecordRoute, RecordWorkspace, RecordForm | Draft/history completeness, submit/trigger, deletion and recovery UI |
| Personal current tasks | V061 backend /workflow-tasks/search only | Route, list, paging/queryVersion, safe detail, real browser |
| Started/completed personal workflows | No matching UI or global read API located | Freeze read contract, authorization and real stored query then UI |
| Workflow definition management | Backend definition GET/PUT, publish/enable/close/delete | Discover/list/create flow, graph editor, settings, server validation, recovery |
| Task preview/actions/local save | Backend V037/V043/V045 | Read latest evidence, edit-mask intersection, save separate from agree/reject/return |
| Lifecycle/parallel flows/round actions | Backend V044/V046/V069 | Per-record status, withdraw/rework/resubmit/reopen and independent rounds |
| Confirmed events/evidence | Backend V062–064 | Timeline, history selection, original evidence view, permissions |
| Templates/copies | Backend V070/V074 | Export/import/upload/mappings/preflight/execute and original-result recovery |
| Private table presets | Backend V071, generic preset component exists | Record workspace persistence binding and scope regressions |
| Record/structure deletion | Backend V072/V073 | Exact impact/confirmation, active-flow guard, restore and recovery |
| Complete product acceptance | Existing older integration suites | New real multi-actor browser/BFF/Flowable full journey, failure path and visual review |

No new production deploy/main approval. Notifications, scheduled/webhook triggers, reports, formulas and business attachments remain excluded per approved scope. This is an initial source inventory, not a percentage or final backend completeness claim.

2026-10-11 16:56UTC source correction: V015 ADR14.2 explicitly excludes automatic reuse of ordinary reference-candidates for task fields. V043 grants task edit mask intersection but no dedicated task candidate endpoint was located. Complete node member/department editing therefore requires a frozen, tested current-task candidate contract; do not silently route it through ordinary form candidates. P3 first non-reference types can proceed independently. This is an additional bounded backend/UI gap, not a claim that the backend is fully complete.

2026-10-11 17:22UTC P4 response audit: OpenAPI WorkflowTaskNoEffect requires sequence even when zero, but WorkflowOperationResult.sequence has omitempty and no custom serializer. A focused real encoding test is needed before correction; do not relax the frontend contract to hide a missing terminal sequence.

## Verified checkpoint — 2026-10-11 19:12 UTC

- P1 inbox and P2 preview: original interaction suites passed exact CI candidates; full combined real-service journey remains outstanding.
- P3 node Save and P4 Agree: exact component cases passed, including unknown-result replay, identity and access revocation. Reject remains blocked on the explicit reason decision, not complete.
- P5 trusted task candidates: actual PG/Redis/HTTPS full Go job114544971758 passed on1444cde4. The next-page revocation UI fix passed on7cfe4385. Final full-product verification still needed.
- P6 history/detail/persisted comparison:11 original UI cases passed on7cfe4385, including layout and cancel/switch. Root visual review discovered clipped comparison heading; new regression is not yet executed, so visual acceptance remains open.
- P7 record workflows: six independent UI RED retained; basic scoped read component implemented locally after that RED. Additional full-page/dirty/cancel RED is running at04f80a84, no full-page migration yet.
- P8 withdrawal: pure wire/request/terminal tests RED→GREEN;8 UI cases at04f80a84 still running with no withdrawal UI/hook implementation. Three extra interaction tests are local and NOT RUN. Derived OpenAPI missing withdrawn result independently RED→126 contract suite GREEN; final lint unverified because of telemetry review block.
- New pushes that would rerun default Redocly telemetry are paused pending the narrow user question. No telemetry or network-security settings changed. Local source/read-only inspection, pure tests, and already-started CI observation continue.
- Remaining definitions/designer/lifecycle rounds/global lists/templates/copy/deletion/presets/full real browser acceptance are unchanged open scope; none is silently dropped or counted complete.

## Resumed checkpoint — 2026-10-12 06:40 UTC

- User continued the work; approved no-telemetry lint now passes with 0 errors/14 existing warnings. CI and local preflight explicitly force telemetry off, with independent 3 RED then 4 GREEN privacy checks.
- Published candidate070c9d04 exactly matches local08d98d14 tree. Full Go job114689729899 succeeded at original10-minute budget, and all engine/recovery/formal/RPC/backup jobs succeeded. Legacy browser suite still running; not all CI green.
- P7 all9 full-page/scoped-summary UI cases passed. P8 original8 plus3 double-click/dirty/identity cases passed on shard4 114689809540. New comparison scroll, accepted-withdraw close, and revoked-confirmation tests produced3 actual REDs; minimal fixes are local, browser GREEN pending.
- Shard1 old readonly footer expected a disabled Save button; approved Figma483:2764 and independent P7 test require no readonly write footer. Updated only this legacy expectation, preserving exact value/noneditable/permission assertions and original evidence.
- P9 private preset consumption: current V071/V015 sources and Figma486:4393/4558 read;90 wire/request/receipt tests passed after independent RED.11 real-Shell tests are written but NOT RUN; no preset UI/mutation integration yet.
- Other full-product gaps and reject/return reason decision remain open. No main merge/deploy or overall completion claim.

P9 additional source-audit gap: shared manager currently has no member/department filter candidate picker. V01514.2 ordinary candidates authorize create/edit only; readonly query permission cannot borrow that route. Complete private presets require an independently scoped, current read-coverage-authorized query-candidate contract and tests. Numeric/text first acceptance is not full reference-filter delivery.

## 2026-10-12 07:47 UTC checkpoint (not final acceptance)

- P1–P8 current task list, authorized task details/local Save/actions/candidates/history, record-associated workflow summaries and withdrawal are implemented and have staged browser evidence. The three P7/P8 boundary fixes (history comparison visibility, retain record after closing withdrawal, revoke withdrawal capability on 403) passed at 169d7fb7 and subsequent candidates. Reject/return reason policy remains unanswered; not silently implemented.
- P9 private presets: all original 14 actual Shell cases passed at a7016008 (product38217717926, job114706843399), plus beforeunload and immutable unknown-input cases. This does not prove complete presets. Five cases failed: unsaved global navigation, confirmed-save reread locks, SPA return before reaching recovery, refresh pointer hit testing, and readonly text color. Two bounded implementation fixes are now published at 7cc7bc34, awaiting their exact rerun; navigation diagnosis and three new visual oracles are running there. System date/column descriptors have ten independent pure cases (5 real RED then 10 GREEN); their actual UI integration tests are local and unexecuted, and the adapter is deliberately not wired yet. Reference query candidate scope is an unanswered product decision.
- CI: a7016008 full real Go job114706986080 passed at the unchanged original ten-minute budget; earlier cancellations remain recorded rather than reclassified. 7cc7bc34 independently discovers critical Go identities and executes the complete race suite once, preserving all original test bodies, serial package budget, count=1, vet/build, storage/fixture and cleanup. Local 1050 checks passed; that candidate's real Go run remains pending. Later local descriptor changes bring the pure/contract/governance aggregate to1060 passing. None of these counts proves full product delivery.
- Workflow-management discovery/list and approver-selection HTTP remain missing despite existing definition CRUD/publish/enable/close/delete operations. V020 ADR was reread and explicitly covers only selected-ID operations. The actual Figma management content483:3310 was read and its screenshot reviewed: live name/trigger/state/in-flight table, new/edit/actions and server-backed filtering/sorting are required. A complete current list/candidate contract and independent real-storage tests are still needed; no invented lists, sample IDs or hardcoded counts will be delivered.
- Remaining rows above still apply: graph designer/settings, manual start/rework/resubmit/review, personal started/completed listings, draft/history/delete/copy/template journeys, complete permission/fault/multi-actor browser-to-BFF-to-Flowable acceptance and final visual review. No ready/accepted status, develop/main merge or deployment has occurred.

## 2026-10-12 08:25 UTC checkpoint (not final acceptance)
- P9 original14 Shell cases remain passed; confirmed-save reread lock, readonly color, desktop dimensions and mobile usability passed9c1c1ee9. Navigation had overlapping dialogs, quick Back was blocked before recovery, and six system/search/numeric assertions were actual RED. Repairs published4ed27e64 await exact browser outcomes; reference candidates and separate visibility prototype remain incomplete.
- Go single-pass with independently discovered24 required identities passed7cc7bc34 and9c1c1ee9, preserving full race suite and original10-minute budget. Product and overall CI still are not green.
- P10 manual start candidate/receipt/request contracts and safe read POST are implemented with independent RED/GREEN. The original shared operation now handles durable accepted/ignored intents, exact generic-operation Location, original replay and identity preflight; nine non-DOM real-code protocol tests passed. Record-scoped manual guards and candidate page offset checks passed after independent RED. Aggregate1179 local checks plus typecheck/build passed.
- P10 ten real-Shell initial tests are running on4ed27e64 and no manual UI exists yet. No percentage/full completion claim; all remaining graph/settings/list/rounds/templates/drafts/deletion/full-product journeys above remain open. Two explicit product decisions remain unanswered, and main/deployment are not authorized.
