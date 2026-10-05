# Root review: recovery ordering and strict UTF-8 replay

Root reviewed `dbfc7f4c0266e0cbe9a21cbd2ef187fe4a05aed9`, identified contextual node validation running before recovery/identity rejection and an ASCII-only durable result reader, and provided exact test commit `4086e8978db50957fcb0c43a46af6c538603fbde` (parent dbfc7f4). It was fetched and cherry-picked as `b0c1486100f43413dfd13869839c264d8c040666`. All four Root files are byte-identical to that authority; original 33 assertions, SQL/BPMN, old gates and dependencies are unchanged. The full Notion ADR was re-read, including the 16:37 review supplement, last edited `2026-10-05T08:36:59.104Z`.

## Actual RED before implementation

```sh
bash services/workflow-engine/run-tests.sh -Dtest=RootExecutionRegistryTest -Dworkflow.reports=/proof/.work/execution-review-red-reports test-compile
bash services/workflow-engine/run-tests.sh '-Dtest=RootExecutionRegistryTest#internalCancellationDoesNotDependOnLiveGraphNodeContext+identityFailuresPrecedeContextualNodeValidation+durableResultCodecPreservesBoundedUtf8OpaqueIds' -Dworkflow.reports=/proof/.work/execution-review-red-reports test
python3 services/workflow-engine/execution_ci_gate_tests.py
```

Separate compilation succeeded, exit0, Maven1.953s. The three selected new tests ran, exit1, three errors/zero failures/zero skipped, JUnit3.074s and Maven4.698s. Cancellation and identity-precedence tests threw InvalidCommand from premature node validation; UTF-8 replay threw IllegalStateException from the ASCII reader. There was no compile/fixture/environment failure. Python16 RED was exit1:15 rejection cases passed, only the new41-case positive errored against the old frozen38 manifest (0.006s). Those results matched Root's conditional authorization to repair without another approval round.

Original `compile.log`, `red.log`, `gate-red.log`, exit files, three-case Surefire XML/text under `red/`, pre-fix production files under `red-source/` and `red-source.sha256` preserve this checkpoint. Earlier evidence remains untouched.

## Repair

After bounded syntax/hash validation and command primary-key arbitration, exact duplicates still replay first. A newly claimed internal cancellation now records its unchanged cancelled receipt immediately, without querying instances/deployments or validating a live graph. For normal actions, instance scope/terminal/sequence/fence/version checks run before bound-graph validation. Start checks instance_exists, deployment_missing and deployment_mismatch before validating the matching deployed graph. Complete exact graph-node checks remain mandatory for matching executable contexts.

The result reader uses a UTF-8 decoder with REPORT for malformed/unmappable input and rejects ISO control code points. UUID grammar and finite state/reason sets remain unchanged; opaque IDs remain at most200 UTF-8 bytes. Encoding also uses REPORT, preventing silent replacement of malformed Java text. Before persistence, encoded results are checked against the same bounded decoder so an overlength/control/invalid result cannot be committed and become unreplayable. Valid canonical result bytes remain exactly Root's independently encoded bytes. The gate only updates its fixed expected identities to Root36+5; all strict detection remains unchanged.

`production-fix.patch` and `green-source/` contain the precise production change and final source snapshots.

## Final GREEN and regressions

| Check | Actual outcome |
| --- | --- |
| Real final execution36 + compatibility5 | exit0, 41/41, zero failure/error/skip; JUnit19.492s and2.879s, Maven25.598s |
| New independent synthetic gate16 | exit0, 16/16, 0.009s |
| Exact final real XML gate | exit0, 41 exact identities |
| Legacy deployment18/envelope8/RPC9 | exit0, 35/35, zero failure/error/skip; JUnit9.022s/0.027s/3.858s, Maven15.049s |
| Genuine Go→Java→Flowable→PostgreSQL interop | exit0, test0.370s/package0.382s |
| Original gates and gate tests | real18/8/9+interop1 accepted, synthetic12/12/20 passed |
| Governance/foundation240 | exit0, 240/240, zero skip,3.880s |

Final selected command:

```sh
bash services/workflow-engine/run-tests.sh -Dtest=RootExecutionRegistryTest,RootReturnCompatibilityTest -Dworkflow.reports=/proof/.work/execution-review-green-reports test
python3 services/workflow-engine/execution_ci_gate.py services/workflow-engine/.work/execution-review-green-reports services/workflow-engine/expected-execution-tests.json
bash services/workflow-engine/run-tests.sh -Dtest=RootDeploymentRegistryTest,RootCommandEnvelopeTest,RootDeploymentGrpcIT -Dworkflow.reports=/proof/.work/execution-review-legacy-reports verify
GOMODCACHE=/workspace/.weaveos-tools/go-mod GOCACHE=/workspace/.weaveos-tools/go-cache WEAVEOS_RPC_GO=/workspace/.weaveos-tools/go/bin/go WEAVEOS_RPC_JSON_REPORT=.work/execution-review-fix/interop.jsonl bash services/workflow-engine/run-rpc-interop.sh
node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs
```

`final-green.log` is the final implementation run; `green.log` preserves the first successful41 run before strict encoder hardening. Final original XML/text is under `green/` and `legacy/`; the real legacy-gate directories retain original XML copies. All original output, exits and genuine interop JSON events are retained, not synthesized. Source comparisons and hashes bind Root tests and final implementation. Locked offline caches/images were reused; no repeated dependency bootstrap or large cache copying. Fixture networks were internal with no published ports, all command traps/Root cleanup completed and residual fixture listings are empty.

These are fixture/build durations, not production throughput/latency/memory benchmarks. Input≤256KiB, result≤64KiB and opaque-ID≤200bytes are protocol bounds. Old proof24 remains unchanged with its earlier A evidence; no claim of a fresh proof24 run here.

## Remaining review boundary

Root source review and exact-head complete remote CI remain pending; no PR40, main, deployment, credentials, production network, frontend, POM, dependencies, Go BFF behavior or service bootstrap was changed. SQL remains the isolated contractual fixture. This repair does not provide public HTTP execution, BFF authorization, business projection integration or full backend delivery. The A worktree530 and preflight c226 remain clean.
