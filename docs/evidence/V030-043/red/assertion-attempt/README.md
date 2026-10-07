# Actual frozen storage assertion run

Source tests/declaration: `85c471d9a733f1c08ecacc7913f2e761cc5b633c`; execution HEAD `591e6e63660596faf910cc01535ec8f28b93c406` differs only by the preserved first dependency-blocked attempt.

Root explicitly authorized only normal `go mod download` through `https://proxy.golang.org` with `sum.golang.org` verification. Download exited0; go.mod/go.sum hashes unchanged. No get/update/tidy/version substitution or verification disabling. This assertion run uses offline GOPROXY after successful download; sum.golang.org verification remains enabled.

Existing unique task PG/Redis were checked healthy and reused. No containers were recreated. Original migrated schema/role grants and real fixture code remain unchanged. Frozen test/declaration hashes match before and after execution.

Actual command: `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowSave' ./internal/apprecordservice`. Go1.27.1, CGO enabled. Exit1;18top-level tests entered,17FAIL/1PASS/0SKIP; both basis subcasesFAIL. All reported failures are assertions in root_workflow_save_test.go after fixture setup. Empty stderr; no race diagnostic. Full stdout JSONL is preserved byte-for-byte.

The17failures establish absent Save semantics (the declaration returns ErrUnavailable) and absent preview editableFieldIds. They do not prove implemented commit/concurrency/permission behavior. HistoryFailureRollsBackTypedRowAndOperation passes because its generic non-nil-error check accepts the declaration's ErrUnavailable while unchanged-row/count checks hold; this is not evidence that the injected history failure was exercised. Root owns any oracle refinement; executor did not alter it.

| Test suffix (prefix TestRootWorkflowSave) | Result | Original assertion output |
| --- | --- | --- |
| WritesOriginalHistoryButNeverAdvancesTask | FAIL | root_workflow_save_test.go:92: record service unavailable |
| ReadOnlyNodeCannotWrite | FAIL | root_workflow_save_test.go:115: readonly node: record service unavailable |
| CannotEditOutsideNodeWhitelist | FAIL | root_workflow_save_test.go:126: mixed forbidden field: record service unavailable |
| CurrentFieldPermissionStillRequired | FAIL | root_workflow_save_test.go:137: node did not preserve revoked edit: record service unavailable |
| OtherActorCannotUseCurrentTask | FAIL | root_workflow_save_test.go:149: foreign actor: record service unavailable |
| StaleRecordBasisRejectsWholeMutation | FAIL | root_workflow_save_test.go:161: stale basis: record service unavailable |
| TaskSequenceChangeRequiresRefresh | FAIL | root_workflow_save_test.go:172: task changed: record service unavailable |
| ExpiredAndCrossSessionBasis/expired | FAIL | root_workflow_save_test.go:189: basis expired: record service unavailable |
| ExpiredAndCrossSessionBasis/session | FAIL | root_workflow_save_test.go:189: basis session: record service unavailable |
| ExpiredAndCrossSessionBasis | FAIL | parent failed because both basis subcases failed |
| IdenticalReplayAndConflictingPayload | FAIL | root_workflow_save_test.go:199: record service unavailable |
| OldApprovalBasisCannotApproveNewValue | FAIL | root_workflow_save_test.go:216: record service unavailable |
| PendingCommandFenceRejectsSave | FAIL | root_workflow_save_test.go:238: pending command did not fence save: record service unavailable |
| ConcurrentWritersHaveOneActualVersion | FAIL | root_workflow_save_test.go:264: unexpected race error: record service unavailable |
| ActualCommitRollbackIsAtomic | FAIL | root_workflow_save_test.go:280: rollback not preserved {OperationID: ID: RecordVersion:0 SchemaVersion:0 CreatedAt: UpdatedAt:} record service unavailable |
| LostCommitReplyReplaysOneRealSave | FAIL | root_workflow_save_test.go:293: lost commit reported as certain {OperationID: ID: RecordVersion:0 SchemaVersion:0 CreatedAt: UpdatedAt:} record service unavailable |
| HistoryFailureRollsBackTypedRowAndOperation | PASS | non-nil declaration error plus unchanged actual row/counts accepted |
| PreviewExposesOnlyEditableVisibleIntersection | FAIL | root_workflow_save_test.go:333: editable intersection: [] |
| DefinitionRevisionDoesNotChangeInflightWhitelist | FAIL | root_workflow_save_test.go:344: record service unavailable |
| DoesNotDependOnEngineReadiness | FAIL | root_workflow_save_test.go:352: record service unavailable |

Original environment-failure artifacts in the parent folder remain unchanged. All submitted log files were scanned for credential/DSN/private-path markers; no redaction was needed. Public run metadata omits private socket/container/cache paths, while complete original metadata remains in the private task run directory. The raw/submitted hashes are recorded in run.json.

No implementation/test/fixture/shared script was changed. No main/PR/deployment action. Task-only resources retained for Root's authorized next run. Waiting for Root review and a scoped implementation contract.
