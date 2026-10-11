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
