# V030-022 P1 execution evidence

Root input commit: `737daa47e8baa0f81f723a72dad63e88927e6f24`. Stacked baseline: `d99b8ebc3b9d0167927c75a2d56827c4b7cc5400`. Draft PR32 targets `task/V030-020-workflow-http`.

## RED → GREEN

1. Root placeholder/test/SQL compiled successfully: `compile.txt`, exit 0, 2026-10-04 18:38:22 UTC. Build dependency preparation is `prepare.txt`; environment startup/downloads were not counted as RED.
2. `bash services/workflow-engine/run-tests.sh test` on original placeholder: `red.txt`, `red.exit`, `red-surefire.xml`, 18:38:48 UTC. All 15 executed, 5 failures + 10 errors, no skipped tests. Setup succeeds, and each failure is the deliberate `IllegalStateException: V030-022 P1 not implemented` or expected exception mismatch caused by that placeholder. RED snapshot commit `db49a78` preceded implementation.
3. Same command after implementation: `green.txt`, `green.exit`, `green-surefire.xml`, 18:42:17 UTC; 15 passed, no failures/errors/skips. Coverage: exact identity/hash/engine definition, durable replay, payload/context/logical-version conflicts, two concurrent identical requests, three failure points, outer REQUIRED rollback, reopened engine, absent lookup, invalid identity/version/XML boundaries, separate app scope.
4. Resource sampling reran the identical suite: `green-resource-run.txt`, `resource-run.json`, `resource-samples.jsonl`, 18:44:05 UTC; 15 passed. Driver command invoked the runner from Python, sampled `docker ps -q --filter label=weaveos.package=V030-022` and `docker stats --no-stream --format '{{json .}}' <ids>` while active, and recorded monotonic wall time.
5. Existing regression: copied the tracked `prototypes/flowable-local-tx/` into `/tmp/v022-regression`, symlinked its `.work/m2` to the same isolated dependency cache, then executed unchanged `bash /tmp/v022-regression/run-proof.sh test`. `regression.txt`, exit 0, 18:43:00 UTC. 15 LocalCommandExecutorTest + 9 GeneratedGraphTest = 24 pass, no skipped. Both original Surefire XML reports preserved.

## Test/source equivalence

`root-test.java.txt`, `root-fixture.sql.txt`, `root-placeholder.java.txt` are byte-for-byte Root snapshots before implementation. `root-source-sha256.txt` binds original test and fixture. `source-equivalence.txt` checks all current test/fixture and old prototype tracked files against `git show 737daa4:<path>`; each regression-copy file also matches baseline bytes. No assertion, test, fixture, old proof source/build file, Go compiler or Go/frontend code was edited; no new tests were authored.

Public Request/Receipt/Stage/exception/constructor/deploy/lookup signatures remain the Root contract. The only production additions are DeploymentRegistry implementation and ControlledBpmn internal helper. New runners copy the old proof's image locks and dependency preparation pattern. The new runner uses task-specific labels, names/cleanup and prints isolation facts. Production code contains no DDL or schema creation. SQL remains solely a random-schema test fixture.

## Environment and measured costs

- Maven image `mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b` (local displayed size 503 MB); Temurin Java 17.0.17+10, Maven 3.9.11, release 17 compilation.
- PG image `mirror.gcr.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722` (local displayed size 457 MB); PostgreSQL 18.6. Boot 4.0.2, Flowable 8.0.0, Surefire 3.5.4. Same POM versions as original proof.
- Dedicated synthetic URL asserted by Root tests: `jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture`. Random schemas per test, fixture-only identity, internal Docker network, no published port/socket mount, PG data in disposable tmpfs. Runner log confirms `published ports={}` and `internal=true`.
- First GREEN: suite 7.334 s, Maven 9.780 s. Resource run: suite 7.364 s, Maven 8.717 s; monotonic driver wall 17.284 s includes container provisioning, readiness, sampling and cleanup. Original 24-test proof: suite components 8.726 s + 4.068 s, Maven 15.043 s.
- Docker sampled maxima, **not guaranteed peaks**: Maven/test container 1.013 GiB memory, 322.43% CPU; PG 133 MiB memory, 43.73% CPU. Sampling was roughly every 2 s due to `docker stats --no-stream`; individual test JVM/engine memory is not separated from the container total. No resource limit or production sizing inferred.
- Measured local dependency cache 101 MiB, extracted toolchain 278 MiB, module build outputs 232 KiB. Docker images may share layers; these figures must not be summed as exact host disk cost.

Validation/hash/XML walk uses O(B) time and memory for UTF-8 byte count B, bounded at 1 MiB with parser depth <=8. Unique-index registration is approximately O(log D) for D deployments; real engine work scales with graph nodes/edges and persistence. Same-key competitors wait on database unique constraints; there is no global application mutex. Receipt rows use O(D) metadata and Flowable retains the XML/definitions. These are theoretical bounds, not a deployment p95, load test or production cost budget.

## Limitations and handoff

Root's 15-test suite directly exercises the minimal generated process and eight unsafe/boundary XML cases; the old nine graph cases verify Flowable execution of compiler-generated approval/routing XML through the unchanged prototype. They do not independently exercise every branch of the new allowlist. No additional tests were written by the implementation executor; Root retains test design/acceptance ownership.

Returned receipts inside a surrounding REQUIRED transaction are provisional until that outer transaction commits. Lookup absence never proves failure. Database serialization/deadlock/connection errors propagate and roll back rather than inventing success; callers must preserve the original identity for retry. P2 durable cross-service intent, gRPC/BFF publishing, authentication/authorization, production migrations/roles and production runtime budgets remain out of scope. Draft saves do not publish.

The new Java module has **no CI gate**. Existing CI status cannot prove this suite executed. Root review is pending; no main merge or deployment. All synthetic containers/networks from these runs were checked absent after cleanup. Worktree, caches and evidence retained for review/reproduction.
