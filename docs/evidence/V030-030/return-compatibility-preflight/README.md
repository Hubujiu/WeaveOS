# Root return compatibility preflight · 2026-10-05 UTC

Base exactly530d877e2d05430690ede608078276e1582e389d. Independent temporary worktree /workspace/WeaveOS-worktrees/V030-030-return-preflight, branch task/V030-030-return-compatibility-preflight. Root supplied both tests in the delegation; only transport HTML entities were decoded and the Go source was gofmt formatted. No Root assertion was modified. Updated ADR fetched in full, last edited2026-10-05T07:29:55.181Z. No ExecutionRegistry, business/gate/dependency/config/migration/old-test changes. No artificial RED, full CI, push or PR.

The locked Go1.27.1 export test actually called existing CompileBPMN. Exit0, one named test pass, package0.004s. Artifacts are src/test/resources/return-compatibility/return-all.bpmn20.xml and return-any.bpmn20.xml. Existing pinned Maven3.9.11 / Temurin17.0.17+10 release17 / PG18.6 / Flowable8.0.0 used offline. Java compile exit0, Maven2.352s. Real Java tests exit0, five pass, zero failure/error/skip; JUnit5.977s, Maven7.550s.

Commands from temporary worktree root:

```sh
WEAVEOS_RETURN_BPMN_OUTPUT=/workspace/WeaveOS-worktrees/V030-030-return-preflight/services/workflow-engine/src/test/resources/return-compatibility GOMODCACHE=/workspace/.weaveos-tools/go-mod GOCACHE=/workspace/.weaveos-tools/go-cache /workspace/.weaveos-tools/go/bin/go -C services/bff test -count=1 -json -run '^TestRootExportReturnCompatibilityFixtures$' ./internal/flowgraph
bash services/workflow-engine/run-tests.sh -Dtest=RootReturnCompatibilityTest -Dworkflow.reports=/proof/.work/return-compatibility-reports test-compile
bash services/workflow-engine/run-tests.sh -Dtest=RootReturnCompatibilityTest -Dworkflow.reports=/proof/.work/return-compatibility-reports test
```

Numbers below are values checked by the Root assertions that actually passed, not separately printed engine snapshots. Actor numbers denote Root id(n), e.g. id(8)=00000008-0000-4000-8000-000000000008; hexadecimal formatting applies to all n. MI triples are nrOfInstances / nrOfActiveInstances / nrOfCompletedInstances.

| Actual case | Before → movement/delete → resulting checks | Time |
| --- | --- | --- |
| returnFromSecondMiRecreatesVisitedRosterAndResetsCounters(String)[1], all | Second node3 tasks for10/11/12; after10 completes2 tasks11/12. Return creates2 first-node tasks8/9, MI2/2/0 and approver=assignee. Old first/second task IDs absent; old second IDs cannot complete. Complete8 leaves1 task9 with completed counter1; complete9 recreates3 second-node tasks10/11/12 with fresh IDs. First-node finished history4; final tasks/process0. | 1.173s |
| same [2], any | Second-node3 tasks10/11/12 → first-node2 tasks8/9, MI2/2/0. Same old-ID/history/re-entry checks as all. On the new second activation complete10 ends instance and leaves0 tasks. | 0.977s |
| returnToCurrentPartiallyCompletedNodeStartsANewActivation | First-node2 tasks8/9 → complete8 leaves1 task9 → return to same node creates2 tasks8/9, MI2/2/0 and fresh IDs → complete both produces3 second-node tasks10/11/12. | 0.675s |
| rollbackAfterMovementRestoresExactPriorMiTasks | Second-node2 tasks11/12 → inside transaction first-node2 tasks8/9 with MI2/2/0 → injected exception → exact original second-node IDs restored,2 tasks11/12 and unchanged historic task count. Complete11 leaves1 task12, complete12 ends with0 tasks. | 0.780s |
| withdrawalRollsBackThenRemovesAllRuntimeTasksButRetainsHistory | Second-node2 tasks11/12 → transaction delete leaves0 → injected exception restores exact prior IDs → committed delete leaves0 runtime tasks/processes, ended historic instance retained with deleteReason=root-fixture-withdraw and5 historic tasks. | 2.315s |

These tests do not print literal engine task IDs or directly inspect MI counters before movement/after rollback; those values are not fabricated here. Freshness/equality and the above reset counters are actually asserted. No failure stack exists: all five cases passed. Java helper/case source locations: counters88–91, second-node return100–119, same-node121–127, rollback129–139, withdraw141–153.

Each test engine closed and its random schema was dropped by Root @AfterEach without errors. Existing run-tests.sh exit traps removed both commands' private PostgreSQL/Maven containers and internal networks; no published ports. Final label-filtered container/network lists are empty. Shared caches were preserved; only this worktree's copied toolchain/cache and disposable build files remain for possible reproduction. Raw logs, XML/text reports, genuine Go JSON, exit codes and source/artifact hashes are saved here. PR40 worktree stays clean at530d877. This compatibility experiment does not implement product return or an execution registry.
