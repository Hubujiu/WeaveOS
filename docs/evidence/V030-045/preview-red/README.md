# Original five lifecycle-preview declaration RED

Exact frozen sourcec1f10c6cec18e0cca9579ceaf590b2e04a9a6701; independent V030-045 worktree. `go test -race -p 1 -count=1 -json -run '^TestRootLifecyclePreview' ./internal/apprecordservice`: exit1,5FAIL/0PASS/0SKIP,5 actual declaration behavior RED, no fixture/compiler/environment failure. Both real PG/session fixture and bounded single-connection pool initialized successfully; stderr empty, no race diagnostic.

| Exact case suffix | Actual assertion failure |
| --- | --- |
| CurrentRecordAndTargets | line19 first valid preview returns record service unavailable |
| RevokedRecordAccessDenied | line35 first valid preview returns record service unavailable |
| RejectsMismatchedRecord | line51 crossrecord expects applications.ErrMissing, receives unavailable with empty result |
| TerminalCannotReturnOrWithdraw | line57 first valid preview returns record service unavailable |
| SingleConnectionDoesNotNestPool | line81 preview returns unavailable, not deadline; original3second bound unmodified |

Later target/basis/session/revocation/terminal assertions remain unreached at the declaration stage. No claim of complete withdrawal/return eligibility or public HTTP/Flowable completion. Root owns independent assertion review and implementation release.

Original full JSONL/run.json/stderr/exit/test and no-behavior stub snapshots are byte-preserved. Exact SDK argv/times/environment/hashes/case messages in run.json; before/after source/test/dependency hashes match. Existing authorized private hot/archivePG18.6/Redis8.2.10 reused sequentially only after V044 complete tests; no fixture overlap/reset. No executor debug/source/test change, PR/merge/deployment. TDD:N/A for evidence-only preservation; commit parent mustc1f10c6 with exact remote lease.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| run.json | 4674 | 99867866a968f272081a5c13466e356602fcff00aaa7e5f8b36eabf3d6e537d5 |
| test.exit | 2 | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 |
| test.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| test.stdout.jsonl | 7506 | 5db8c7716da2f796c0128561564ee8d0d52f0c399d1df4cb4e59ba608b9f16cf |
| workflow_lifecycle_preview.go | 1355 | 46769aa3be5c0c7971c9a7cbbd0ee4d098bd9d3f03d607947fd1678c9fea9469 |
| root_workflow_lifecycle_preview_test.go | 3279 | b0e5e93b5c96feeafd7b537ae8efe73f39f5bbc71f65c89cdff4422381490641 |
