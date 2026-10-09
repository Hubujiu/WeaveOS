# V030-044 safe manual-start discovery and formal fixture race

## Source and scope
V044 PRD/ADR and P2c data bodies: seventh-slice plan appended and actually reread before implementation. Ordinary record users get only eligible creator-owned first-start choices, with six safe identity/CAS fields. No manager permission, graph, approvers, conditions or business values are exposed. Current Session/menu/actual row-field access is checked on every RR read, before stale-token errors. Noncreators see an empty list. Current enabled/deployed/manual/matching definitions with no history for this record are eligible; unrelated flows/records are independent.

New isolated workflow-manual-options query context binds actor/resource/Session. Stable flowId paging retains only a requested page, with a streamed actual-projection fingerprint and total. Changed projection needs an explicit refresh. Initial matching costs O(F) candidate/condition reads; memory is O(pageSize) plus bounded field/configuration facts. Each call has a 10-second maximum context; no production-throughput or zero-scan claim. No DDL or role change.

## Genuine sequence
- cf470b4: declaration-only service stub and eight independent PostgreSQL/Redis acceptance groups. `manual-options-service-red`: 8 top/12 sub behavior failures after fixtures succeeded, absent service behavior. Initial shell attempt was not executable (126); invoking the same script through bash worked. The shell error is not counted as RED.
- Service/isolated Store implementation, then `manual-options-service-green`: 8 top/12 sub PASS.
- e5c74b2 contains the now-tested service plus new HTTP/contract tests. `manual-options-http-red`: two valid real HTTPS requests get missing-route404. `manual-options-contract-red`: two missing route/schema failures. Then adapter/closed decoder/OpenAPI implemented; both sets GREEN.
- Additional cross-view, own-scope, empty read-field mask, namespace, no-business-write regression cases. A no-side-effect assertion initially queried nonexistent workflow_commands.app_id; fixed to the actual command_json AppID field, retaining the same app-scoped count assertions. Original failed final-regression is retained and is a test-fixture defect, not production RED.
- `manual-options-final-regression-2`: eight real PostgreSQL/Redis Go race packages, 551 top and 293 sub PASS, zero test failures/skips. Previous full-regression predates three extra service tests and lists appworkflows as a package with no test files; it is not the final eight-package run.
- Real public manual Java cases now discover the safe options through HTTPS and use the returned revision/schema/record CAS in the manual POST, preserving the real engine-start and committed-reply-loss assertions. `manual-options-java-final`: 3 genuine recovery and 7 public automatic/manual cases PASS, no failures/skips. This ten-case native selection differs from the ten-case CI actions selection, which includes three older action cases instead of recovery.

## Actual CI defect, not a weakened fence
Remote222ed9d4 CI37919905577: workflow-recovery and workflow-actions passed; actions explicitly reports 10 top scenes/12 identities. Formal-runtime failed its old EngineRestart setup with pending command fence. Original artifact11611940575 hash188ce8161361b4431a206be75d67bd562872f5498c257c0dafa804557afd3558 is retained.

The old existing-task fixture committed a starting reservation, then accepted a handcrafted start in a second transaction. The newly installed genuine background start admitter could correctly win that gap. The fixture now reserves and accepts its original start/fence in one transaction. No production logic, worker disabling, fence rule, timeouts or business assertions changed. Actual BFF/WorkflowEngineMain/HTTPS/receipt and all prior approval/Save/restart/close tests remain. Tagged full formal suite compiles locally; Docker formal runtime is NOT RUN locally and requires exact new-head CI GREEN.

## Other validation and remaining scope
Node518 PASS; vet, tagged compile, source format, repository structure, task scope, Gitleaks pass. OpenAPI lint uses telemetry/update checks disabled and retains 12 existing warnings. Own Go1.27.2, PG18.6, Redis8.2.10, JDK17; Node24.19.0 differs from repo24.14.0 pin. No user computer, delegated work, main merge or deployment.

This completes local trigger/discovery scope, not the whole form/approval backend. Rework/resubmit/re-review, independent history/inbox/deletion and other form-management gaps remain. User question Sentinel_c7168e68b3fc81919b41973269f5a9f0 about older rounds being history-only is pending, not adopted. Exact CI and review remain required before develop integration.
