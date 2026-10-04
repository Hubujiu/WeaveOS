# V030-019 real Flowable compatibility run

## Source and fixed toolchain

- Fetched the third-package contract from the Notion ADR page `V030-019 ADR｜流程图结构与统一条件校验` at `2026-10-04T04:22:13.738Z`; page path: project → Architecture & ADR → ADR database → ADR-009 proposal. Verification state is `unverified`; response had no truncation indicator. The third package freezes a real Flowable compatibility check and does not claim product service integration.
- Plan: `docs/evidence/V030-019/engine/implementation-plan.md`.
- Docker engine: `28.4.0 linux/amd64`.
- Maven/Java: Maven `3.9.11`; Temurin `17.0.17+10`, release 17.
- Flowable dependency: `8.0.0` from the fixed test module POM.
- PostgreSQL image: `mirror.gcr.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722` (fixed PostgreSQL 18.6 image).
- Maven toolchain image: `mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b`.
- The 15 imported B3 files each matched the blob SHA in `import-manifest.json` (`0` mismatches). `prepare-build.sh` exited `0`; it used the module-local Maven Central mirror and task-local `.work` caches. Its private generated settings file was removed on exit.

## Compiler-exported inputs

Fixtures were written directly by `TestRootExportEngineCompatibilityFixtures` using the actual Go compiler. The test used all/any with two assignees and did not edit or post-process XML.

| Fixture | SHA-256 before engine run | SHA-256 after engine run |
| --- | --- | --- |
| `generated-all.bpmn20.xml` | `080fb548c65f73471ec8096d2f55fcb39aeeea6447593339465f322ee531134d` | `080fb548c65f73471ec8096d2f55fcb39aeeea6447593339465f322ee531134d` |
| `generated-any.bpmn20.xml` | `9ac8eb04a2d29de94affe7906baecfb26183b42f7c06999196dc7db650f4693d` | `9ac8eb04a2d29de94affe7906baecfb26183b42f7c06999196dc7db650f4693d` |

Export test command from `services/bff`:

```sh
WEAVEOS_FLOW_BPMN_OUTPUT=/workspace/WeaveOS-worktrees/V030-019/prototypes/flowable-local-tx/src/test/resources/v019 GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test -run '^TestRootExportEngineCompatibilityFixtures$' ./internal/flowgraph
```

Exit code: `0`. The test source SHA-256 is `fcaa0f596590494dc6f37e48d9362585909ed525ebbd1d52391a7cedfb267e1d`.

## Real-engine execution

- Command: `bash prototypes/flowable-local-tx/prepare-build.sh` — exit `0`.
- Command: `bash prototypes/flowable-local-tx/run-proof.sh verify` — exit `0`.
- The proof script used its unique internal Docker network, a temporary PostgreSQL data directory, no published ports, and the pinned Maven image for an offline Maven verify. Its EXIT trap removed its generated PostgreSQL container and network. After completion, no active or stopped `weaveos.package=B3` container and no network with that label remained.
- Result: original `LocalCommandExecutorTest` 15/15 passed; Root-authored `GeneratedGraphTest` 9/9 passed; total 24 tests, `0 failures`, `0 errors`, `0 skipped`. Surefire reported 27.066 seconds for the original suite and 8.695 seconds for the generated-graph suite.
- The actual Surefire XML reports are retained at `docs/evidence/V030-019/engine/reports/`:
  - `TEST-org.weaveos.proof.LocalCommandExecutorTest.xml` SHA-256 `0ba67c109b85904876ac0dbf2eb953ef9ac60aa11c1aee662941df652259f568`.
  - `TEST-org.weaveos.proof.GeneratedGraphTest.xml` SHA-256 `a860c3d331168e83d6c5318b6808f48d58a5780f00d46192b37a6c5cc84f8808`.
- Root Java test SHA-256: `f48b5ba696123a415918f8deac26ee74a710daf0811a8b70c1202c6c5d30f370`.
- No compiler defect or fixture error was found. The actual BPMN files were byte-identical before and after execution.

## Go and repository checks

- `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test -race ./internal/flowgraph` — exit `0`, all 22 Go test functions included.
- `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go vet ./internal/flowgraph` — exit `0`.
- `git diff --check` — exit `0`.
- This is real Flowable 8.0.0/PostgreSQL 18.6 test-module compatibility evidence for the generated XML and imported B3 proof. It does not validate product HTTP/gRPC wiring, distributed atomicity, production authorization, process deployment, or deployment configuration.
