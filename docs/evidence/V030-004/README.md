# B3 proof evidence

Source oracle: PRD/ADR009 B3 finite PLAN (read after root released documentation gate, last edits 2026-10-02 13:14:33.192 / 13:14:39.459 UTC). Only independent engine service local transaction proof, not distributed product acceptance.

Baseline: remote main 6ffba4d41cb7ee1c70a862e12a9bd9ca88be77a5; PR21 head 74cc5824ee4336dd76c72874f0cc4cf38fe9e889 OPEN, all five current checks SUCCESS. No changes to PR21/main/production.

The first attempt (`red-attempt-1.log`) failed resolving a parent POM because the fixture container had no direct egress. This is infrastructure failure, NOT a valid RED. No command behavior had been implemented at that stage.

`red-attempt-2.log` could not load the offline Surefire provider; not RED. `red-attempt-3.log` reached 12 target failures, but two concurrent-case helper errors rejected placeholder null results before their assertions. The helper now retains null so those same expected assertions are reached; no expectation was changed.

Effective original RED: 2026-10-02T13:35:41Z, `prototypes/flowable-local-tx/run-proof.sh verify`, exit 1, 15 tests / 14 target failures / 0 errors / 0 skipped. The one passing test proves fixture wiring, not command behavior. `red-source.tar.gz` preserves the exact no-behavior implementation and tests before GREEN; `red-source.sha256` and `test-source.sha256` identify bytes. `red.log` and `red-junit.xml` retain original execution results.

Acceptance map (test method names):

| B3 goal | Independent assertion |
| --- | --- |
| Same underlying DataSource / transactionManager, REQUIRED | bootAndEngineUseTheSameUnderlyingDataSourceAndTransactionManager checks bean identities and identical physical PostgreSQL connection |
| All three stores commit atomically | completeCommitsLedgerEngineAndOutboxTogether; pureDefaultConditionEndsTheProcess |
| Three failure points roll back | failureBeforeCommitRollsBackAllThreeStores checks ledger, outbox, task, history, runtime, and variable state |
| Join caller REQUIRED | completeJoinsAnOuterRequiredTransaction rolls back after observing all local writes |
| Deduplication / payload conflict | duplicateCommandReplaysExactReceiptWithoutCompletingNextTask; sameIdWithDifferentPayloadIsAConflict; sameIdCannotChangeTaskOrConditionDespiteIdenticalOpaquePayload |
| Real concurrency | simultaneousIdenticalCommandsAdvanceOnce; simultaneousConflictingCommandsHaveOneWinnerAndOneConflict |
| Commit then response loss | responseLossAfterCommitReplaysAfterEngineRestart closes and recreates engine/Boot context against surviving PostgreSQL |
| Retry rollback / no fake success on task absence | rolledBackCommandIdCanBeRetried; missingTaskDoesNotInventSuccessOrPersistAReceipt |

Full fault matrix, two-database atomicity, application visibility/fence/inbox/audit, cancellation tombstones, engine transition sequence, external transport, actual OS process kill and backup restoration remain outside this B3 local proof. These are NOT RUN, not passed by the local tests.
