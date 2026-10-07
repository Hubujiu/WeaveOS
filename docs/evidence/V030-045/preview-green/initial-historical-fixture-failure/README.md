# Original candidate failure: historical evidence fixture

Exact sourceedd030e0363237268cec8ade442029abdfe9feb5. Frozen targeted command exit1: preview6PASS/1FAIL, pure qualification6PASS,0SKIP; full service was stopped/not run. Sole failure TestRootLifecyclePreviewHistoricalApproverOnlyOwnActualNode, line105: record service unavailable: invalid workflow evidence. Original JSONL/run.json/stderr/exit and pre-fix test/production snapshots preserved unchanged. Empty stderr, no race report.

Root independently diagnosed old projection fixture incomplete Default/Config metadata; production evidence builder correctly refused it. Root subsequently fixed only the fixture to use complete text/member definitions in014e3529; historical target assertion and production unchanged. This original failure remains a fixture failure, not proof of a production defect or valid pre-implementation RED. Two additional confirmed qualification regressions were added with the candidate and are explicitly not claimed to have earlier RED. Original five declaration RED remains preview-red. TDD:N/A for evidence-only preservation.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| run.json | 6314 | a23be48a8d0363e80dd5eb5956e6ff98fa707f7bfafad0a05e4ac36c299e5cbb |
| preview-and-policy.exit | 2 | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 |
| preview-and-policy.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| preview-and-policy.stdout.jsonl | 14279 | f404ed8d252f5c220f2aebf5cd55a627976d9ec4e6dfb87ef16fd1a7aab9deb2 |
| workflow_lifecycle_policy.go | 1420 | dcf001030a2abae5f3a229f147556e101c9ae55d311c07e63371bdc8315f05c4 |
| workflow_lifecycle_preview.go | 12144 | 948a164015b19a0807b933de77aa4c411bf72ce3d03ed1bb7f91e27a3effa390 |
| root_workflow_lifecycle_policy_test.go | 4291 | 971d9acdfc0febbd654b1e8cc2a5a0c9228f018db1c58cb1af8bc7394176653b |
| root_workflow_lifecycle_preview_test.go | 4469 | fcc88738848373eb7d8c98ddc7a7ba6a0014eb9036a4c5c6fdd048fcbd0c1041 |
