# Root ExecutionRegistry contract: compile-clean RED

This is a test-only checkpoint. ExecutionRegistry and execution_ci_gate remain Root's behaviorless declarations. Implementation awaits Root review and explicit continuation. No full CI, push, PR, production migration, frontend, deployment, dependency or network configuration changes were performed for this checkpoint.

## Provenance

- Frozen Stage A base: `530d877e2d05430690ede608078276e1582e389d`.
- Independent worktree: `/workspace/WeaveOS-worktrees/V030-030-engine-actions`; branch `task/V030-030-engine-actions`.
- Return compatibility preflight was cherry-picked from local `c226602d1a4c690108800906ed14c74fe0406bb2` as `80216da` before this phase.
- Root's authoritative unified test commit: `66b8862108b47bedf3dabe5f4998d7b46b9fb3b0`, parent exact Stage A base. Every added Java/Python/JSON/SQL/BPMN/workflow file is byte-identical to that commit. The Go exporter differs only by gofmt; full comparison hashes are in `root-source-comparison.json`.
- Full applicable Notion ADR re-read before applying the execution contract, last edited `2026-10-05T07:55:25.548Z`; no inaccessible or truncated contract content. Root authors all tests and contract files.

## Actual commands and outcomes

From the independent worktree:

```sh
bash services/workflow-engine/run-tests.sh -Dtest=RootExecutionRegistryTest -Dworkflow.reports=/proof/.work/execution-red-reports test-compile
bash services/workflow-engine/run-tests.sh -Dtest=RootExecutionRegistryTest -Dworkflow.reports=/proof/.work/execution-red-reports test
python3 services/workflow-engine/execution_ci_gate_tests.py
```

- `compile.log` / `compile.exit`: successful separate test-compile, exit 0, Maven 2.533 seconds, completed `2026-10-05T08:06:23Z`.
- `test.log` / `test.exit`: actual RED, exit 1, 33 tests, 5 assertion failures, 27 errors, 0 skipped; JUnit 13.140 seconds, Maven 14.565 seconds, completed `2026-10-05T08:06:56Z`.
- The one passing Java case is `missingLookupNeverCreatesAReceiptOrCancellationProof`. The 27 errors are unimplemented execution calls, including concurrent Future wrappers. Three start injection cases and malformed-payload rejection expected specialized exceptions but received the declared unsupported-operation exception. The wiring case expected constructor rejection but the declaration's empty constructor accepted it. All failures represent missing behavior; no compile, SQL fixture, deployment or Flowable initialization failure was observed.
- Actual Java class/name set matches all 33 frozen manifest names exactly. Original Surefire XML/text and case summary are retained without alteration.
- `gate-tests.log` / `gate-tests.exit`: actual Python RED, exit 1, 16 tests in 0.006 seconds; 15 rejection cases pass; the sole 38-case positive test errors with `GateError("not implemented")`. Passing rejection tests on an always-rejecting declaration do not establish correct validation behavior.

## Isolation and scope

The unchanged locked runner used cached offline Maven dependencies, release 17, the fixed Maven/Temurin image, Flowable 8.0.0 and PostgreSQL 18.6. The real database ran on an internal Docker fixture network with no published ports. Runner cleanup removed its containers/networks; captured remaining fixture listings are empty. Shared caches were retained. Test durations measure these fixtures, not production performance or theoretical complexity.

Root's workflow change is exactly seven added lines: independent execution gate tests, the exact 33+5 execution selection, strict gate invocation and artifact directory. Existing deployment18, envelope8, RPC9, gates, timeouts, dependencies and other workflow steps are preserved. Those regressions were not rerun in this RED-only phase; the previously recorded compatibility5 GREEN remains a separate preflight result.

Root must review this RED before either ExecutionRegistry or its report gate is implemented. Stage A and this contract checkpoint do not complete approval or the full backend chain.
