# Root review — recoverable workflow deletion

2026-10-10 04:04 UTC, Root alone; no delegated coding. PRD, ADR and Data fetched in full again04:02. Source identities and boundaries are unchanged; Data last edited03:29. This report is a local reviewed stage, not final exact-head CI approval.

## Scope and dependency identity

Remote checkpoint e56e1a7c149151dcd87e4314b11515c74e2962e4 has tree ed29f218f58edf8547904293ac2746fc7fdbbf14, confirmed by actual fetch. Actual develop22ddd50b contains merged PR74. V064 d770e693 is still a tested PR candidate, not yet merged; V065 published stage2cfbd93c is not yet a PR. Normal merges preserve test-first history. All services/contracts/db/infra/.github bytes remain identical to tested1994edab after upstream integration; executable-identity.patch is empty. Against latest V065 stage the task changes43 executable/test files,3423 additions and29 removals; no Java production implementation change.

## Requirement and independent evidence mapping

- D01: TestRootDeletionHTTPAcceptedReplayAndCommittedStatus, RejectsInvalidBodyWithoutWrites, CurrentIdentityAndOwnership, OperationNamespaceAndChangedScope, RejectsStaleScopeAndUnavailableRuntime. Real Session/CSRF/manager checks, immutable original operation acceptance, current authoritative deletion status, cross-actor/scope rejection. RuntimeProbeHasNoDomainLocks independently observed NOWAIT failure before the fix. SafeRevisionExhaustionIsConflict independently observed400 before the409 fix. Dedicated OpenAPI request bounds preserve old other lifecycle schemas.
- D02: acceptance, schema binding and identity guards; ConcurrentNewVersionWaitsForAcceptance uses actual two-connection blocking, configuration HTTP and catalog Enable reject even after normal closing finalization. Domain/DB barriers preserve permanent global flow UUID. No ordinary version UPDATE/DELETE privilege added.
- D03: WaitsForAcceptedPublication, WaitsForPendingStartCommand, HistoryDrainsActiveTaskAndKeepsOriginalEvidence and actual formal process deletion. New admission is closed atomically; accepted pending/unknown publication and starting/active/task/command work drain before engine RPC. Existing approvals remain usable.
- D04: LostReplyWithOriginalLookup, StaleLeaseCannotCommit, ProcessLoopRecoversAndWaitsForCancellation and TestRootFormalRuntimeDeletionDrainsAndRecoversOriginalReceipt. Actual authenticated Go to Java/Flowable/PG commits deletion, loses the transport response, restarts both services, then resumes the original operation without a second Delete RPC. Random45-second lease and at-most30-second RPC; no database transaction held over RPC. Process owner waits for third worker cleanup and cancels peers on unexpected exit.
- D05: CleanupFailureRollsBackAndRecovers and the two actual-history tests verify terminal task/instance removal while original event/evidence/command/receipt/publication rows and manual-start result stay unchanged. Complete operation, audit and physical configuration cleanup are one application transaction; the cross-database protocol is recoverable, not falsely described as distributed atomicity.
- D06: CleanupCapabilityRequiresBoundConfirmedLease, LeastPrivilegeAndFixedSearchPath, SchemaAndRuntimeBoundary and real backup-restore. Fixed pg_catalog and qualified SQL, PUBLIC execute revoked, runtime gets only bounded new columns and explicit cleanup capability. Nonempty hot/cold Down refuses; empty Up/Down/Up passes. Backup copies three synthetic marker states byte-for-byte; this is not claimed as actual engine execution. Real maintenance archive test preserves the original completion event summary bytes.

## Verification and limitations

Fresh full ten-package PG18.6/Redis8.2.10 Go-race:770 top-level and607 subtests passed, zero failed, three existing opt-in capacity probes skipped. Final Node545 passed with no skips, OpenAPI0 errors/13 existing warnings, all Go vet/build passed. Current exact e56e Git archive Gitleaks8.30.1 exit0, zero leaks. Old hot1..28/cold1..6 and original roles prefix are immutable; existing evidence includes SHA checks.

Formal process group: first full10 cases passed; repetition overlapping the large race run failed two pre-existing Java startup/readiness scenarios. Cause is not established. After the large run ended, both repeated three times passed, then the complete unchanged10-case group in formal-final-serial passed. Original failures and serial reruns remain retained, with no timeout, assertion or skip changes. Future exact CI must pass this mandatory gate again; local reruns do not erase the original failures.

Backward compatibility with old approval binaries has not been proven. Manifest explicitly false; original package.mjs, receiver and main delivery are unchanged and reject promotion. PR helper requires original exact compatibility rejection and absence of any promotable bundle; arbitrary Docker/build errors fail. Full product/runtime/backup/restore/rollback/security stages remain mandatory. Actual Docker same-artifact packaging refusal is NOT RUN locally and must be established by final CI. No deployment, main merge or real-user data deletion occurred.

## Remaining acceptance

Integrate actual V064/V065 squash commits, verify exact source identity, finalize task/PR metadata, run final candidate CI including actual container product/refusal, review failures if any and only then squash into develop. Latest-round rework/resubmit/review policy remains explicitly unanswered and outside this implementation.
