# ExecutionRegistry implementation checkpoint for Root review

Root accepted the compile-clean RED checkpoint `f637869f5ed2da18baf8a445d1c18a2bf47c4782` and explicitly authorized implementation of ExecutionRegistry, its bounded production codec and its independent gate. The immutable test source authority is `66b8862108b47bedf3dabe5f4998d7b46b9fb3b0`. Root tests, manifests, SQL fixture, BPMN and workflow wiring remain unchanged; the Go exporter differs only by previously authorized gofmt. The full ADR was re-read including the frozen 15:55 contract and independent CI section, last edited `2026-10-05T08:00:27.539Z`.

## Behavior

Request owns both input arrays and returns copies; Result owns a sorted immutable task list. The codec bounds every count/string/read, validates payload flags and exact SHA256 binding, and persists canonical complete result bytes and hash. Only the controlled graph's exact approval roster and condition-route sets are accepted. Actions receive only boolean routes; start additionally supplies the immutable controlled actor collections.

ExecutionRegistry requires the identical Spring transaction manager and underlying DataSource as Flowable. The REQUIRED transaction claims the command primary key before engine actions; equal bytes replay the original receipt, conflicting bytes throw. Instance row locks serialize actions; scope/definition checks precede terminal, sequence, fence, versions and task/epoch/actor checks. No-effect stores a finite unchanged receipt without updating the instance, task or engine projection. Infrastructure faults throw and roll back. Internal cancellation competes on the same command key and preserves the first durable outcome.

Partial MI completion preserves surviving task UUIDs and epochs. Newly activated nodes receive fresh UUIDs and a new visit epoch; cancelled runtime tasks become invalidated, while completed decisions remain. Return uses the tested top-level activity-ID migration, including historical task ownership/epoch rules. Reject follows the controlled rejection branch; withdrawal requires the initiator and frozen configuration and retains history. Every effect and receipt shares the transaction, including the three injection points and outer rollback.

The gate fixes both class/name sets in production code, validates the supplied manifest against that frozen contract, and requires the exact two reports and 38 identities, zero failure/error/skip, correct counters and direct testcase children; malformed XML and DOCTYPE are rejected.

## Actual verification

| Check | Actual result |
| --- | --- |
| Final real execution33 + compatibility5 | exit0, 38/38, zero failure/error/skip; execution JUnit18.342s, compatibility3.027s, Maven24.887s |
| New synthetic gate16 | exit0, 16/16, 0.009s |
| Actual final 38-case XML gate | exit0, exact38 |
| Legacy real deployment18 / envelope8 / RPC9 | exit0, 35/35, zero failure/error/skip; JUnit8.744s / 0.046s / 4.196s, Maven14.977s |
| Real Go→Java→Flowable→PostgreSQL interop | exit0, genuine test pass0.370s, package0.384s |
| Original strict gates | deployment18, envelope8, RPC9+interop1 accepted; original synthetic gates12/12/20 passed |
| Governance/foundation | exit0, 240/240, zero skipped, 2.843s |

Commands and original complete output are preserved in `final-test.log`, `legacy.log`, `interop.log`, genuine `interop.jsonl`, gate logs and corresponding exit files. Actual XML and text reports are under `execution/` and `legacy/`; per-suite copies used with the unchanged legacy gates are retained separately. The first successful 38 run is also preserved in `test.log`; final evidence corresponds to the final production code snapshots under `source/`. No timing is a production latency, memory or throughput estimate. The old proof24 was unchanged and retains its earlier independent A GREEN evidence; it was not rerun for this Java-only slice.

Commands:

```sh
bash services/workflow-engine/run-tests.sh -Dtest=RootExecutionRegistryTest,RootReturnCompatibilityTest -Dworkflow.reports=/proof/.work/execution-green-reports test
python3 services/workflow-engine/execution_ci_gate_tests.py
python3 services/workflow-engine/execution_ci_gate.py services/workflow-engine/.work/execution-green-reports services/workflow-engine/expected-execution-tests.json
bash services/workflow-engine/run-tests.sh -Dtest=RootDeploymentRegistryTest,RootCommandEnvelopeTest,RootDeploymentGrpcIT -Dworkflow.reports=/proof/.work/execution-legacy-reports verify
GOMODCACHE=/workspace/.weaveos-tools/go-mod GOCACHE=/workspace/.weaveos-tools/go-cache WEAVEOS_RPC_GO=/workspace/.weaveos-tools/go/bin/go WEAVEOS_RPC_JSON_REPORT=.work/execution-green/interop.jsonl bash services/workflow-engine/run-rpc-interop.sh
node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs
```

Locked offline Maven/Temurin17, Flowable8 and PG18.6 fixture tools/caches were reused. All real engine tests ran on internal Docker fixture networks without published ports; captured residual fixture container/network listings are empty. No dependencies, POM, proto, old gates, Go BFF behavior, frontend, production credentials/network or migrations changed.

## Review boundary

Root implementation review is next. Complete exact-head remote CI awaits that review. This is an isolated engine execution slice with a contractual test SQL fixture; no formal service bootstrap, public HTTP/RPC execution endpoint, business-record projection or BFF authorization integration was added. It is not deployable/full-chain delivery. Nothing merged to main or deployed. A worktree remains at exact530 and the prior return preflight worktree remains clean at c226602.

RED native ZIP: `file_00000000c29c81f99313505da19625b8`, SHA256 `d10b756ad9d5975990cc5f559707bba94b92c007a1ff614460c4aae491fbfa0d`.
