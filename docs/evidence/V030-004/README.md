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


Original GREEN: 2026-10-02T13:37:43Z, the same `run-proof.sh verify`, exit 0, 15/15 passed, no errors or skips. Suite 9.366s; complete verify 11.917s. `green.log` / `green-junit.xml` preserve original output. `test-source.sha256` was checked after GREEN and remained identical to effective RED.

`resolved-dependencies.txt` records actual mediation under Boot4.0.2; `artifact-sha256.txt` records all 92 test classpath artifacts, 26,160,520 bytes total. `bytecode-and-dependency-footprint.txt` verifies class major61 (Java17). Shell syntax, source/document diff whitespace (excluding unmodified raw Maven logs and dependency command output with tool-generated trailing spaces), repository structure and existing task governance passed; those structural checks do not prove formal V030 task integration or product acceptance.

RED declarations/tests commit: f5ad180b869f9f7cf0b1a3bee7c8ad5e6d609823; original logs commit: 4d4c06899c6ca7899dc44a7f35d104416b632fcb. Neither is a reconstructed RED. Source snapshots can be replayed separately; such runs must be labeled REPLAY.


Stacked integration (2026-10-02): root/user explicitly authorized merging B0 and creating draft PRs. Non-destructive merges consumed B0 through `5571420c8a50fe41909d75790509b9e5b6b5dbd5`; no B0-owned source was edited by B3. Draft PR24 uses the B0 branch as base; formal task metadata is now in `docs/tasks/V030-004.md`. Java15/15 (exit0, 14:26:11Z), governance182/182 and actual PR24 scope passed locally. `stacked-final-java-verify.stdout.json` preserves exact original stdout bytes as base64 with SHA256; `stacked-final-java-junit.xml`, `stacked-final-governance-182.txt` and `stacked-integration-record.json` retain rerun evidence. These are integration reruns, not reconstructed original TDD. Java CI remains unwired; common GitHub CI results do not prove remote Java execution. Final post-push head checks are reported separately.
