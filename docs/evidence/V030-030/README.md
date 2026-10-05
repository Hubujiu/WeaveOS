# V030-030 Stage A cloud evidence

Original execution on 2026-10-05 UTC in /workspace/WeaveOS-worktrees/V030-030. Base80e4151; Root Java tests/stub f869911 -> 3cd02c3. Root gate tests/stub/manifest/CI f6252e3 -> d3fc4f1. Source patches are preserved for recovery after squash; snapshots are not replay claims. No test/fixture/expected-case changes were authored by the worker.

| Phase | Command from worktree root | Exit / actual result |
| --- | --- | --- |
| Prepare | `bash services/workflow-engine/prepare-build.sh` | 0; locked toolchain/dependencies |
| Compile | `bash services/workflow-engine/run-tests.sh test-compile` | 0; Maven2.225s |
| Original Java RED | `bash services/workflow-engine/run-tests.sh -Dtest=RootCommandEnvelopeTest -Dworkflow.reports=/proof/.work/v030-030-red test` | 1; 8 run, 3 stub behavior errors, 5 rejection cases pass |
| Java GREEN | `bash services/workflow-engine/run-tests.sh -Dtest=RootCommandEnvelopeTest -Dworkflow.reports=/proof/.work/v030-030-green test` | 0; 8 pass, JUnit0.102s, Maven3.002s |
| Original gate RED / GREEN | `python3 services/workflow-engine/envelope_ci_gate_tests.py` | RED1, 12 run/1 stub error; GREEN0, 12 pass/0.004s |
| Real legacy regressions | `bash services/workflow-engine/run-tests.sh -Dtest=RootDeploymentRegistryTest,RootDeploymentGrpcIT -Dworkflow.reports=/proof/.work/v030-030-regression clean verify` | 0; deployment18 and RPC9 pass, Maven15.519s |
| Legacy proof | `bash prototypes/flowable-local-tx/prepare-build.sh` then `bash prototypes/flowable-local-tx/run-proof.sh clean verify` | 0; local transaction15 plus generated graph9 pass, Maven15.596s |
| Interop | `WEAVEOS_RPC_GO=/workspace/.weaveos-tools/go/bin/go GOMODCACHE=/workspace/.weaveos-tools/go-mod GOCACHE=/workspace/.weaveos-tools/go-cache WEAVEOS_RPC_JSON_REPORT=docs/evidence/V030-030/interop.jsonl bash services/workflow-engine/run-rpc-interop.sh` | 0; genuine RootGoJavaPostgresDeploymentInterop pass |
| Envelope XML | `python3 services/workflow-engine/envelope_ci_gate.py docs/evidence/V030-030/green services/workflow-engine/expected-envelope-tests.json` | 0; exact8 |
| Legacy deployment XML | `python3 services/workflow-engine/ci_gate.py docs/evidence/V030-030/deployment services/workflow-engine/expected-tests.json` | 0; exact18 |
| RPC XML + interop JSON | `python3 services/workflow-engine/rpc_ci_gate.py docs/evidence/V030-030/rpc services/workflow-engine/expected-rpc-tests.json docs/evidence/V030-030/interop.jsonl` | 0; exact9 Java +1 Go |
| Legacy gate tests | `python3 services/workflow-engine/ci_gate_tests.py` / `python3 services/workflow-engine/rpc_ci_gate_tests.py` | 0;12 /20 pass |
| Shared classification | `node --test infra/runtime/advisory-review.test.mjs` | 0;5 pass |
| Real transport | `GOMODCACHE=/workspace/.weaveos-tools/go-mod GOCACHE=/workspace/.weaveos-tools/go-cache /workspace/.weaveos-tools/go/bin/go -C services/bff test -count=1 -json ./internal/securityreview` | 0;1 pass,0.018s package |
| Governance | `node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs` | 0;240 pass |

Raw logs, exit files, XML and genuine JSON event streams are retained beside this README. Initial RED XML is at the evidence root; GREEN, deployment, RPC and proof reports have separate directories. Regression reports were copied into separate deployment/RPC directories to run each unchanged gate. They are the original XML, not synthesized reports. Gate acceptance tests themselves use Root synthetic XML; they do not represent real Java/PG runs. SHA256 manifests bind source and output. Repo ignores *.log, so raw logs are explicitly staged.

Security commits61494fbc and bf8c5aa1 were reused exactly as authorized, with no dependency update. Their original RED and source review belong to Root/shared task; this task records only real local classification/transport regression. Those tests alone do not establish complete vulnerability-scan success. Base remote CI/governance succeeded but product run37264265995 failed on Firefox refresh; final-head status is recorded in task/PR when observed.

Parser accepts at most538bytes and uses O(n) parsing/hashing/copy space with fixed12 strings/6 integers. This is a theoretical input bound, not heap measurement or production throughput. Only Stage A identity parsing and report validation are delivered. No engine execution, runtime permission check, payload/receipt/DDL, application projection or cross-service atomicity claim.
