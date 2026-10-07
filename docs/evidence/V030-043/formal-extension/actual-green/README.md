# Actual frozen default-package and formal runtime GREEN

Exact source0987d3779da2e94cf6c4a8585492196878bbb066. Root owns the scale measurement change, its oracle self-tests and the new formal Save chain; executor changed none of source/tests/gates/runners/dependency versions/security configuration. All recorded before/after hashes match. Original old scale failure and full EXPLAIN remain in regression/replay-fix; original test source remains in original-test-seam. This run does not rewrite that historical failure.

| Default package | Exit | Top-level PASS | Subcase PASS |
| --- | --- | --- | --- |
| apprecordservice | 0 | 187 | 95 |
| apprecordhttp | 0 | 0 | 0 (no test files; compiles) |
| appstructure | 0 | 118 | 26 |
| applications | 0 | 29 | 12 |
| cmd/bff | 0 | 66 | 42 |

Totals400top-levelPASS/175subcasePASS,0FAIL/0SKIP; sequential `go test -race -p 1 -count=1 -json -failfast <package>`, full default selection. `go vet` on the same five packages and `go build ./cmd/bff` each exit0. Original stderr empty, no race report. Private build binary retained; SHA256 submitted.

Existing `node services/workflow-engine/run-formal-runtime.mjs` exit0 and `python3 services/workflow-engine/formal_runtime_gate.py services/workflow-engine/formal-runtime-reports/tests.jsonl` exit0. Exactly6identities PASS, no failures/skips:

- TestRootFormalRuntimeNodeSaveThenApproveLatest
- TestRootFormalRuntimePublishAndApprove
- TestRootFormalRuntimePublishAndApprove/agree
- TestRootFormalRuntimePublishAndApprove/reject
- TestRootFormalRuntimeLostReplyBffRestart
- TestRootFormalRuntimeEngineRestart

Real BFF/WorkflowEngineMain OS processes, authenticated loopback gRPC and real HTTPS ingress, local Save -> stale approval conflict -> engine outage preserves Save -> restarted engine approval binds latest v2 -> immutable history/execution evidence -> closed-task Save replay. Original scenarios retained. Native32-table engine schema explicitly migrated by existing script, offline Java compilation succeeded. Isolation report verifies internal network, no published ports or Docker socket. Runner created/cleaned only its own resources. Earlier private task PG/Redis retained unchanged.

Fixed Maven/JDK/PostgreSQL/Redis images pulled at script digest; existing prepare-build.sh exit0, both go-offline phases BUILD SUCCESS using Maven Central official registry and unchanged POM. Maven3.9.11/JDK17.0.17, static Goosev3.28.0 prepared with CI's CGO_ENABLED=0, Go1.27.1/Node24.19.0. Tool prefetch logs/cache SHA256 inventory retained; no lastUpdated files; temporary proxy settings removed by original script. No version upgrade, real credential, insecure network/trust setting or old task container change. Go test/formal build uses verified cached modules with GOPROXY=off/GOSUMDB=sum.golang.org.

Raw JSONL/stdout/stderr/exits, full formal step/container logs, exact commands/times/source hashes/isolation/report identities and original/submitted artifact hashes in run.json. Private generated report directory archived outside Git after byte-identical copies; no binary/settings/session secret committed. Scope does not include all repository packages, frontend/browser/product/final CI, public record-create workflow trigger, PR, merge or deployment. Root source/evidence review and acceptance remain pending.

Three raw stdout streams contain original whitespace/control framing and are stored as lossless gzip (.stdout.gz); decompressed SHA256 equals original bytes. Originals remain task-private. Other logs are unchanged raw files.

Container .log streams are also stored as lossless .log.gz to preserve original trailing blank lines and include them despite the repository global log ignore rule. Manifest coverage was checked against tracked files.
