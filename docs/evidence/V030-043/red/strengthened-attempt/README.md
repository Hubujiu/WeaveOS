# Strengthened frozen V030-043 assertion run

Exact source `d30da6bcd4faaf5d3aee18d7b8509e8aec2dbb5e`, parent `2bd3d26dcf01ad464814206a21534187ce3711ee`, tree `03bddaaa7a9d1924224a41cf6e852ab78be278f0`. Fast-forwarded unchanged; no implementation/test/fixture/shared-script changes.

Same existing isolated real PG/Redis resources were checked healthy and reused; no duplicate resources. Official locked dependency download from the previous run remains cached; go.mod/go.sum and all four frozen test/declaration file hashes remain unchanged. Go1.27.1, CGO enabled, race detector, local toolchain, GOPROXY=off and GOSUMDB=sum.golang.org.

Commands run sequentially: `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowSave' ./internal/apprecordservice`, then `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowSaveHTTP' ./cmd/bff`. Both exit1 after real fixtures initialize. Service18FAIL/0PASS/0SKIP, basis2FAIL. HTTPS top3FAIL/1PASS/0SKIP, auth-boundary4subcasesPASS and closed-body7subcasesFAIL. Both stderr streams are empty; no race diagnostic or environment/listen failure.

Strengthened history-fault case now FAILS at the independent sequence-state assertion: `history fault was not actually reached: <nil>`; the prior insufficient control PASS is preserved in its historical folder and is not reused as coverage.

HTTPS uses Root's unchanged real local httptest listener and TLS client, without network-policy changes. SaveAndGenericReceipt stops at missing preview editable IDs; closed-body cases receive404/API_NOT_FOUND rather than400 validation; read-only node receives404 rather than403. Session/CSRF/origin/expected-actor controls pass through existing admission. These are business assertions, not an environment failure.

`node --test tests/foundation/root-node-save-http-contract.test.mjs`: exit0,3PASS/0FAIL/0SKIP; proves registration contract only, not HTTP implementation.

## service

| Exact test | Result | Original assertion output |
| --- | --- | --- |
| TestRootWorkflowSaveWritesOriginalHistoryButNeverAdvancesTask | FAIL | root_workflow_save_test.go:92: record service unavailable |
| TestRootWorkflowSaveReadOnlyNodeCannotWrite | FAIL | root_workflow_save_test.go:115: readonly node: record service unavailable |
| TestRootWorkflowSaveCannotEditOutsideNodeWhitelist | FAIL | root_workflow_save_test.go:126: mixed forbidden field: record service unavailable |
| TestRootWorkflowSaveCurrentFieldPermissionStillRequired | FAIL | root_workflow_save_test.go:137: node did not preserve revoked edit: record service unavailable |
| TestRootWorkflowSaveOtherActorCannotUseCurrentTask | FAIL | root_workflow_save_test.go:149: foreign actor: record service unavailable |
| TestRootWorkflowSaveStaleRecordBasisRejectsWholeMutation | FAIL | root_workflow_save_test.go:161: stale basis: record service unavailable |
| TestRootWorkflowSaveTaskSequenceChangeRequiresRefresh | FAIL | root_workflow_save_test.go:172: task changed: record service unavailable |
| TestRootWorkflowSaveExpiredAndCrossSessionBasis/expired | FAIL | root_workflow_save_test.go:189: basis expired: record service unavailable |
| TestRootWorkflowSaveExpiredAndCrossSessionBasis/session | FAIL | root_workflow_save_test.go:189: basis session: record service unavailable |
| TestRootWorkflowSaveExpiredAndCrossSessionBasis | FAIL | parent inherits failing subcase(s) |
| TestRootWorkflowSaveIdenticalReplayAndConflictingPayload | FAIL | root_workflow_save_test.go:199: record service unavailable |
| TestRootWorkflowSaveOldApprovalBasisCannotApproveNewValue | FAIL | root_workflow_save_test.go:216: record service unavailable |
| TestRootWorkflowSavePendingCommandFenceRejectsSave | FAIL | root_workflow_save_test.go:238: pending command did not fence save: record service unavailable |
| TestRootWorkflowSaveConcurrentWritersHaveOneActualVersion | FAIL | root_workflow_save_test.go:264: unexpected race error: record service unavailable |
| TestRootWorkflowSaveActualCommitRollbackIsAtomic | FAIL | root_workflow_save_test.go:280: rollback not preserved {OperationID: ID: RecordVersion:0 SchemaVersion:0 CreatedAt: UpdatedAt:} record service unavailable |
| TestRootWorkflowSaveLostCommitReplyReplaysOneRealSave | FAIL | root_workflow_save_test.go:293: lost commit reported as certain {OperationID: ID: RecordVersion:0 SchemaVersion:0 CreatedAt: UpdatedAt:} record service unavailable |
| TestRootWorkflowSaveHistoryFailureRollsBackTypedRowAndOperation | FAIL | root_workflow_save_test.go:320: history fault was not actually reached: <nil> |
| TestRootWorkflowSavePreviewExposesOnlyEditableVisibleIntersection | FAIL | root_workflow_save_test.go:341: editable intersection: [] |
| TestRootWorkflowSaveDefinitionRevisionDoesNotChangeInflightWhitelist | FAIL | root_workflow_save_test.go:352: record service unavailable |
| TestRootWorkflowSaveDoesNotDependOnEngineReadiness | FAIL | root_workflow_save_test.go:360: record service unavailable |

## http

| Exact test | Result | Original assertion output |
| --- | --- | --- |
| TestRootWorkflowSaveHTTPRealHostSaveAndGenericReceipt | FAIL | root_workflow_save_http_test.go:20: preview did not expose exact writable intersection |
| TestRootWorkflowSaveHTTPPreservesSessionCSRFAndActorBoundaries/session | PASS | unchanged admission control assertions pass |
| TestRootWorkflowSaveHTTPPreservesSessionCSRFAndActorBoundaries/csrf | PASS | unchanged admission control assertions pass |
| TestRootWorkflowSaveHTTPPreservesSessionCSRFAndActorBoundaries/origin | PASS | unchanged admission control assertions pass |
| TestRootWorkflowSaveHTTPPreservesSessionCSRFAndActorBoundaries/actor | PASS | unchanged admission control assertions pass |
| TestRootWorkflowSaveHTTPPreservesSessionCSRFAndActorBoundaries | PASS | unchanged admission control assertions pass |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges/unknown | FAIL | root_workflow_save_http_test.go:82: want400/COMMON_VALIDATION_FAILED actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"b2f32500ac2a7d29a432135f60ad11c2"}} |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges/duplicate-operation | FAIL | root_workflow_save_http_test.go:82: want400/COMMON_VALIDATION_FAILED actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"d33316b5eedfa435b06e7556c4bd01bc"}} |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges/empty | FAIL | root_workflow_save_http_test.go:82: want400/COMMON_VALIDATION_FAILED actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"c8991b6e0c93b2c737639f3960e8640d"}} |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges/null | FAIL | root_workflow_save_http_test.go:82: want400/COMMON_VALIDATION_FAILED actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"77a0ec4c1758b33e1b09f3352487528a"}} |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges/number | FAIL | root_workflow_save_http_test.go:82: want400/COMMON_VALIDATION_FAILED actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"dcc70e0f2a21cd0c742901e559e8aeea"}} |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges/duplicate-field | FAIL | root_workflow_save_http_test.go:82: want400/COMMON_VALIDATION_FAILED actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"ce796b8ccfdba066bc7f9e03105955d2"}} |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges/extra-query | FAIL | root_workflow_save_http_test.go:82: want400/COMMON_VALIDATION_FAILED actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"d293c1f7b6410fd896eaba4a61a510b2"}} |
| TestRootWorkflowSaveHTTPClosedBodyAndNonemptyChanges | FAIL | parent inherits failing subcase(s) |
| TestRootWorkflowSaveHTTPReadonlyApprovalNodeDenied | FAIL | root_workflow_save_http_test.go:90: want403/APPLICATION_FORBIDDEN actual404 {"code":"API_NOT_FOUND","message":"请求未完成","data":null,"meta":{"requestId":"aa63422e41596445c8350422ce3c6d4d"}} |

All raw JSONL/stdout/stderr/exit streams are preserved byte-for-byte after review for credentials, DSNs and private machine paths; submitted hashes are in run.json. Private original connection metadata remains outside Git. Both prior attempts remain unchanged.

No source/test/fixture repair or implementation performed. Root owns interpretation and release of Save/Preview and HTTP scopes. Task resources retained for pending authorized runs; no main/PR/deployment action.
