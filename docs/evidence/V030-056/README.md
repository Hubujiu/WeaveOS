# V030-056 · Exact fixed BPMN graph mapping

## Independent oracle and limits

V019 ADR defines graph edge identities, source/target mapping, approval gateway redirection, internal u_/r_ edges, defaults and fixed expressions. The existing independent fixture is start1 to approval2 to condition3, with true to end4 and false to end5. Root wrote the complete six-edge and two-default expected maps directly from that contract, not by traversing input edges or calling the renderer.

Only the existing TestRootBPMNMapsGraphAndClosedEdges is strengthened. Original node types, XML ID uniqueness, six-edge count and endpoint existence remain. The other eight BPMN tests and all helpers are byte-identical. No compiler, fixture, scoped entry, dependency or engine test change. This is the edge/default slice of ORACLE-041, not a complete XML namespace/attribute/parent-child allowlist or exhaustive graph proof.

## Actual sequence

- Fixed develop baseline a6f048407b99d15de5db60b49465b5ed81f2fa55; Go1.27.1, CGO1, GCC14.2.0, task-owned module/build caches, readonly module mode.
- Original BPMN9 and package race27 pass. In separate module copies, e_3 target is changed from end5 to existing end4, or e_0 source from start1 to existing end5. An independent probe first confirms the output really changed. Each original BPMN suite still passes9. Restored9/27/vet pass; no skip or build failure.
- Root adds only the complete edge/default comparison. Fixed gofmt aligns four map entries; pre-format source and exact diff are preserved. Final compiler_test.go is9236 bytes, SHA25640532681f760a181cbd8fb99d235fb4bee4bf648ab5cba107d0c1e70ac57ea91.
- New baseline9/27 pass. The same two probes each pass, and each mutated BPMN run is8 PASS/1 FAIL at the target mapping assertion, with the exact wrong endpoint visible in got/want. No dependency/syntax/compilation ERROR. Restored9/27/vet pass. No real Flowable run is claimed for these isolated unit mutants.
- Frozen pnpm installation, Node governance/foundation/contracts390, typecheck, actual Redocly lint, build, check-tasks, verify-repo and diff-check pass. Twelve existing OpenAPI warnings and the build chunk warning remain.

## Recoverable original records

execution-records.json contains the124 listed original UTF-8 files plus the frozen inventory, Root archive script and final candidate test. manifest.json records each path, byte count and SHA256, plus the bundle digest. Reconstruct each file from its text encoded as UTF-8 and verify its digest before replay. Replay must not be described as original execution.

Variant module copies are not archived wholesale: only each changed compiler, exact test and independent probe are included. Other original module files come from the immutable baseline commit; no module cache, binary or private data is included. Original commands, JSON test events, failure output, before/after hashes, formatting and scope proof remain intact.

The bundle is frozen before archive-checks and exact-HEAD scanning; those later checkpoints are recorded separately. Final exact-candidate CI and develop integration are archived in the PR and Notion without rewriting this snapshot. Real engine/product verification, release signoff and vulnerability acceptance are distinct. Main and deployment are not authorized by these unit results.
