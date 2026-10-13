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

## 2026-10-12 09:09 UTC checkpoint (not final acceptance)
- P9 system/search/numeric/refresh and shared-navigation prompt repairs passed39a57b33; actual SPA unmount original-operation recovery passedf1edfbfc. New independent visibility dialog four real RED cases are now minimally implemented at40f5e5af awaiting browser verification; reference filters remain decision-blocked.
- P10 first ten manual-start Shell cases passed39a57b33 andf1edfbfc. Additional recovery case was blocked by disabled Close while unknown. Published40f5 fixes only parent busy rendering; full unmount recovery has not yet been reached or implemented. Twelve original scoped-packet pure tests pass.
- P11 latest-round preview/receipt pure contracts42 pass after actual RED; sourceV069 reread confirms later latest-round decision supersedes oldV044 pending note. No round-action UI, rework editor, or shared-operation adapter yet.
- Full Go f1 cancelled at original budget despite successful cold source-cache save; next warm-cache behavior still unverified. All specialized backend jobs pass but this is not complete CI. Entire scope rows above remain open; no final delivery/develop/main/deploy claim.

## 2026-10-12 10:02 UTC checkpoint
Published dde: P11 selected resubmit/review9 UI passed; rework4 UI missing-entry RED preserved before implementation. P9 heading/geometry/focus passed;2 color expectations red then narrow local fix, remote GREEN pending. Local14 current rework permission intersection checks passed; RecordForm dedicated rework reuse now wired, full UI verification pending. New actual-unmount round/rework recovery tests await RED, not yet implemented. Exact dde complete Go and specialized backend jobs passed, browser still running. Full feature/real-service/visual delivery matrix above remains open.

## Verified checkpoint — 2026-10-12 10:43 UTC

- P9 display fields:8854 shard3 186PASS, including independent geometry/color/focus; exact screenshot personally inspected and retained. Private presets earlier SPA recovery remains verified.
- P10 manual start: original ten cases plus true-unmount original-key recovery passed. Complete real-service journey remains outstanding.
- P11 rounds: nine original resubmit/review cases and four original rework-save cases passed. Rework true-SPA recovery passed4944; lost current eligibility for already committed rework remained RED and is fixed in currentabd candidate pending results. Round post-retry locator corrected with all original assertions preserved. Footer geometry defect found in actual image; new independent Figma test unrun.
- P12 drafts: closed authorized read adapter27 initial cases8RED→27GREEN, cursor/field-conflict extension43total4RED→43GREEN. Seven actual-Shell UI tests published first onabd41843; no draft UI implementation yet. CRUD/list/delete/atomic submit/full recovery still open.
- Five warm exact-source candidates have complete Go success without reducing tests or raising time limits; latest4944 other specialized backend jobs passed. Current/full-product CI is not all green.
- Workflow manager/designer, personal lists, templates/copy, ordinary record history and deletion/restore, full product E2E and final visual remain open. No feature removed to create apparent completion.

## Verified checkpoint — 2026-10-12 11:30 UTC

- P11 all16 round/rework behavior cases plus Figma footer regression PASS on948261a0; actual revised footer screenshot personally inspected. Full multi-flow selection/whole-product visuals remain distinct.
- P12 original7 draft save/update/exact formal consumption/error cases PASS on948261a0. Actual POST new-draft Shell recovery RED preserved before original-packet wiring; pendingea4 GREEN. Currentea4 also has9 new list/deletion/explicit-refresh/edit-draft-POST/saved-draft-PATCH recovery/primary-color cases, no implementation until RED.
- Existing HTTP draft list contains extra pagination.pageSize; ADR002§5.4 reread and P12 source supplement verified. OpenAPI exact pagination binding1RED→7GREEN. Real BFF composition assertion is published but not yet run; no handler change yet.
- Current edit-draft UI requires ordinary target read; edit-only/no-read grants need independent handling/verification without fake record data. Drafts must remain accessible through their capabilities without forcing record list access.
- New-record input still uses existing Modal rather than full Figma page; layout/motion and final whole-product real-service journey remain open. No partial stage is claimed as final delivery.

## Verified checkpoint — 2026-10-13 08:03 UTC

Exact remote head e008a3218f2ffdca13285b31ff5ba185f11dfaa4, local13da983e/tree5356b81b. Current CI38365956024/product38365955924 are running; no develop/main merge or deployment.

- Definition read9/save9/publication recovery9: verified on preceding exact candidates. Current-runtime/schema3, real approver picker7 and initial close/enable7: GREEN on22ac. The full main CI on22ac, a9 and435 was SUCCESS, but each product run retained its explicitly new test-first RED cases.
- Nested conditions: original tree/large exact decimal/invalid decimal3 GREEN on a9; boolean/NULL/group insertion/leaf deletion3 and existing trigger configuration4 GREEN on435. Typed single/multi options and explicit null-trigger add/remove were RED on435 then implemented on e008; actual GREEN pending.
- Exact-edge graph helper17 GREEN. Existing-node approval insertion2 and closing reopen/current-candidate guard2 were RED on435 then implemented on e008; actual GREEN pending. True-branch insertion and same-group leaf append first run on e008; no corresponding implementation yet.
- Publication result modal: visible confirmed version/name and truthful disabled state verified on435, but original immediate geometry sample captured an animation intermediate frame. Corrected same520×340 oracle now waits for actual settled dimensions within the default assertion budget; no animation disabling/time-budget expansion. Final modal geometry/focus/visual review pending e008.
- Approver/close screenshots from435 are animation intermediate frames, not final visual approval. Independent checkbox/text alignment regression and settled screenshot checks are local test-first work, NOT RUN. Search/departments remain a separately documented P16 gap.
- P19 recoverable-record lifecycle protocol45 and shared original-operation hook8 GREEN; no lifecycle UI or deleted-record discovery is claimed.
- Backend global started/approved personal lists still absent. Definitions still need new-flow creation, condition-node insertion, node removal, nonexecution name/coordinates persistence, search/departments, deletion and final full designer interactions. Local new tests for condition-node insertion2 and workflow naming2 are NOT RUN, with corresponding UI not implemented.
- Templates/copies, structure/record deletion-restoration journeys, complete real multi-actor browser→BFF→Flowable acceptance and final visual/motion audit remain open. These are not dropped because narrow components pass.
- Two explicit product choices are still unanswered as of actual07:33 Notion reread: reject/return reason collection; read-only member/department filter-candidate visibility. No recommendation is treated as approval.

Full pure/governance/contracts1911, TypeScript/build, actual PR path scope and same-version official native Gitleaks of the tracked archive pass locally. Native scanning is supplemental, not a substitute for the unchanged CI Docker scanner. The b02 preflight failure was a diagnosed generic-key false positive on a Figma file locator; accurate source URL metadata removed the ambiguity without suppressing scan rules. That failed candidate never ran its business tests.

## Verified checkpoint — 2026-10-13 09:16 UTC

Current remote85409315227b9deed3779445dfa9f161ca7c5072 matches local77af5039/tree3aa02ea0; main/develop remain unchanged. Current CI38374245985/product38374245950 are running, not accepted.

- Exact480d full main CI38371674572 SUCCESS. Product components1/2/3 SUCCESS, component4 job115210579424185PASS/5FAIL. First creation6, name2, condition insertion2, retaining unsaved topology under invalid condition input all GREEN; picker actual row geometry GREEN. Actual settled close620×390 and manager narrow/trigger-settings screenshots personally inspected from verified artifact11799712787.
- Four new expected RED cases (node removal3, condition input geometry1) retained before their minimal implementation. The fifth failure is an OLD real Agree-completion beforeunload regression, not a new expected RED or a flaky pass. Original test and Workspace/CSS/page prestate retained before fixing the listener to consult current synchronous guards, as existing route protection does. Current854 browser regression outcome pending.
- P20 workflow-deletion wire59, transport23, shared hook11 passed after distinct actual REDs. Original deletion operation preserves CAS/key and queries its dedicated status, never the generic stale202 admission ledger; only confirmed deleted clears uncertainty. Seven first-run real-Shell deletion UI tests are now running on854; corresponding page UI has NOT been implemented.
- P19 recoverable record lifecycle remains protocol/hook only; deleted-record discovery and UI outstanding. Global personal started/approved lists, template/copy/structure lifecycle, approver search/departments, nonexecution node metadata and fuller graph manipulation, combined real multi-actor browser→BFF→Flowable journeys and final visual/motion review remain open.
- Two product choices (reject/return reasons and read-only reference-filter candidates) remain unanswered; independent work continues without inventing their decisions.

Local2015 pure/governance/contracts, typecheck/build, actual PR path scope and exact tracked archive native Gitleaks pass. Current exact CI still must finish; no overall product completion, merge or deployment claim.

## Verified checkpoint — 2026-10-13 10:23 UTC

Remote2529d2a372e31a0326ab6c66373bb049cf74b992 matches local2b3f6f95/tree6d71a2f3. CI38382045090/product38382045156 are running; repository governance SUCCESS. No merge or deployment.

- Exact854 verified node removal3, condition input geometry and OLD Agree terminal beforeunload regression GREEN. Its seven new deletion UI failures were preserved before implementing that UI.
- Exact0ee7e424 main CI38377033105 and governance SUCCESS; all four component shards plus coverage SUCCESS. Component4 job115228972979188/188, no skip/flaky; all seven deletion confirmation/pending/replay/actual unmount404/dirty/receipt/identity cases GREEN. Verified artifact11801694769 hash16a08b8da4b8738cbac06029a936e8e7c8756211458436f6eeaf949645af232d, confirmed screenshot personally inspected and preserved.
- The SAME0ee complete product job115232548592 FAILED: real browsers138PASS/3FAIL. The three browsers' common record-create/edit journey retained obsolete dialog locators after the source-approved P13/P7 full-page migration. Full raw and original test preserved. Current2529 only corrects three locators to named regions after rereading Figma483:2693 and Notion; all real persistence, decimal rounding, CAS, exact change body, reload, Shell identity and120-second budget stay. Actual rerun GREEN still pending; other successful layers do not hide this failure.
- P21 template file admission including lossless numeric tokens; preflight/201 receipt; exact request/read-only transport; immutable shared import recovery; typed source reference discovery all observed RED→GREEN. Current2529 contains20 first-run UI tests (export9/import first-slice11), WITHOUT their implementation. These are test-first pending, not delivered UI.
- Local after2529: two creation-scoped template-target candidate paths had actual contract RED then GREEN. Five new real HTTP suites compile but have NOT RUN with PG/Redis; no corresponding candidate backend implementation. The global creation scope has no fake target app, no borrowing personnel.manage or read-only filter rights; final mapping remains open.
- Latest local pure/governance/contracts2104PASS/0skip/fail; candidate contract lint0errors/14existing warnings, TypeScript/build passed before the subsequent contract/HTTP-only work. Native same tracked archive scan255.43MB was clean for2529 and is supplemental to CI.
- P19 deleted-record discovery/UI, personal started/approved lists, full template mappings/copy/structure lifecycle, approver search/departments, nonexecution node metadata/full graph manipulation, real multi-actor browser→BFF→Flowable journeys and final visuals remain open. Two unanswered product decisions remain open. No overall completion claim.

## 2026-10-13 11:58 UTC checkpoint (not delivery)
- Verified remote758b2069/locald25b9ec5: entire main CI38391842861 and repository governance passed. Full real Go115278217478 passed at the unchanged ten-minute budget, including all five new template candidate HTTP suites. The original raw Go report is preserved as deterministic gzip from artifact11809457848, SHA256729b2c56799eed757d2829a0bdb490def20e3be0bf897bfa189f2cf9d1d598c9.
- P21 original export9/import11 and six of seven typed mapping UI cases passed. Component3 199PASS/1FAIL: initial explicit picker issued two reads under StrictMode; cancellation was not reached. Original test/implementation/raw retained before a local mount-opening fix. Remote GREEN pending. Screenshot artifact11811206508 SHA256c9b3fb2958236f62c723a273f16c7f07af7ea211b61721de980b7fa0bf232b79 verified and actual mapping image reviewed; 740x540 outer geometry passes but stage heading/header require source-aligned fixes. New visual test is unexecuted; related layout is intentionally not changed before RED.
- Real full template download→typed mapping→new disabled application and prior record creation/edit full-page locator corrections remain NOT RUN because the product prerequisite gate stopped at the one component failure. Separate engine jobs do not prove that browser end-to-end journey.
- P19 minimal manager-only deleted discovery contract3RED→3GREEN; five real HTTP tests compile but are unexecuted, with no service implementation. Actual Monochrome Figma page478:1729 was inspected after metadata omitted it; current62 prototype pages do not include record deletion/restoration UI. No substitute design has been fabricated.
- Narrow same-environment Go incremental compilation cache contract2RED→4GREEN and full single-pass gate26GREEN; actual v2 Actions reuse is pending. Only GOCACHE is restored; identity includes runtime image/OS/arch/toolchain/dependencies/workflow and exact source suffix with one bounded fallback. No timeout, retries, workers, test-result cache, skip or security change.
- Local2113 pure/governance/contracts, typecheck/build/scope and 256.88MB native source secret scan passed. Entire remaining scope and both unanswered product decisions remain open. No develop/main merge or deployment.
